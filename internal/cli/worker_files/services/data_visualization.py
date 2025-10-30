"""
Data Visualization Tools for Agent Control Plane Worker

This module provides tools for agents to create diagrams and visualizations
using Mermaid syntax. The tools emit special events that the UI can parse
and render as interactive diagrams.

Event Format:
    The tool emits events in the following format:

    <<DIAGRAM_START>>
    ```mermaid
    [mermaid diagram code]
    ```
    <<DIAGRAM_END>>

This format allows the UI to:
1. Detect diagram events via <<DIAGRAM_START>> and <<DIAGRAM_END>> markers
2. Extract the Mermaid syntax between the markers
3. Render the diagram using a Mermaid renderer component
"""

import json
import re
from typing import Optional, Callable, Any, Dict, List
from agno.tools import Toolkit
import logging

logger = logging.getLogger(__name__)


# Mermaid syntax validation patterns
MERMAID_DIAGRAM_TYPES = {
    'flowchart': r'^flowchart\s+(TD|TB|BT|RL|LR)',
    'graph': r'^graph\s+(TD|TB|BT|RL|LR)',
    'sequenceDiagram': r'^sequenceDiagram',
    'classDiagram': r'^classDiagram',
    'stateDiagram': r'^stateDiagram(-v2)?',
    'erDiagram': r'^erDiagram',
    'journey': r'^journey',
    'gantt': r'^gantt',
    'pie': r'^pie',
    'gitGraph': r'^gitGraph',
    'mindmap': r'^mindmap',
    'timeline': r'^timeline',
    'quadrantChart': r'^quadrantChart',
}

# Forbidden patterns that could cause rendering issues
FORBIDDEN_PATTERNS = [
    r'<script',  # No script tags
    r'javascript:',  # No javascript: URLs
    r'on\w+\s*=',  # No event handlers
    r'eval\(',  # No eval
    r'\.innerHTML',  # No innerHTML manipulation
]


class MermaidValidator:
    """Validates Mermaid diagram syntax to ensure safe and correct rendering."""

    @staticmethod
    def validate_syntax(diagram_code: str, expected_type: Optional[str] = None) -> tuple[bool, Optional[str]]:
        """
        Validate Mermaid diagram syntax.

        Args:
            diagram_code: The Mermaid diagram code to validate
            expected_type: Optional expected diagram type (e.g., 'flowchart', 'sequenceDiagram')

        Returns:
            Tuple of (is_valid, error_message)
        """
        if not diagram_code or not diagram_code.strip():
            return False, "Diagram code is empty"

        # Check for forbidden patterns
        for pattern in FORBIDDEN_PATTERNS:
            if re.search(pattern, diagram_code, re.IGNORECASE):
                return False, f"Forbidden pattern detected: {pattern}"

        # Check for valid diagram type
        diagram_lines = [line.strip() for line in diagram_code.strip().split('\n') if line.strip()]
        if not diagram_lines:
            return False, "No content in diagram"

        # Skip metadata lines (---...---)
        first_code_line = None
        in_metadata = False
        for line in diagram_lines:
            if line == '---':
                in_metadata = not in_metadata
                continue
            if not in_metadata and line:
                first_code_line = line
                break

        if not first_code_line:
            return False, "No diagram definition found"

        # Validate diagram type
        found_valid_type = False
        detected_type = None
        for diagram_type, pattern in MERMAID_DIAGRAM_TYPES.items():
            if re.match(pattern, first_code_line, re.IGNORECASE):
                found_valid_type = True
                detected_type = diagram_type
                break

        if not found_valid_type:
            return False, f"Invalid diagram type. First line: {first_code_line[:50]}"

        # If expected type provided, verify it matches
        if expected_type and detected_type:
            if expected_type.lower() not in detected_type.lower():
                return False, f"Expected {expected_type} but detected {detected_type}"

        # Check for balanced brackets/parentheses (basic structural validation)
        open_chars = {'(': 0, '[': 0, '{': 0}
        close_chars = {')': '(', ']': '[', '}': '{'}
        stack = []

        for char in diagram_code:
            if char in open_chars:
                stack.append(char)
            elif char in close_chars:
                if not stack or stack[-1] != close_chars[char]:
                    # Don't fail on unbalanced - just warn
                    logger.warning(f"Potentially unbalanced brackets in diagram")
                elif stack:
                    stack.pop()

        return True, None

    @staticmethod
    def sanitize_content(content: str) -> str:
        """
        Sanitize diagram content to remove potentially problematic characters.

        Args:
            content: Content to sanitize

        Returns:
            Sanitized content
        """
        # Remove null bytes
        content = content.replace('\x00', '')

        # Normalize line endings
        content = content.replace('\r\n', '\n').replace('\r', '\n')

        # Remove excessive whitespace while preserving structure
        lines = content.split('\n')
        cleaned_lines = []
        for line in lines:
            # Preserve indentation but trim trailing spaces
            cleaned_line = line.rstrip()
            cleaned_lines.append(cleaned_line)

        return '\n'.join(cleaned_lines)


class DataVisualizationTools(Toolkit):
    """
    Data Visualization toolkit for creating diagrams using Mermaid syntax.

    Agents can use these tools to create various types of diagrams for:
    - Data analysis and BI intelligence
    - System architecture visualization
    - Process flows and workflows
    - Database schemas and ER diagrams
    - Project timelines and Gantt charts
    """

    def __init__(
        self,
        max_diagram_size: int = 50000,
        enable_flowchart: bool = True,
        enable_sequence: bool = True,
        enable_class_diagram: bool = True,
        enable_er_diagram: bool = True,
        enable_gantt: bool = True,
        enable_pie_chart: bool = True,
        enable_state_diagram: bool = True,
        enable_git_graph: bool = True,
        enable_user_journey: bool = True,
        enable_quadrant_chart: bool = True,
        stream_callback: Optional[Callable[[str], None]] = None,
    ):
        """
        Initialize DataVisualizationTools.

        Args:
            max_diagram_size: Maximum size of diagram in characters
            enable_*: Enable/disable specific diagram types
            stream_callback: Optional callback for streaming output
        """
        super().__init__(name="data_visualization")

        self.max_diagram_size = max_diagram_size
        self.enable_flowchart = enable_flowchart
        self.enable_sequence = enable_sequence
        self.enable_class_diagram = enable_class_diagram
        self.enable_er_diagram = enable_er_diagram
        self.enable_gantt = enable_gantt
        self.enable_pie_chart = enable_pie_chart
        self.enable_state_diagram = enable_state_diagram
        self.enable_git_graph = enable_git_graph
        self.enable_user_journey = enable_user_journey
        self.enable_quadrant_chart = enable_quadrant_chart
        self.stream_callback = stream_callback

        # Register all enabled tools
        if enable_flowchart:
            self.register(self.create_flowchart)
        if enable_sequence:
            self.register(self.create_sequence_diagram)
        if enable_class_diagram:
            self.register(self.create_class_diagram)
        if enable_er_diagram:
            self.register(self.create_er_diagram)
        if enable_gantt:
            self.register(self.create_gantt_chart)
        if enable_pie_chart:
            self.register(self.create_pie_chart)
        if enable_state_diagram:
            self.register(self.create_state_diagram)
        if enable_git_graph:
            self.register(self.create_git_graph)
        if enable_user_journey:
            self.register(self.create_user_journey)
        if enable_quadrant_chart:
            self.register(self.create_quadrant_chart)

        # Generic create_diagram tool
        self.register(self.create_diagram)

    def _emit_diagram(
        self,
        diagram_code: str,
        diagram_type: str,
        validate: bool = True,
        auto_fix: bool = True
    ) -> str:
        """
        Emit a diagram event with proper formatting for UI parsing.
        Includes validation, sanitization, and error handling.

        Args:
            diagram_code: Mermaid diagram code
            diagram_type: Type of diagram (for logging/metadata)
            validate: Whether to validate the diagram syntax (default: True)
            auto_fix: Whether to attempt auto-fixing common issues (default: True)

        Returns:
            Formatted diagram event string or error message
        """
        try:
            # Sanitize content
            sanitized_code = MermaidValidator.sanitize_content(diagram_code)

            # Validate size
            if len(sanitized_code) > self.max_diagram_size:
                error_msg = f"Error: Diagram exceeds maximum size of {self.max_diagram_size} characters (current: {len(sanitized_code)})"
                logger.error(f"[DataVisualization] {error_msg}")
                return error_msg

            # Validate syntax if enabled
            if validate:
                is_valid, error_message = MermaidValidator.validate_syntax(
                    sanitized_code,
                    expected_type=diagram_type
                )

                if not is_valid:
                    if auto_fix:
                        # Attempt to fix common issues
                        fixed_code = self._attempt_fix(sanitized_code, diagram_type)
                        if fixed_code:
                            # Re-validate fixed code
                            is_valid, error_message = MermaidValidator.validate_syntax(
                                fixed_code,
                                expected_type=diagram_type
                            )
                            if is_valid:
                                logger.info(f"[DataVisualization] Auto-fixed diagram syntax issue")
                                sanitized_code = fixed_code
                            else:
                                error_msg = f"Error: Invalid diagram syntax - {error_message}\n\nAttempted auto-fix failed. Please check your Mermaid syntax."
                                logger.error(f"[DataVisualization] {error_msg}")
                                return error_msg
                    else:
                        error_msg = f"Error: Invalid diagram syntax - {error_message}"
                        logger.error(f"[DataVisualization] {error_msg}")
                        return error_msg

            # Log successful validation
            logger.info(
                f"[DataVisualization] Emitting {diagram_type} diagram "
                f"({len(sanitized_code)} chars)"
            )

            # Format the diagram event
            output = f"""<<DIAGRAM_START>>
```mermaid
{sanitized_code.strip()}
```
<<DIAGRAM_END>>"""

            # Stream if callback provided
            if self.stream_callback:
                try:
                    self.stream_callback(output)
                except Exception as stream_error:
                    logger.error(
                        f"[DataVisualization] Stream callback failed: {stream_error}"
                    )
                    # Continue anyway - the output is still returned

            return output

        except Exception as e:
            error_msg = f"Error: Failed to emit diagram - {str(e)}"
            logger.error(f"[DataVisualization] {error_msg}", exc_info=True)
            return error_msg

    def _attempt_fix(self, diagram_code: str, diagram_type: str) -> Optional[str]:
        """
        Attempt to auto-fix common diagram syntax issues.

        Args:
            diagram_code: The diagram code with issues
            diagram_type: The type of diagram

        Returns:
            Fixed diagram code or None if can't be fixed
        """
        try:
            lines = diagram_code.split('\n')
            fixed_lines = []

            for line in lines:
                # Remove trailing semicolons (common mistake)
                if line.rstrip().endswith(';'):
                    line = line.rstrip()[:-1]

                # Fix common arrow syntax issues
                line = line.replace('-->', '-->').replace('--->', '-->')

                fixed_lines.append(line)

            return '\n'.join(fixed_lines)
        except Exception as e:
            logger.warning(f"[DataVisualization] Auto-fix failed: {e}")
            return None

    def create_diagram(
        self, diagram_code: str, diagram_type: str = "flowchart"
    ) -> str:
        """
        Create a diagram from Mermaid syntax.

        Args:
            diagram_code: Complete Mermaid diagram code
            diagram_type: Type of diagram (for metadata)

        Returns:
            Success message with diagram event

        Example:
            create_diagram('''
            flowchart TD
                A[Start] --> B{Decision}
                B -->|Yes| C[Action 1]
                B -->|No| D[Action 2]
            ''', 'flowchart')
        """
        return self._emit_diagram(diagram_code, diagram_type)

    def create_flowchart(
        self,
        title: str,
        nodes: str,
        direction: str = "TD",
    ) -> str:
        """
        Create a flowchart diagram with automatic validation and error recovery.

        Args:
            title: Title of the flowchart
            nodes: Mermaid flowchart node definitions
            direction: Direction (TD=top-down, LR=left-right, RL=right-left, BT=bottom-top)

        Returns:
            Success message with diagram event or error message

        Example:
            create_flowchart(
                title="User Login Flow",
                nodes='''
                A[User] --> B[Login Page]
                B --> C{Valid?}
                C -->|Yes| D[Dashboard]
                C -->|No| E[Error]
                ''',
                direction="TD"
            )
        """
        try:
            # Validate direction parameter
            valid_directions = ['TD', 'TB', 'BT', 'RL', 'LR']
            direction = direction.upper()
            if direction not in valid_directions:
                logger.warning(
                    f"[DataVisualization] Invalid direction '{direction}', "
                    f"defaulting to 'TD'. Valid: {valid_directions}"
                )
                direction = 'TD'

            # Sanitize title
            title = title.replace('"', "'").strip()
            if not title:
                title = "Flowchart"

            # Build diagram
            diagram = f"""---
title: {title}
---
flowchart {direction}
{nodes.strip()}"""

            return self._emit_diagram(diagram, "flowchart")

        except Exception as e:
            error_msg = f"Error creating flowchart: {str(e)}"
            logger.error(f"[DataVisualization] {error_msg}", exc_info=True)
            return error_msg

    def create_sequence_diagram(
        self,
        title: str,
        participants: list[str],
        interactions: str,
    ) -> str:
        """
        Create a sequence diagram.

        Args:
            title: Title of the sequence diagram
            participants: List of participant names
            interactions: Mermaid sequence diagram interactions

        Returns:
            Success message with diagram event

        Example:
            create_sequence_diagram(
                title="API Authentication Flow",
                participants=["Client", "API", "Database"],
                interactions='''
                Client->>API: POST /login
                API->>Database: Verify credentials
                Database-->>API: User data
                API-->>Client: JWT token
                '''
            )
        """
        participant_defs = "\n".join([f"    participant {p}" for p in participants])
        diagram = f"""---
title: {title}
---
sequenceDiagram
{participant_defs}
{interactions.strip()}"""
        return self._emit_diagram(diagram, "sequence")

    def create_class_diagram(
        self,
        title: str,
        classes: str,
    ) -> str:
        """
        Create a class diagram.

        Args:
            title: Title of the class diagram
            classes: Mermaid class diagram definitions

        Returns:
            Success message with diagram event

        Example:
            create_class_diagram(
                title="User Management System",
                classes='''
                class User {
                    +String name
                    +String email
                    +login()
                    +logout()
                }
                class Admin {
                    +String permissions
                    +deleteUser()
                }
                User <|-- Admin
                '''
            )
        """
        diagram = f"""---
title: {title}
---
classDiagram
{classes.strip()}"""
        return self._emit_diagram(diagram, "class")

    def create_er_diagram(
        self,
        title: str,
        entities: str,
    ) -> str:
        """
        Create an Entity-Relationship diagram.

        Args:
            title: Title of the ER diagram
            entities: Mermaid ER diagram definitions

        Returns:
            Success message with diagram event

        Example:
            create_er_diagram(
                title="E-commerce Database Schema",
                entities='''
                CUSTOMER ||--o{ ORDER : places
                ORDER ||--|{ LINE-ITEM : contains
                PRODUCT ||--o{ LINE-ITEM : includes
                '''
            )
        """
        diagram = f"""---
title: {title}
---
erDiagram
{entities.strip()}"""
        return self._emit_diagram(diagram, "er")

    def create_gantt_chart(
        self,
        title: str,
        tasks: str,
        date_format: str = "YYYY-MM-DD",
    ) -> str:
        """
        Create a Gantt chart.

        Args:
            title: Title of the Gantt chart
            tasks: Mermaid Gantt chart task definitions
            date_format: Date format (default: YYYY-MM-DD)

        Returns:
            Success message with diagram event

        Example:
            create_gantt_chart(
                title="Project Timeline",
                tasks='''
                section Planning
                Requirements : 2024-01-01, 2w
                Design : 2024-01-15, 1w
                section Development
                Backend : 2024-01-22, 3w
                Frontend : 2024-02-05, 3w
                '''
            )
        """
        diagram = f"""---
title: {title}
---
gantt
    dateFormat {date_format}
{tasks.strip()}"""
        return self._emit_diagram(diagram, "gantt")

    def create_pie_chart(
        self,
        title: str,
        data: dict[str, float],
    ) -> str:
        """
        Create a pie chart with data validation.

        Args:
            title: Title of the pie chart
            data: Dictionary of label: value pairs

        Returns:
            Success message with diagram event or error message

        Example:
            create_pie_chart(
                title="Revenue by Product",
                data={
                    "Product A": 35.5,
                    "Product B": 28.3,
                    "Product C": 20.1,
                    "Product D": 16.1
                }
            )
        """
        try:
            # Validate data
            if not data or not isinstance(data, dict):
                return "Error: Pie chart data must be a non-empty dictionary"

            if len(data) == 0:
                return "Error: Pie chart must have at least one data point"

            if len(data) > 50:
                logger.warning(
                    f"[DataVisualization] Pie chart has {len(data)} slices, "
                    "which may be hard to read. Consider grouping data."
                )

            # Validate and sanitize data
            validated_data = {}
            for label, value in data.items():
                # Sanitize label
                clean_label = str(label).replace('"', "'").strip()
                if not clean_label:
                    clean_label = "Unnamed"

                # Validate value
                try:
                    numeric_value = float(value)
                    if numeric_value < 0:
                        logger.warning(
                            f"[DataVisualization] Negative value for '{clean_label}': {numeric_value}, "
                            "using absolute value"
                        )
                        numeric_value = abs(numeric_value)
                    validated_data[clean_label] = numeric_value
                except (ValueError, TypeError):
                    logger.warning(
                        f"[DataVisualization] Invalid value for '{clean_label}': {value}, skipping"
                    )
                    continue

            if not validated_data:
                return "Error: No valid data points after validation"

            # Sanitize title
            title = title.replace('"', "'").strip()
            if not title:
                title = "Pie Chart"

            # Build diagram
            data_lines = "\n".join(
                [f'    "{label}" : {value}' for label, value in validated_data.items()]
            )
            diagram = f"""---
title: {title}
---
pie
{data_lines}"""

            return self._emit_diagram(diagram, "pie")

        except Exception as e:
            error_msg = f"Error creating pie chart: {str(e)}"
            logger.error(f"[DataVisualization] {error_msg}", exc_info=True)
            return error_msg

    def create_state_diagram(
        self,
        title: str,
        states: str,
    ) -> str:
        """
        Create a state diagram.

        Args:
            title: Title of the state diagram
            states: Mermaid state diagram definitions

        Returns:
            Success message with diagram event

        Example:
            create_state_diagram(
                title="Order Processing States",
                states='''
                [*] --> Pending
                Pending --> Processing : Payment received
                Processing --> Shipped : Items packed
                Shipped --> Delivered : Delivery confirmed
                Delivered --> [*]
                Processing --> Cancelled : Cancel request
                Cancelled --> [*]
                '''
            )
        """
        diagram = f"""---
title: {title}
---
stateDiagram-v2
{states.strip()}"""
        return self._emit_diagram(diagram, "state")

    def create_git_graph(
        self,
        title: str,
        commits: str,
    ) -> str:
        """
        Create a Git graph diagram.

        Args:
            title: Title of the Git graph
            commits: Mermaid Git graph commit definitions

        Returns:
            Success message with diagram event

        Example:
            create_git_graph(
                title="Feature Branch Workflow",
                commits='''
                commit
                branch develop
                checkout develop
                commit
                branch feature
                checkout feature
                commit
                commit
                checkout develop
                merge feature
                checkout main
                merge develop
                '''
            )
        """
        diagram = f"""---
title: {title}
---
gitGraph
{commits.strip()}"""
        return self._emit_diagram(diagram, "git")

    def create_user_journey(
        self,
        title: str,
        journey: str,
    ) -> str:
        """
        Create a user journey diagram.

        Args:
            title: Title of the user journey
            journey: Mermaid user journey definitions

        Returns:
            Success message with diagram event

        Example:
            create_user_journey(
                title="Customer Onboarding Journey",
                journey='''
                section Sign up
                  Visit website: 5: User
                  Create account: 3: User
                section Getting Started
                  Complete profile: 4: User
                  First purchase: 5: User
                '''
            )
        """
        diagram = f"""---
title: {title}
---
journey
{journey.strip()}"""
        return self._emit_diagram(diagram, "journey")

    def create_quadrant_chart(
        self,
        title: str,
        x_axis: str,
        y_axis: str,
        items: str,
    ) -> str:
        """
        Create a quadrant chart.

        Args:
            title: Title of the quadrant chart
            x_axis: X-axis label (left --> right)
            y_axis: Y-axis label (bottom --> top)
            items: Mermaid quadrant chart item definitions

        Returns:
            Success message with diagram event

        Example:
            create_quadrant_chart(
                title="Product Priority Matrix",
                x_axis="Effort",
                y_axis="Impact",
                items='''
                Feature A: [0.8, 0.9]
                Feature B: [0.3, 0.7]
                Feature C: [0.6, 0.4]
                Feature D: [0.2, 0.2]
                '''
            )
        """
        diagram = f"""---
title: {title}
---
quadrantChart
    x-axis {x_axis}
    y-axis {y_axis}
{items.strip()}"""
        return self._emit_diagram(diagram, "quadrant")
