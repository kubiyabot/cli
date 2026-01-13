"""
Agent Runtime Server Manager

Manages the agent-runtime server process lifecycle.
"""

import asyncio
import os
import signal
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Optional
import aiohttp
import structlog
import yaml

logger = structlog.get_logger(__name__)


@dataclass
class ServerConfig:
    """Configuration for agent-runtime server."""
    grpc_port: int = 50052
    http_port: int = 8082
    health_port: int = 8083
    config_dir: Path = Path.home() / ".kubiya"
    log_level: str = "info"
    database_url: Optional[str] = None


class AgentRuntimeServer:
    """Manages agent-runtime server process."""

    def __init__(self, binary_path: Path, config: ServerConfig):
        """
        Initialize server manager.

        Args:
            binary_path: Path to agent-runtime binary
            config: Server configuration
        """
        self.binary_path = binary_path
        self.config = config
        self.process: Optional[subprocess.Popen] = None
        self.pid_file = config.config_dir / "agent-runtime.pid"
        self.log_file = config.config_dir / "logs" / "agent-runtime.log"
        self.config_file = config.config_dir / "agent-runtime.yaml"

        # Create directories
        self.log_file.parent.mkdir(parents=True, exist_ok=True)

        # GRPC address for clients
        self.grpc_address = f"localhost:{config.grpc_port}"

    async def start(self, wait_for_health: bool = True, timeout: int = 30) -> bool:
        """
        Start agent-runtime server.

        Args:
            wait_for_health: Whether to wait for health check
            timeout: Health check timeout in seconds

        Returns:
            True if started successfully

        Raises:
            RuntimeError: If server fails to start
        """
        logger.info("starting_agent_runtime_server",
                   grpc_port=self.config.grpc_port,
                   http_port=self.config.http_port)

        # Check if already running
        if self._is_running():
            logger.warning("server_already_running", pid=self._get_pid())
            return True

        # Generate configuration file
        self._generate_config()

        # Build command
        cmd = [
            str(self.binary_path),
            "--grpc-port", str(self.config.grpc_port),
            "--http-port", str(self.config.http_port),
            "--health-port", str(self.config.health_port),
            "--config", str(self.config_file),
            "--log-level", self.config.log_level,
        ]

        # Build environment
        env = os.environ.copy()
        env["AGENT_RUNTIME_BASE_DIR"] = str(self.config.config_dir)

        if self.config.database_url:
            env["DATABASE_URL"] = self.config.database_url
        else:
            # Default SQLite database
            db_path = self.config.config_dir / "agent-runtime.db"
            env["DATABASE_URL"] = f"sqlite:{db_path}"

        try:
            # Start process
            with open(self.log_file, 'a') as log_f:
                self.process = subprocess.Popen(
                    cmd,
                    env=env,
                    stdout=log_f,
                    stderr=subprocess.STDOUT,
                    start_new_session=True,  # Detach from parent
                )

            # Save PID
            self._save_pid(self.process.pid)

            logger.info("server_started", pid=self.process.pid, log_file=str(self.log_file))

            # Wait for health check
            if wait_for_health:
                if not await self.wait_for_health(timeout):
                    self.stop()
                    raise RuntimeError("Server failed health check")

            return True

        except Exception as e:
            logger.error("failed_to_start_server", error=str(e))
            raise RuntimeError(f"Failed to start server: {e}")

    def stop(self, timeout: int = 10) -> bool:
        """
        Stop agent-runtime server gracefully.

        Args:
            timeout: Graceful shutdown timeout in seconds

        Returns:
            True if stopped successfully
        """
        logger.info("stopping_agent_runtime_server")

        pid = self._get_pid()
        if not pid:
            logger.warning("no_pid_found")
            return True

        try:
            # Check if process exists
            try:
                os.kill(pid, 0)
            except OSError:
                logger.info("process_not_running", pid=pid)
                self._cleanup_pid_file()
                return True

            # Send SIGTERM for graceful shutdown
            logger.info("sending_sigterm", pid=pid)
            os.kill(pid, signal.SIGTERM)

            # Wait for process to exit
            start_time = time.time()
            while time.time() - start_time < timeout:
                try:
                    os.kill(pid, 0)
                    time.sleep(0.1)
                except OSError:
                    logger.info("server_stopped_gracefully", pid=pid)
                    self._cleanup_pid_file()
                    return True

            # Force kill if still running
            logger.warning("forcing_kill", pid=pid)
            os.kill(pid, signal.SIGKILL)
            time.sleep(0.5)

            self._cleanup_pid_file()
            logger.info("server_stopped_forcefully", pid=pid)
            return True

        except Exception as e:
            logger.error("error_stopping_server", error=str(e), pid=pid)
            return False

    async def wait_for_health(self, timeout: int = 30) -> bool:
        """
        Wait for server to become healthy.

        Args:
            timeout: Maximum time to wait in seconds

        Returns:
            True if healthy, False if timeout
        """
        logger.info("waiting_for_health_check", timeout=timeout)

        url = f"http://localhost:{self.config.http_port}/health"
        start_time = time.time()
        attempt = 0

        while time.time() - start_time < timeout:
            attempt += 1
            try:
                async with aiohttp.ClientSession() as session:
                    async with session.get(url, timeout=aiohttp.ClientTimeout(total=2)) as resp:
                        if resp.status == 200:
                            data = await resp.json()
                            if data.get("status") == "healthy":
                                logger.info("server_healthy", attempts=attempt)
                                return True

            except Exception as e:
                logger.debug("health_check_failed", attempt=attempt, error=str(e))

            await asyncio.sleep(0.5)

        logger.error("health_check_timeout", timeout=timeout, attempts=attempt)
        return False

    async def health_check(self) -> bool:
        """
        Check if server is currently healthy.

        Returns:
            True if healthy, False otherwise
        """
        try:
            url = f"http://localhost:{self.config.http_port}/health"
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=aiohttp.ClientTimeout(total=2)) as resp:
                    if resp.status == 200:
                        data = await resp.json()
                        return data.get("status") == "healthy"
            return False
        except Exception:
            return False

    def _is_running(self) -> bool:
        """Check if server is running."""
        pid = self._get_pid()
        if not pid:
            return False

        try:
            os.kill(pid, 0)
            return True
        except OSError:
            return False

    def _get_pid(self) -> Optional[int]:
        """Get PID from file."""
        if not self.pid_file.exists():
            return None

        try:
            return int(self.pid_file.read_text().strip())
        except Exception as e:
            logger.error("failed_to_read_pid", error=str(e))
            return None

    def _save_pid(self, pid: int):
        """Save PID to file."""
        try:
            self.pid_file.write_text(str(pid))
        except Exception as e:
            logger.error("failed_to_save_pid", error=str(e))

    def _cleanup_pid_file(self):
        """Remove PID file."""
        try:
            if self.pid_file.exists():
                self.pid_file.unlink()
        except Exception as e:
            logger.error("failed_to_cleanup_pid_file", error=str(e))

    def _generate_config(self):
        """Generate agent-runtime configuration file."""
        # Look for Claude Code binary (future: download/manage it too)
        claude_code_path = self._find_claude_code_binary()

        config = {
            "server": {
                "grpc_port": self.config.grpc_port,
                "http_port": self.config.http_port,
                "health_port": self.config.health_port,
            },
            "runtimes": [],
            "logging": {
                "level": self.config.log_level,
                "format": "json",
            }
        }

        # Add Claude Code runtime if found
        if claude_code_path:
            config["runtimes"].append({
                "name": "claude-code",
                "runtime_type": "binary",
                "executable": str(claude_code_path),
                "enabled": True,
            })
        else:
            # Add Python example runtime as fallback
            python_runtime_path = self.config.config_dir / "runtimes" / "python-example" / "main.py"
            if python_runtime_path.exists():
                config["runtimes"].append({
                    "name": "claude-code",
                    "runtime_type": "python",
                    "executable": str(python_runtime_path),
                    "enabled": True,
                })

        # Write config
        with open(self.config_file, 'w') as f:
            yaml.dump(config, f, default_flow_style=False)

        logger.info("config_generated", path=str(self.config_file), runtimes=len(config["runtimes"]))

    def _find_claude_code_binary(self) -> Optional[Path]:
        """Try to find Claude Code binary."""
        # Check common locations
        possible_paths = [
            Path.home() / ".kubiya" / "bin" / "claude-code",
            Path("/usr/local/bin/claude-code"),
            Path("/opt/claude-code/claude-code"),
        ]

        for path in possible_paths:
            if path.exists() and os.access(path, os.X_OK):
                logger.info("found_claude_code_binary", path=str(path))
                return path

        logger.warning("claude_code_binary_not_found")
        return None

    def get_status(self) -> dict:
        """Get server status information."""
        pid = self._get_pid()
        is_running = self._is_running()

        return {
            "running": is_running,
            "pid": pid if is_running else None,
            "grpc_address": self.grpc_address,
            "http_port": self.config.http_port,
            "config_file": str(self.config_file),
            "log_file": str(self.log_file),
        }

    def get_logs(self, lines: int = 50) -> str:
        """
        Get recent log lines.

        Args:
            lines: Number of lines to return

        Returns:
            Recent log content
        """
        if not self.log_file.exists():
            return ""

        try:
            with open(self.log_file, 'r') as f:
                all_lines = f.readlines()
                return ''.join(all_lines[-lines:])
        except Exception as e:
            logger.error("failed_to_read_logs", error=str(e))
            return ""
