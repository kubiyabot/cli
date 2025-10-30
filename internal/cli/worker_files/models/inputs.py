"""Input dataclasses for all Temporal activities"""

from dataclasses import dataclass
from typing import Optional, List, Dict, Any


@dataclass
class AgentExecutionInput:
    """Input for agent execution activity"""
    execution_id: str
    agent_id: str
    organization_id: str
    prompt: str
    system_prompt: Optional[str] = None
    model_id: Optional[str] = None
    model_config: Optional[Dict[str, Any]] = None
    mcp_servers: Optional[Dict[str, Any]] = None
    session_id: Optional[str] = None
    user_id: Optional[str] = None

    def __post_init__(self):
        if self.model_config is None:
            self.model_config = {}
        if self.mcp_servers is None:
            self.mcp_servers = {}


@dataclass
class TeamExecutionInput:
    """Input for team execution activity"""
    execution_id: str
    team_id: str
    organization_id: str
    prompt: str
    system_prompt: Optional[str] = None
    model_id: Optional[str] = None
    model_config: Optional[Dict[str, Any]] = None
    mcp_servers: Optional[Dict[str, Any]] = None
    session_id: Optional[str] = None
    user_id: Optional[str] = None
    team_config: Optional[Dict[str, Any]] = None

    def __post_init__(self):
        if self.model_config is None:
            self.model_config = {}
        if self.mcp_servers is None:
            self.mcp_servers = {}
        if self.team_config is None:
            self.team_config = {}


@dataclass
class CancelExecutionInput:
    """Input for cancellation activity"""
    execution_id: str


@dataclass
class UpdateExecutionStatusInput:
    """Input for status update activity"""
    execution_id: str
    status: str
    started_at: Optional[str] = None
    completed_at: Optional[str] = None
    response: Optional[str] = None
    error_message: Optional[str] = None
    usage: Optional[Dict[str, Any]] = None
    execution_metadata: Optional[Dict[str, Any]] = None

    def __post_init__(self):
        if self.usage is None:
            self.usage = {}
        if self.execution_metadata is None:
            self.execution_metadata = {}


@dataclass
class UpdateAgentStatusInput:
    """Input for agent status update"""
    agent_id: str
    organization_id: str
    status: str
    last_active_at: str
    error_message: Optional[str] = None
    state: Optional[Dict[str, Any]] = None

    def __post_init__(self):
        if self.state is None:
            self.state = {}
