"""
Binary Manager for Agent Runtime

Handles downloading, verifying, and managing the agent-runtime binary.
"""

import asyncio
import hashlib
import os
import platform
import shutil
import tarfile
import tempfile
from pathlib import Path
from typing import Optional, Tuple
import aiohttp
import structlog
from packaging import version

logger = structlog.get_logger(__name__)

GITHUB_REPO = "kubiya-ai/agent-runtime"
GITHUB_API_BASE = f"https://api.github.com/repos/{GITHUB_REPO}"
GITHUB_RELEASES_BASE = f"https://github.com/{GITHUB_REPO}/releases/download"


class BinaryManager:
    """Manages agent-runtime binary lifecycle."""

    def __init__(self, config_dir: Path):
        """
        Initialize binary manager.

        Args:
            config_dir: Configuration directory (e.g., ~/.kubiya)
        """
        self.config_dir = Path(config_dir)
        self.binary_dir = self.config_dir / "bin"
        self.agent_runtime_path = self.binary_dir / "agent-runtime"
        self.version_file = self.binary_dir / ".agent-runtime-version"

        # Create directories
        self.binary_dir.mkdir(parents=True, exist_ok=True)

    async def ensure_binary(self, version: str = "latest") -> Path:
        """
        Ensure agent-runtime binary is available.

        Downloads binary if missing or outdated.

        Args:
            version: Version to ensure (e.g., "v0.1.0" or "latest")

        Returns:
            Path to agent-runtime binary

        Raises:
            RuntimeError: If binary cannot be obtained
        """
        logger.info("ensuring_agent_runtime_binary", version=version, path=str(self.agent_runtime_path))

        # Check if binary exists and is valid
        if self._is_binary_valid():
            current_version = self._get_current_version()
            logger.info("binary_found", version=current_version)

            # If requesting latest, check for updates
            if version == "latest":
                latest_version = await self._fetch_latest_version()
                if latest_version and self._should_update(current_version, latest_version):
                    logger.info("update_available", current=current_version, latest=latest_version)
                    await self._download_binary(latest_version)
                    return self.agent_runtime_path

            # If requesting specific version, check if we have it
            if version != "latest" and current_version != version:
                logger.info("different_version_requested", current=current_version, requested=version)
                await self._download_binary(version)

            return self.agent_runtime_path

        # Binary doesn't exist or is invalid, download it
        logger.info("binary_not_found", version=version)
        if version == "latest":
            version = await self._fetch_latest_version()

        await self._download_binary(version)
        return self.agent_runtime_path

    def _is_binary_valid(self) -> bool:
        """Check if binary exists and is executable."""
        if not self.agent_runtime_path.exists():
            return False

        if not os.access(self.agent_runtime_path, os.X_OK):
            logger.warning("binary_not_executable", path=str(self.agent_runtime_path))
            return False

        return True

    def _get_current_version(self) -> Optional[str]:
        """Get currently installed version."""
        if not self.version_file.exists():
            return None

        try:
            return self.version_file.read_text().strip()
        except Exception as e:
            logger.error("failed_to_read_version_file", error=str(e))
            return None

    def _save_version(self, ver: str):
        """Save installed version to file."""
        try:
            self.version_file.write_text(ver)
        except Exception as e:
            logger.error("failed_to_save_version", error=str(e))

    async def _fetch_latest_version(self) -> str:
        """Fetch latest version from GitHub."""
        try:
            async with aiohttp.ClientSession() as session:
                url = f"{GITHUB_API_BASE}/releases/latest"
                async with session.get(url) as resp:
                    if resp.status != 200:
                        logger.error("failed_to_fetch_latest_version", status=resp.status)
                        raise RuntimeError(f"Failed to fetch latest version: {resp.status}")

                    data = await resp.json()
                    latest = data["tag_name"]
                    logger.info("fetched_latest_version", version=latest)
                    return latest

        except Exception as e:
            logger.error("error_fetching_latest_version", error=str(e))
            raise RuntimeError(f"Failed to fetch latest version: {e}")

    def _should_update(self, current: Optional[str], latest: str) -> bool:
        """Check if should update to latest version."""
        if not current:
            return True

        try:
            # Remove 'v' prefix for comparison
            current_ver = version.parse(current.lstrip('v'))
            latest_ver = version.parse(latest.lstrip('v'))
            return latest_ver > current_ver
        except Exception as e:
            logger.error("version_comparison_failed", error=str(e))
            return False

    async def _download_binary(self, ver: str):
        """
        Download and install binary for specified version.

        Args:
            ver: Version to download (e.g., "v0.1.0")
        """
        logger.info("downloading_binary", version=ver)

        # Detect platform and architecture
        platform_name, arch = self._detect_platform()
        target = f"{arch}-{platform_name}"

        # Construct download URL
        filename = f"agent-runtime-{ver}-{target}.tar.gz"
        download_url = f"{GITHUB_RELEASES_BASE}/{ver}/{filename}"
        checksum_url = f"{download_url}.sha256"

        logger.info("download_url", url=download_url)

        # Download to temporary file
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            archive_path = temp_path / filename
            checksum_path = temp_path / f"{filename}.sha256"

            try:
                # Download archive
                await self._download_file(download_url, archive_path)
                logger.info("downloaded_archive", size=archive_path.stat().st_size)

                # Download checksum
                await self._download_file(checksum_url, checksum_path)

                # Verify checksum
                if not self._verify_checksum(archive_path, checksum_path):
                    raise RuntimeError("Checksum verification failed")

                # Extract binary
                self._extract_binary(archive_path)

                # Make executable
                os.chmod(self.agent_runtime_path, 0o755)

                # Save version
                self._save_version(ver)

                logger.info("binary_installed_successfully", version=ver, path=str(self.agent_runtime_path))

            except Exception as e:
                logger.error("binary_download_failed", error=str(e))
                raise RuntimeError(f"Failed to download binary: {e}")

    def _detect_platform(self) -> Tuple[str, str]:
        """
        Detect current platform and architecture.

        Returns:
            Tuple of (platform, architecture) for download URL
        """
        system = platform.system().lower()
        machine = platform.machine().lower()

        # Map platform
        if system == "darwin":
            platform_name = "apple-darwin"
        elif system == "linux":
            platform_name = "unknown-linux-gnu"
        else:
            raise RuntimeError(f"Unsupported platform: {system}")

        # Map architecture
        if machine in ("x86_64", "amd64"):
            arch = "x86_64"
        elif machine in ("arm64", "aarch64"):
            arch = "aarch64"
        else:
            raise RuntimeError(f"Unsupported architecture: {machine}")

        logger.info("detected_platform", platform=platform_name, arch=arch)
        return platform_name, arch

    async def _download_file(self, url: str, dest: Path, chunk_size: int = 8192):
        """Download file with progress."""
        async with aiohttp.ClientSession() as session:
            async with session.get(url) as resp:
                if resp.status != 200:
                    raise RuntimeError(f"Download failed: {resp.status}")

                total_size = int(resp.headers.get('content-length', 0))
                downloaded = 0

                with open(dest, 'wb') as f:
                    async for chunk in resp.content.iter_chunked(chunk_size):
                        f.write(chunk)
                        downloaded += len(chunk)

                        if total_size > 0:
                            progress = (downloaded / total_size) * 100
                            logger.debug("download_progress", progress=f"{progress:.1f}%")

    def _verify_checksum(self, archive_path: Path, checksum_path: Path) -> bool:
        """Verify archive checksum."""
        try:
            # Read expected checksum
            checksum_content = checksum_path.read_text().strip()
            expected_checksum = checksum_content.split()[0]

            # Calculate actual checksum
            sha256 = hashlib.sha256()
            with open(archive_path, 'rb') as f:
                for chunk in iter(lambda: f.read(8192), b''):
                    sha256.update(chunk)

            actual_checksum = sha256.hexdigest()

            if actual_checksum != expected_checksum:
                logger.error("checksum_mismatch", expected=expected_checksum, actual=actual_checksum)
                return False

            logger.info("checksum_verified")
            return True

        except Exception as e:
            logger.error("checksum_verification_failed", error=str(e))
            return False

    def _extract_binary(self, archive_path: Path):
        """Extract binary from tar.gz archive."""
        try:
            with tarfile.open(archive_path, 'r:gz') as tar:
                # Extract all files
                tar.extractall(path=self.binary_dir)

            logger.info("binary_extracted", path=str(self.binary_dir))

        except Exception as e:
            logger.error("extraction_failed", error=str(e))
            raise RuntimeError(f"Failed to extract archive: {e}")

    async def check_for_updates(self) -> Optional[str]:
        """
        Check if newer version is available.

        Returns:
            New version string if available, None otherwise
        """
        current = self._get_current_version()
        if not current:
            return await self._fetch_latest_version()

        latest = await self._fetch_latest_version()
        if self._should_update(current, latest):
            return latest

        return None

    async def update_binary(self) -> bool:
        """
        Update to latest version.

        Returns:
            True if updated, False if already on latest
        """
        latest = await self.check_for_updates()
        if not latest:
            logger.info("already_on_latest_version")
            return False

        logger.info("updating_to_latest", version=latest)
        await self._download_binary(latest)
        return True

    def get_binary_info(self) -> dict:
        """Get information about installed binary."""
        return {
            "path": str(self.agent_runtime_path),
            "exists": self.agent_runtime_path.exists(),
            "executable": os.access(self.agent_runtime_path, os.X_OK) if self.agent_runtime_path.exists() else False,
            "version": self._get_current_version(),
            "size": self.agent_runtime_path.stat().st_size if self.agent_runtime_path.exists() else 0,
        }
