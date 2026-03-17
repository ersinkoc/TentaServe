package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/mcp"
	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

func toolsCmd(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("tools", flag.ExitOnError)
	configPath := fs.String("config", "tentaserve.yaml", "Path to configuration file")
	format := fs.String("format", "table", "Output format: table, json, names")
	upstreamFilter := fs.String("upstream", "", "Filter to specific upstream (default: all)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create tool registry
	registry := mcp.NewToolRegistry(slog.Default())

	// Build tools from each upstream
	for _, upstream := range cfg.Upstreams {
		// Apply upstream filter if specified
		if *upstreamFilter != "" && upstream.Name != *upstreamFilter {
			continue
		}

		// Build tools based on upstream type
		if err := buildToolsFromUpstream(registry, upstream); err != nil {
			return fmt.Errorf("building tools for %s: %w", upstream.Name, err)
		}
	}

	// Output based on format
	tools := registry.List()

	switch *format {
	case "table":
		printToolsTable(tools)
	case "json":
		if err := printToolsJSON(tools); err != nil {
			return fmt.Errorf("outputting JSON: %w", err)
		}
	case "names":
		printToolsNames(tools)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, names)", *format)
	}

	return nil
}

// buildToolsFromUpstream builds tools from an upstream configuration.
func buildToolsFromUpstream(registry *mcp.ToolRegistry, upstream config.UpstreamConfig) error {
	switch upstream.Type {
	case "rest":
		// Build from OpenAPI spec if available
		if upstream.OpenAPI != nil && upstream.OpenAPI.Source != "" {
			spec, err := openapi.LoadOpenAPISpec(upstream.OpenAPI.Source)
			if err != nil {
				return fmt.Errorf("loading OpenAPI spec: %w", err)
			}
			if err := registry.BuildFromOpenAPI(spec, upstream.Name); err != nil {
				return fmt.Errorf("building from OpenAPI: %w", err)
			}
		}
	case "graphql":
		// Build from GraphQL introspection
		// For now, create a simple schema and build from it
		s := schema.NewSchemaDefinition()
		s.Query = &schema.OperationDef{
			Name: "Query",
			Type: "query",
			Fields: []*schema.FieldDef{
				{
					Name:        "hello",
					Description: "Hello world query",
					Type:        schema.StringType(),
				},
			},
		}
		if err := registry.BuildFromSchema(s, upstream.Name); err != nil {
			return fmt.Errorf("building from schema: %w", err)
		}
	}

	return nil
}

// printToolsTable prints tools in table format.
func printToolsTable(tools []*mcp.Tool) {
	if len(tools) == 0 {
		fmt.Println("No tools registered")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tUPSTREAM\tOPERATION")
	fmt.Fprintln(w, "----\t-----------\t--------\t---------")

	for _, tool := range tools {
		desc := tool.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tool.Name, desc, tool.Upstream, tool.Operation)
	}

	w.Flush()
	fmt.Printf("\nTotal: %d tools\n", len(tools))
}

// printToolsJSON prints tools in JSON format.
func printToolsJSON(tools []*mcp.Tool) error {
	// Convert to JSON-friendly format
	output := make([]map[string]any, len(tools))
	for i, tool := range tools {
		var inputSchema map[string]any
		if tool.InputSchema != nil {
			json.Unmarshal(tool.InputSchema, &inputSchema)
		}

		output[i] = map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": inputSchema,
			"upstream":    tool.Upstream,
			"operation":   tool.Operation,
			"method":      tool.Method,
			"path":        tool.Path,
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// printToolsNames prints just the tool names.
func printToolsNames(tools []*mcp.Tool) {
	for _, tool := range tools {
		fmt.Println(tool.Name)
	}
	fmt.Printf("\nTotal: %d tools\n", len(tools))
}
