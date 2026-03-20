// Package llmstxt generates llms.txt content describing a running Tentaserve instance.
// The generated text helps AI agents understand what endpoints, tools, and
// capabilities are available on this instance.
package llmstxt

import (
	"fmt"
	"strings"
	"time"
)

// Upstream describes an upstream for llms.txt generation.
type Upstream struct {
	Name     string
	Type     string // "rest" or "graphql"
	BaseURL  string
	Endpoint string
}

// Tool describes an MCP tool for llms.txt generation.
type Tool struct {
	Name        string
	Description string
	Upstream    string
	InputSchema string
}

// Config holds the information needed to generate llms.txt.
type Config struct {
	Version     string
	Host        string
	Port        int
	GraphQLPath string
	RESTPrefix  string
	MCPPath     string
	Upstreams   []Upstream
	Tools       []Tool
}

// Generate creates the llms.txt content from the given configuration.
func Generate(cfg Config) string {
	var b strings.Builder

	// Header
	b.WriteString("# Tentaserve API Gateway\n\n")
	b.WriteString(fmt.Sprintf("> Version: %s\n", cfg.Version))
	b.WriteString(fmt.Sprintf("> Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	// Overview
	b.WriteString("## Overview\n\n")
	b.WriteString("Tentaserve is a self-hosted API gateway that provides bi-directional\n")
	b.WriteString("protocol translation between REST and GraphQL, with built-in MCP\n")
	b.WriteString("(Model Context Protocol) server integration for AI agents.\n\n")

	// Base URL
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	if cfg.Host == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}

	// Endpoints
	b.WriteString("## Endpoints\n\n")

	if cfg.GraphQLPath != "" {
		b.WriteString(fmt.Sprintf("- GraphQL: `POST %s%s`\n", baseURL, cfg.GraphQLPath))
	}
	if cfg.RESTPrefix != "" {
		b.WriteString(fmt.Sprintf("- REST API: `%s%s/*`\n", baseURL, cfg.RESTPrefix))
	}
	if cfg.MCPPath != "" {
		b.WriteString(fmt.Sprintf("- MCP (Model Context Protocol): `POST %s%s`\n", baseURL, cfg.MCPPath))
	}
	b.WriteString(fmt.Sprintf("- Health Check: `GET %s/-/health`\n", baseURL))
	b.WriteString(fmt.Sprintf("- Metrics: `GET %s/-/metrics`\n", baseURL))
	b.WriteString(fmt.Sprintf("- Admin: `GET %s/-/admin`\n\n", baseURL))

	// Upstreams
	if len(cfg.Upstreams) > 0 {
		b.WriteString("## Upstreams\n\n")
		for _, u := range cfg.Upstreams {
			url := u.BaseURL
			if url == "" {
				url = u.Endpoint
			}
			b.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", u.Name, u.Type, url))
		}
		b.WriteString("\n")
	}

	// MCP Tools
	if len(cfg.Tools) > 0 {
		b.WriteString("## MCP Tools\n\n")
		b.WriteString("The following tools are available via the MCP endpoint:\n\n")
		for _, t := range cfg.Tools {
			b.WriteString(fmt.Sprintf("### %s\n", t.Name))
			if t.Description != "" {
				b.WriteString(fmt.Sprintf("%s\n", t.Description))
			}
			b.WriteString(fmt.Sprintf("- Upstream: %s\n", t.Upstream))
			if t.InputSchema != "" {
				b.WriteString(fmt.Sprintf("- Input Schema: `%s`\n", t.InputSchema))
			}
			b.WriteString("\n")
		}
	}

	// Usage
	b.WriteString("## Usage\n\n")

	if cfg.GraphQLPath != "" {
		b.WriteString("### GraphQL\n\n")
		b.WriteString("```bash\n")
		b.WriteString(fmt.Sprintf("curl -X POST %s%s \\\n", baseURL, cfg.GraphQLPath))
		b.WriteString("  -H 'Content-Type: application/json' \\\n")
		b.WriteString("  -d '{\"query\": \"{ __typename }\"}'\n")
		b.WriteString("```\n\n")
	}

	if cfg.MCPPath != "" {
		b.WriteString("### MCP\n\n")
		b.WriteString("```bash\n")
		b.WriteString(fmt.Sprintf("curl -X POST %s%s \\\n", baseURL, cfg.MCPPath))
		b.WriteString("  -H 'Content-Type: application/json' \\\n")
		b.WriteString("  -d '{\"jsonrpc\":\"2.0\",\"method\":\"tools/list\",\"id\":1}'\n")
		b.WriteString("```\n\n")
	}

	return b.String()
}

// GenerateHandler returns an http.HandlerFunc that serves the generated llms.txt.
func GenerateHandler(cfg Config) func(w interface{ Write([]byte) (int, error) }) {
	content := Generate(cfg)
	return func(w interface{ Write([]byte) (int, error) }) {
		w.Write([]byte(content))
	}
}
