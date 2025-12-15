package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/contextgraph"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

// newGraphCommand creates the graph command with subcommands
func newGraphCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Manage and query the context graph",
		Long: `Access the organizational context graph to discover relationships between resources.

The context graph stores information about:
- Users and teams
- Cloud resources (AWS, Azure, GCP)
- Integrations (Slack, GitHub, etc.)
- Relationships between all entities

Use this to understand your organization's infrastructure and context.`,
		Example: `  # Get graph statistics
  kubiya graph stats

  # List all nodes
  kubiya graph nodes list

  # Search for specific nodes
  kubiya graph nodes search --label User --property email --value john@example.com

  # Get a specific node
  kubiya graph nodes get <node-id>

  # Get node relationships
  kubiya graph nodes relationships <node-id>

  # List integrations
  kubiya graph integrations list`,
	}

	// Add subcommands
	cmd.AddCommand(
		newGraphStatsCommand(cfg),
		newGraphNodesCommand(cfg),
		newGraphLabelsCommand(cfg),
		newGraphRelationshipTypesCommand(cfg),
		newGraphIntegrationsCommand(cfg),
		newGraphSubgraphCommand(cfg),
		newGraphQueryCommand(cfg),
		newGraphSearchCommand(cfg),
	)

	return cmd
}

// newGraphStatsCommand creates the stats command
func newGraphStatsCommand(cfg *config.Config) *cobra.Command {
	var (
		integration string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Get statistics about the context graph",
		Long:  `Retrieve statistics about the context graph including node count, relationship count, labels, and relationship types.`,
		Example: `  # Get overall stats
  kubiya graph stats

  # Get stats for a specific integration
  kubiya graph stats --integration Azure

  # Get stats in JSON format
  kubiya graph stats --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			stats, err := client.GetGraphStats(integration)
			if err != nil {
				return fmt.Errorf("failed to get graph stats: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(stats)
			}

			// Pretty print stats
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render("Context Graph Statistics"))
			fmt.Println()

			if integration != "" {
				fmt.Printf("  %s Integration: %s\n", style.RobotIconStyle.Render("🔌"), style.HighlightStyle.Render(integration))
				fmt.Println()
			}

			fmt.Printf("  %s Total Nodes: %s\n", style.RobotIconStyle.Render("📊"), style.HighlightStyle.Render(fmt.Sprintf("%d", stats.TotalNodes)))
			fmt.Printf("  %s Total Relationships: %s\n", style.RobotIconStyle.Render("🔗"), style.HighlightStyle.Render(fmt.Sprintf("%d", stats.TotalRelationships)))
			fmt.Println()

			if len(stats.Labels) > 0 {
				fmt.Printf("  %s Node Labels (%d):\n", style.RobotIconStyle.Render("🏷️"), len(stats.Labels))
				for _, label := range stats.Labels {
					fmt.Printf("    %s %s\n", style.BulletStyle.Render("•"), label)
				}
				fmt.Println()
			}

			if len(stats.RelationshipTypes) > 0 {
				fmt.Printf("  %s Relationship Types (%d):\n", style.RobotIconStyle.Render("🔗"), len(stats.RelationshipTypes))
				for _, relType := range stats.RelationshipTypes {
					fmt.Printf("    %s %s\n", style.BulletStyle.Render("•"), relType)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration (e.g., Azure, AWS, Slack)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphNodesCommand creates the nodes command group
func newGraphNodesCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage and query graph nodes",
		Long:  `Query and explore nodes in the context graph.`,
	}

	cmd.AddCommand(
		newGraphNodesListCommand(cfg),
		newGraphNodesGetCommand(cfg),
		newGraphNodesSearchCommand(cfg),
		newGraphNodesSearchTextCommand(cfg),
		newGraphNodesRelationshipsCommand(cfg),
	)

	return cmd
}

// newGraphNodesListCommand creates the nodes list command
func newGraphNodesListCommand(cfg *config.Config) *cobra.Command {
	var (
		integration string
		limit       int
		skip        int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all nodes in the graph",
		Example: `  # List first 10 nodes
  kubiya graph nodes list --limit 10

  # List nodes from Azure integration
  kubiya graph nodes list --integration Azure

  # List in JSON format
  kubiya graph nodes list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			nodes, err := client.ListNodes(integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to list nodes: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(nodes)
			}

			// Pretty print nodes
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Context Graph Nodes (%d)", len(nodes))))
			fmt.Println()

			for i, node := range nodes {
				fmt.Printf("%s Node %d\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), i+1)
				fmt.Printf("  ID: %s\n", style.HighlightStyle.Render(node.ID))
				fmt.Printf("  Labels: %s\n", strings.Join(node.Labels, ", "))
				if len(node.Properties) > 0 {
					fmt.Printf("  Properties:\n")
					for key, value := range node.Properties {
						fmt.Printf("    %s: %v\n", key, value)
					}
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of nodes to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of nodes to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphNodesGetCommand creates the nodes get command
func newGraphNodesGetCommand(cfg *config.Config) *cobra.Command {
	var (
		integration string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "get <node-id>",
		Short: "Get a specific node by ID",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get a node
  kubiya graph nodes get node-123

  # Get node in JSON format
  kubiya graph nodes get node-123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			node, err := client.GetNode(nodeID, integration)
			if err != nil {
				return fmt.Errorf("failed to get node: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(node)
			}

			// Pretty print node
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render("Node Details"))
			fmt.Println()
			fmt.Printf("  %s ID: %s\n", style.RobotIconStyle.Render("🔑"), style.HighlightStyle.Render(node.ID))
			fmt.Printf("  %s Labels: %s\n", style.RobotIconStyle.Render("🏷️"), strings.Join(node.Labels, ", "))
			fmt.Println()

			if len(node.Properties) > 0 {
				fmt.Println(style.DimStyle.Render("  Properties:"))
				for key, value := range node.Properties {
					fmt.Printf("    %s %s: %v\n", style.BulletStyle.Render("•"), key, value)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphNodesSearchCommand creates the nodes search command
func newGraphNodesSearchCommand(cfg *config.Config) *cobra.Command {
	var (
		label        string
		property     string
		value        string
		integration  string
		limit        int
		skip         int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for nodes by label and properties",
		Example: `  # Search for users
  kubiya graph nodes search --label User

  # Search for active users
  kubiya graph nodes search --label User --property active --value true

  # Search in specific integration
  kubiya graph nodes search --label Team --integration Slack`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &controlplane.NodeSearchRequest{
				Label:        label,
				PropertyName: property,
			}
			if value != "" {
				req.PropertyValue = value
			}

			
			nodes, err := client.SearchNodes(req, integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to search nodes: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(nodes)
			}

			// Pretty print results
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Search Results (%d nodes)", len(nodes))))
			fmt.Println()

			for i, node := range nodes {
				fmt.Printf("%s %s\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), style.HighlightStyle.Render(node.ID))
				fmt.Printf("  Labels: %s\n", strings.Join(node.Labels, ", "))
				if name, ok := node.Properties["name"].(string); ok {
					fmt.Printf("  Name: %s\n", name)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Node label to filter by")
	cmd.Flags().StringVar(&property, "property", "", "Property name to filter by")
	cmd.Flags().StringVar(&value, "value", "", "Property value to match")
	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of nodes to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of nodes to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphNodesSearchTextCommand creates the nodes search-text command
func newGraphNodesSearchTextCommand(cfg *config.Config) *cobra.Command {
	var (
		property     string
		text         string
		label        string
		integration  string
		limit        int
		skip         int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "search-text",
		Short: "Search for nodes by text in properties",
		Example: `  # Search for users with 'john' in email
  kubiya graph nodes search-text --property email --text john --label User

  # Search for teams with 'eng' in name
  kubiya graph nodes search-text --property name --text eng --label Team`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if property == "" || text == "" {
				return fmt.Errorf("--property and --text are required")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &controlplane.TextSearchRequest{
				PropertyName: property,
				SearchText:   text,
				Label:        label,
			}

			
			nodes, err := client.SearchNodesByText(req, integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to search nodes: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(nodes)
			}

			// Pretty print results
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Text Search Results (%d nodes)", len(nodes))))
			fmt.Println()

			for i, node := range nodes {
				fmt.Printf("%s %s\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), style.HighlightStyle.Render(node.ID))
				fmt.Printf("  Labels: %s\n", strings.Join(node.Labels, ", "))
				if val, ok := node.Properties[property]; ok {
					fmt.Printf("  %s: %v\n", property, val)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&property, "property", "", "Property name to search in (required)")
	cmd.Flags().StringVar(&text, "text", "", "Text to search for (required)")
	cmd.Flags().StringVar(&label, "label", "", "Node label to filter by")
	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of nodes to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of nodes to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphNodesRelationshipsCommand creates the nodes relationships command
func newGraphNodesRelationshipsCommand(cfg *config.Config) *cobra.Command {
	var (
		direction    string
		relType      string
		integration  string
		limit        int
		skip         int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "relationships <node-id>",
		Short: "Get relationships for a node",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get all relationships
  kubiya graph nodes relationships node-123

  # Get outgoing relationships only
  kubiya graph nodes relationships node-123 --direction outgoing

  # Get specific relationship type
  kubiya graph nodes relationships node-123 --type MEMBER_OF`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			relationships, err := client.GetNodeRelationships(nodeID, direction, relType, integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to get relationships: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(relationships)
			}

			// Pretty print relationships
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Node Relationships (%d)", len(relationships))))
			fmt.Println()

			for i, rel := range relationships {
				fmt.Printf("%s Relationship %d\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), i+1)
				fmt.Printf("  ID: %s\n", rel.ID)
				fmt.Printf("  Type: %s\n", style.HighlightStyle.Render(rel.Type))
				fmt.Printf("  %s %s → %s\n",
					style.RobotIconStyle.Render("🔗"),
					rel.StartNode,
					rel.EndNode)
				if len(rel.Properties) > 0 {
					fmt.Printf("  Properties: %v\n", rel.Properties)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&direction, "direction", "both", "Direction of relationships (both, incoming, outgoing)")
	cmd.Flags().StringVar(&relType, "type", "", "Filter by relationship type")
	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of relationships to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of relationships to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphLabelsCommand creates the labels command
func newGraphLabelsCommand(cfg *config.Config) *cobra.Command {
	var (
		integration string
		limit       int
		skip        int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "labels",
		Short: "List all node labels in the graph",
		Example: `  # List all labels
  kubiya graph labels

  # List labels for specific integration
  kubiya graph labels --integration Azure`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			labels, err := client.ListLabels(integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to list labels: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(labels)
			}

			// Pretty print labels
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Node Labels (%d)", len(labels))))
			fmt.Println()

			for _, label := range labels {
				fmt.Printf("  %s %s\n", style.BulletStyle.Render("•"), label)
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of labels to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of labels to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphRelationshipTypesCommand creates the relationship-types command
func newGraphRelationshipTypesCommand(cfg *config.Config) *cobra.Command {
	var (
		integration string
		limit       int
		skip        int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "relationship-types",
		Short: "List all relationship types in the graph",
		Example: `  # List all relationship types
  kubiya graph relationship-types

  # List for specific integration
  kubiya graph relationship-types --integration Slack`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			types, err := client.ListRelationshipTypes(integration, skip, limit)
			if err != nil {
				return fmt.Errorf("failed to list relationship types: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(types)
			}

			// Pretty print types
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Relationship Types (%d)", len(types))))
			fmt.Println()

			for _, relType := range types {
				fmt.Printf("  %s %s\n", style.BulletStyle.Render("•"), relType)
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of types to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of types to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphIntegrationsCommand creates the integrations command
func newGraphIntegrationsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage graph integrations",
	}

	cmd.AddCommand(newGraphIntegrationsListCommand(cfg))

	return cmd
}

// newGraphIntegrationsListCommand creates the integrations list command
func newGraphIntegrationsListCommand(cfg *config.Config) *cobra.Command {
	var (
		limit       int
		skip        int
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available integrations",
		Example: `  # List all integrations
  kubiya graph integrations list

  # List in JSON format
  kubiya graph integrations list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			
			integrations, err := client.ListIntegrations(skip, limit)
			if err != nil {
				return fmt.Errorf("failed to list integrations: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(integrations)
			}

			// Pretty print integrations
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Available Integrations (%d)", len(integrations))))
			fmt.Println()

			for i, integration := range integrations {
				fmt.Printf("%s %s\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), style.HighlightStyle.Render(integration.Name))
				fmt.Printf("  Type: %s\n", integration.Type)
				if integration.Description != "" {
					fmt.Printf("  Description: %s\n", integration.Description)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of integrations to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of integrations to skip")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphSubgraphCommand creates the subgraph command
func newGraphSubgraphCommand(cfg *config.Config) *cobra.Command {
	var (
		depth        int
		relTypes     []string
		integration  string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "subgraph <node-id>",
		Short: "Get a subgraph starting from a node",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get subgraph with depth 2
  kubiya graph subgraph node-123 --depth 2

  # Get subgraph with specific relationship types
  kubiya graph subgraph node-123 --depth 2 --rel-types MEMBER_OF,MANAGES`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &controlplane.SubgraphRequest{
				NodeID:            nodeID,
				Depth:             depth,
				RelationshipTypes: relTypes,
			}

			
			subgraph, err := client.GetSubgraph(req, integration)
			if err != nil {
				return fmt.Errorf("failed to get subgraph: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(subgraph)
			}

			// Pretty print subgraph
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render("Subgraph"))
			fmt.Println()
			fmt.Printf("  %s Nodes: %d\n", style.RobotIconStyle.Render("📊"), len(subgraph.Nodes))
			fmt.Printf("  %s Relationships: %d\n", style.RobotIconStyle.Render("🔗"), len(subgraph.Relationships))
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 2, "Traversal depth (1-5)")
	cmd.Flags().StringSliceVar(&relTypes, "rel-types", nil, "Relationship types to follow (comma-separated)")
	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// newGraphQueryCommand creates the query command
func newGraphQueryCommand(cfg *config.Config) *cobra.Command {
	var (
		query        string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Execute a custom Cypher query (read-only)",
		Long: `Execute a custom Cypher query against the context graph.

The query will be automatically scoped to your organization's data.
Only read-only queries are allowed.`,
		Example: `  # Execute a simple query
  kubiya graph query --query "MATCH (n:User) RETURN n LIMIT 10"

  # Get results in JSON
  kubiya graph query --query "MATCH (n:Team) RETURN n" --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &controlplane.CustomQueryRequest{
				Query: query,
			}

			
			results, err := client.ExecuteCustomQuery(req)
			if err != nil {
				return fmt.Errorf("failed to execute query: %w", err)
			}

			if outputFormat == "json" {
				return printGraphJSON(results)
			}

			// Pretty print results
			fmt.Println()
			fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Query Results (%d rows)", len(results))))
			fmt.Println()

			for i, row := range results {
				fmt.Printf("%s Row %d:\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), i+1)
				for key, value := range row {
					fmt.Printf("  %s: %v\n", key, value)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Cypher query to execute (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// printGraphJSON prints data as formatted JSON
func printGraphJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// newGraphSearchCommand creates the intelligent search command
func newGraphSearchCommand(cfg *config.Config) *cobra.Command {
	var (
		maxTurns             int
		model                string
		temperature          float64
		integration          string
		labelFilter          string
		enableSemanticSearch bool
		enableCypherQueries  bool
		strategy             string
		sessionID            string
		stream               bool
		outputFormat         string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Perform AI-powered intelligent graph search",
		Long: `AI-powered intelligent search using natural language to query the context graph.

Features:
  🤖 AI-powered: Claude-based agent with custom graph tools
  🎯 Flexible: Configurable model, temperature, and search parameters
  🔍 Smart tools: 10 graph operations (property search, relationships, subgraphs, etc.)
  📡 Streaming: Real-time progress updates (use --stream flag)

The agent has access to specialized tools:
  - search_nodes_by_property - Exact property matching
  - search_nodes_by_text_pattern - Regex pattern matching
  - get_node_by_id - Retrieve specific node
  - get_node_relationships - Explore connections
  - get_subgraph - Extract neighborhood
  - get_available_labels - Schema discovery
  - get_available_relationship_types - Relationship types
  - get_graph_statistics - High-level metrics
  - search_nodes_semantic - AI semantic search (conditional)
  - execute_read_only_cypher - Custom queries (conditional)`,
		Example: `  # Simple search
  kubiya graph search "Find all production environments in AWS"

  # Search with streaming
  kubiya graph search "Show me critical security issues" --stream

  # Advanced search with parameters
  kubiya graph search "Find Kubernetes clusters" \
    --max-turns 10 \
    --model kubiya/claude-opus-4 \
    --temperature 0.5 \
    --integration AWS

  # Continue a conversation
  kubiya graph search "Tell me more about the first one" --session abc-123-def

  # Search in JSON format
  kubiya graph search "List all teams" --output json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

			// Get control plane client to fetch config
			cpClient, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create control plane client: %w", err)
			}

			// Fetch client config to get context graph API URL
			config, err := cpClient.GetClientConfig()
			if err != nil {
				return fmt.Errorf("failed to get client config: %w", err)
			}

			req := &controlplane.IntelligentSearchRequest{
				Keywords:              query,
				MaxTurns:              maxTurns,
				Model:                 model,
				Temperature:           temperature,
				Integration:           integration,
				LabelFilter:           labelFilter,
				EnableSemanticSearch:  enableSemanticSearch,
				EnableCypherQueries:   enableCypherQueries,
				Strategy:              strategy,
				SessionID:             sessionID,
			}

			// Streaming search
			if stream {
				return performStreamingSearch(config.ContextGraphAPIBase, cfg.APIKey, req, outputFormat)
			}

			// Non-streaming search
			return performNonStreamingSearch(cpClient, req, outputFormat)
		},
	}

	cmd.Flags().IntVar(&maxTurns, "max-turns", 5, "Maximum agent conversation turns (1-20)")
	cmd.Flags().StringVar(&model, "model", "", "LiteLLM model name (default: kubiya/claude-sonnet-4)")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Model temperature (0.0-2.0)")
	cmd.Flags().StringVar(&integration, "integration", "", "Filter by integration (e.g., AWS, Azure, Slack)")
	cmd.Flags().StringVar(&labelFilter, "label", "", "Filter by node label")
	cmd.Flags().BoolVar(&enableSemanticSearch, "semantic", false, "Enable semantic search")
	cmd.Flags().BoolVar(&enableCypherQueries, "cypher", false, "Enable custom Cypher queries")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Agent strategy (claude_sdk or agno)")
	cmd.Flags().StringVar(&sessionID, "session", "", "Continue previous conversation session")
	cmd.Flags().BoolVar(&stream, "stream", true, "Enable streaming mode with real-time updates")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")

	return cmd
}

// performStreamingSearch executes streaming intelligent search with bubbletea TUI
func performStreamingSearch(graphAPIBase, apiKey string, req *controlplane.IntelligentSearchRequest, outputFormat string) error {
	// Create direct context graph client
	graphClient := contextgraph.NewClient(graphAPIBase, apiKey)

	// Convert request to contextgraph format
	graphReq := contextgraph.IntelligentSearchRequest{
		Keywords:    req.Keywords,
		MaxTurns:    req.MaxTurns,
		SessionID:   req.SessionID,
		Model:       req.Model,
		Temperature: req.Temperature,
	}

	// Start streaming search
	ctx := context.Background()
	eventChan, err := graphClient.StreamSearch(ctx, graphReq)
	if err != nil {
		return fmt.Errorf("failed to start streaming search: %w", err)
	}

	// Run bubbletea TUI for real-time progress display
	result, err := runGraphSearchTUI(ctx, req.Keywords, eventChan)
	if err != nil {
		return err
	}

	if result == nil {
		return fmt.Errorf("search completed but no result was returned")
	}

	// Handle JSON output
	if outputFormat == "json" {
		jsonResult := map[string]interface{}{
			"answer":      result.answer,
			"tool_calls":  result.toolCalls,
			"turns_used":  result.turnsUsed,
			"confidence":  result.confidence,
			"suggestions": result.suggestions,
			"session_id":  result.sessionID,
		}
		return printGraphJSON(jsonResult)
	}

	// Text output is already rendered by the TUI
	return nil
}

// performNonStreamingSearch executes non-streaming intelligent search
func performNonStreamingSearch(client *controlplane.Client, req *controlplane.IntelligentSearchRequest, outputFormat string) error {
	fmt.Println()
	fmt.Println(style.HeadingStyle.Render("🔍 Intelligent Search"))
	fmt.Println()
	fmt.Printf("  %s Query: %s\n", style.RobotIconStyle.Render("💬"), style.HighlightStyle.Render(req.Keywords))
	fmt.Printf("  %s Searching...\n", style.RobotIconStyle.Render("⚡"))
	fmt.Println()

	result, err := client.IntelligentSearch(req)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if outputFormat == "json" {
		return printGraphJSON(result)
	}

	// Pretty print results
	fmt.Println(style.HeadingStyle.Render("Answer"))
	fmt.Println()
	fmt.Println(style.DimStyle.Render(result.Answer))
	fmt.Println()

	if len(result.ToolCalls) > 0 {
		fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Tools Used (%d)", len(result.ToolCalls))))
		fmt.Println()
		for i, tc := range result.ToolCalls {
			fmt.Printf("  %s %s\n", style.DimStyle.Render(fmt.Sprintf("%d.", i+1)), style.HighlightStyle.Render(tc.ToolName))
			if tc.ResultSummary != "" {
				fmt.Printf("    %s\n", tc.ResultSummary)
			}
		}
		fmt.Println()
	}

	if len(result.Nodes) > 0 {
		fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Nodes Found (%d)", len(result.Nodes))))
		fmt.Println()
		for i, node := range result.Nodes {
			if i >= 5 {
				fmt.Printf("  %s ... and %d more\n", style.DimStyle.Render("•"), len(result.Nodes)-5)
				break
			}
			fmt.Printf("  %s %s (%s)\n", style.BulletStyle.Render("•"), style.HighlightStyle.Render(node.ID), strings.Join(node.Labels, ", "))
		}
		fmt.Println()
	}

	if len(result.Relationships) > 0 {
		fmt.Println(style.HeadingStyle.Render(fmt.Sprintf("Relationships Found (%d)", len(result.Relationships))))
		fmt.Println()
		for i, rel := range result.Relationships {
			if i >= 5 {
				fmt.Printf("  %s ... and %d more\n", style.DimStyle.Render("•"), len(result.Relationships)-5)
				break
			}
			fmt.Printf("  %s %s → %s (%s)\n", style.BulletStyle.Render("•"), rel.StartNode, rel.EndNode, rel.Type)
		}
		fmt.Println()
	}

	if result.Confidence != "" {
		confidenceIcon := "✓"
		if result.Confidence == "low" {
			confidenceIcon = "⚠"
		}
		fmt.Printf("  %s Confidence: %s\n", style.RobotIconStyle.Render(confidenceIcon), style.HighlightStyle.Render(result.Confidence))
		fmt.Printf("  %s Turns: %s\n", style.RobotIconStyle.Render("🔄"), style.HighlightStyle.Render(fmt.Sprintf("%d", result.TurnsUsed)))
		fmt.Println()
	}

	if len(result.Suggestions) > 0 {
		fmt.Println(style.HeadingStyle.Render("Follow-up Suggestions"))
		fmt.Println()
		for _, sug := range result.Suggestions {
			fmt.Printf("  %s %s\n", style.BulletStyle.Render("•"), sug)
		}
		fmt.Println()
	}

	return nil
}
