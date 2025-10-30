"""
Runtime factory for creating runtime instances.

This module provides a centralized factory for creating runtime instances
based on agent configuration.
"""

from typing import TYPE_CHECKING, List
import structlog

from .base import RuntimeType, AgentRuntime

if TYPE_CHECKING:
    from control_plane_client import ControlPlaneClient
    from services.cancellation_manager import CancellationManager

logger = structlog.get_logger(__name__)


class RuntimeFactory:
    """
    Factory for creating runtime instances based on agent configuration.

    This class centralizes runtime instantiation logic and provides
    discoverability of supported runtimes.
    """

    @staticmethod
    def create_runtime(
        runtime_type: RuntimeType,
        control_plane_client: "ControlPlaneClient",
        cancellation_manager: "CancellationManager",
        **kwargs,
    ) -> AgentRuntime:
        """
        Create a runtime instance.

        Args:
            runtime_type: Type of runtime to create
            control_plane_client: Client for Control Plane API
            cancellation_manager: Manager for execution cancellation
            **kwargs: Additional runtime-specific configuration

        Returns:
            AgentRuntime instance

        Raises:
            ValueError: If runtime_type is not supported

        Example:
            >>> factory = RuntimeFactory()
            >>> runtime = factory.create_runtime(
            ...     RuntimeType.DEFAULT,
            ...     control_plane_client,
            ...     cancellation_manager
            ... )
        """
        logger.info(
            "Creating runtime instance",
            runtime_type=runtime_type,
            has_kwargs=bool(kwargs),
        )

        # Import here to avoid circular dependencies
        if runtime_type == RuntimeType.DEFAULT:
            from .default_runtime import DefaultRuntime

            return DefaultRuntime(
                control_plane_client=control_plane_client,
                cancellation_manager=cancellation_manager,
                **kwargs,
            )
        elif runtime_type == RuntimeType.CLAUDE_CODE:
            from .claude_code_runtime import ClaudeCodeRuntime

            return ClaudeCodeRuntime(
                control_plane_client=control_plane_client,
                cancellation_manager=cancellation_manager,
                **kwargs,
            )
        else:
            raise ValueError(
                f"Unsupported runtime type: {runtime_type}. "
                f"Supported types: {RuntimeFactory.get_supported_runtimes()}"
            )

    @staticmethod
    def get_default_runtime_type() -> RuntimeType:
        """
        Get the default runtime type.

        This is used when no runtime is explicitly specified in agent config.

        Returns:
            Default RuntimeType (RuntimeType.DEFAULT)
        """
        return RuntimeType.DEFAULT

    @staticmethod
    def get_supported_runtimes() -> List[RuntimeType]:
        """
        Get list of supported runtimes.

        Returns:
            List of RuntimeType enum values
        """
        return [RuntimeType.DEFAULT, RuntimeType.CLAUDE_CODE]

    @staticmethod
    def parse_runtime_type(runtime_str: str) -> RuntimeType:
        """
        Parse runtime type from string with fallback to default.

        Args:
            runtime_str: Runtime type as string

        Returns:
            RuntimeType enum value, defaults to DEFAULT if invalid

        Example:
            >>> RuntimeFactory.parse_runtime_type("claude_code")
            RuntimeType.CLAUDE_CODE
            >>> RuntimeFactory.parse_runtime_type("invalid")
            RuntimeType.DEFAULT
        """
        try:
            return RuntimeType(runtime_str)
        except ValueError:
            logger.warning(
                "Invalid runtime type, using default",
                runtime_str=runtime_str,
                default=RuntimeType.DEFAULT.value,
            )
            return RuntimeType.DEFAULT

    @staticmethod
    def validate_runtime_config(
        runtime_type: RuntimeType, config: dict
    ) -> tuple[bool, str]:
        """
        Validate runtime-specific configuration.

        Args:
            runtime_type: Type of runtime
            config: Configuration dict to validate

        Returns:
            Tuple of (is_valid, error_message)

        Example:
            >>> is_valid, error = RuntimeFactory.validate_runtime_config(
            ...     RuntimeType.CLAUDE_CODE,
            ...     {"allowed_tools": ["Bash"]}
            ... )
        """
        # Runtime-specific validation logic
        if runtime_type == RuntimeType.CLAUDE_CODE:
            # Validate Claude Code specific config
            if "cwd" in config and not isinstance(config["cwd"], str):
                return False, "cwd must be a string"

        return True, ""
