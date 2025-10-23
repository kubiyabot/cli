"""Toolset factory - instantiates Agno toolsets from Control Plane configuration"""

from typing import Optional, Any, List
import structlog

from agno.tools.shell import ShellTools
from agno.tools.python import PythonTools
from agno.tools.file import FileTools

logger = structlog.get_logger()


class ToolsetFactory:
    """
    Factory for creating Agno toolkit instances from Control Plane toolset configurations.

    Centralizes toolset instantiation logic that was previously duplicated
    in agent_activities.py and team_activities.py.
    """

    @staticmethod
    def create_toolset(toolset_data: dict) -> Optional[Any]:
        """
        Create an Agno toolkit from Control Plane configuration.

        Args:
            toolset_data: Toolset config from Control Plane API:
                - type: Toolset type (file_system, shell, python, etc.)
                - name: Toolset name
                - configuration: Dict with toolset-specific config
                - enabled: Whether toolset is enabled

        Returns:
            Instantiated Agno toolkit or None if disabled/unsupported
        """
        if not toolset_data.get("enabled", True):
            logger.info(
                "toolset_skipped_disabled",
                toolset_name=toolset_data.get("name")
            )
            return None

        toolset_type = toolset_data.get("type", "").lower()
        config = toolset_data.get("configuration", {})
        name = toolset_data.get("name", "Unknown")

        try:
            # File system tools
            if toolset_type in ["file_system", "file", "file_generation"]:
                base_dir = config.get("base_directory", "/workspace")
                return FileTools(base_dir=base_dir)

            # Shell/terminal tools
            elif toolset_type in ["shell", "terminal", "bash"]:
                return ShellTools()

            # Python tools
            elif toolset_type in ["python", "python_code"]:
                return PythonTools()

            else:
                logger.warning(
                    "toolset_type_not_supported",
                    toolset_type=toolset_type,
                    toolset_name=name
                )
                return None

        except Exception as e:
            logger.error(
                "toolset_instantiation_failed",
                toolset_type=toolset_type,
                toolset_name=name,
                error=str(e)
            )
            return None

    @classmethod
    def create_toolsets_from_list(
        cls,
        toolset_configs: List[dict]
    ) -> List[Any]:
        """
        Create multiple toolsets from a list of configurations.

        Args:
            toolset_configs: List of toolset config dicts

        Returns:
            List of instantiated toolsets (non-None)
        """
        toolsets = []

        for config in toolset_configs:
            toolset = cls.create_toolset(config)
            if toolset:
                toolsets.append(toolset)

        logger.info(
            "toolsets_created",
            requested_count=len(toolset_configs),
            created_count=len(toolsets)
        )

        return toolsets
