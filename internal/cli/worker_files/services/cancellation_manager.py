"""Cancellation manager - handles agent/team registry and cancellation"""

from typing import Dict, Any, Optional
from datetime import datetime, timezone
import structlog

logger = structlog.get_logger()


class CancellationManager:
    """
    Manages active agent/team instances for cancellation support.

    Provides a centralized registry and cancellation logic that works
    with Agno's cancel_run() API.
    """

    def __init__(self):
        # Key: execution_id, Value: {agent/team, run_id, started_at}
        self._registry: Dict[str, Dict[str, Any]] = {}

    def register(
        self,
        execution_id: str,
        instance: Any,  # Agent or Team
        instance_type: str = "agent"
    ) -> None:
        """
        Register an agent or team for cancellation support.

        Args:
            execution_id: Unique execution ID
            instance: Agno Agent or Team instance
            instance_type: "agent" or "team"
        """
        self._registry[execution_id] = {
            "instance": instance,
            "instance_type": instance_type,
            "run_id": None,  # Set when run starts
            "started_at": datetime.now(timezone.utc).isoformat(),
        }

        logger.info(
            f"{instance_type}_registered_for_cancellation",
            execution_id=execution_id[:8],
            instance_type=instance_type
        )

    def set_run_id(self, execution_id: str, run_id: str) -> None:
        """
        Set the Agno run_id for an execution (captured from first streaming chunk).

        Args:
            execution_id: Execution ID
            run_id: Agno run_id from streaming response
        """
        if execution_id in self._registry:
            self._registry[execution_id]["run_id"] = run_id
            logger.info(
                "run_id_captured",
                execution_id=execution_id[:8],
                run_id=run_id[:16]
            )

    def cancel(self, execution_id: str) -> Dict[str, Any]:
        """
        Cancel an active execution using Agno's cancel_run API.

        Args:
            execution_id: Execution to cancel

        Returns:
            Dict with success status and details
        """
        # Check if execution exists in registry
        if execution_id not in self._registry:
            logger.warning(
                "cancel_execution_not_found",
                execution_id=execution_id[:8]
            )
            return {
                "success": False,
                "error": "Execution not found or already completed",
                "execution_id": execution_id,
            }

        entry = self._registry[execution_id]
        instance = entry["instance"]
        run_id = entry.get("run_id")
        instance_type = entry.get("instance_type", "agent")

        # Check if run has started
        if not run_id:
            logger.warning(
                "cancel_no_run_id",
                execution_id=execution_id[:8]
            )
            return {
                "success": False,
                "error": "Execution not started yet",
                "execution_id": execution_id,
            }

        logger.info(
            f"cancelling_{instance_type}_run",
            execution_id=execution_id[:8],
            run_id=run_id[:16]
        )

        try:
            # Call Agno's cancel_run API
            success = instance.cancel_run(run_id)

            if success:
                logger.info(
                    f"{instance_type}_run_cancelled",
                    execution_id=execution_id[:8],
                    run_id=run_id[:16]
                )

                # Clean up registry
                del self._registry[execution_id]

                return {
                    "success": True,
                    "execution_id": execution_id,
                    "run_id": run_id,
                    "instance_type": instance_type,
                    "cancelled_at": datetime.now(timezone.utc).isoformat(),
                }
            else:
                logger.warning(
                    f"{instance_type}_cancel_failed",
                    execution_id=execution_id[:8],
                    run_id=run_id[:16]
                )
                return {
                    "success": False,
                    "error": "Cancel failed - run may be completed",
                    "execution_id": execution_id,
                    "run_id": run_id,
                }

        except Exception as e:
            logger.error(
                f"{instance_type}_cancel_error",
                execution_id=execution_id[:8],
                error=str(e)
            )
            return {
                "success": False,
                "error": str(e),
                "execution_id": execution_id,
            }

    def unregister(self, execution_id: str) -> None:
        """
        Remove an execution from the registry (called on completion).

        Args:
            execution_id: Execution to unregister
        """
        if execution_id in self._registry:
            instance_type = self._registry[execution_id].get("instance_type", "agent")
            del self._registry[execution_id]
            logger.info(
                f"{instance_type}_unregistered",
                execution_id=execution_id[:8]
            )

    def get_active_count(self) -> int:
        """Get number of active executions in registry."""
        return len(self._registry)


# Global singleton instance
cancellation_manager = CancellationManager()
