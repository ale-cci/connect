package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMcpServerInitialization(t *testing.T) {
	// Create a test server to verify our registration and helper structure works
	s := server.NewMCPServer("test-mcp-server", "1.0.0")

	// Verify we can define list_tables tool
	listTablesTool := mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in the connected MySQL database"),
	)
	if listTablesTool.Name != "list_tables" {
		t.Errorf("expected tool name to be list_tables, got %s", listTablesTool.Name)
	}

	// Verify we can define describe_table tool with string property
	describeTableTool := mcp.NewTool("describe_table",
		mcp.WithDescription("Get column definitions, types, keys, and default values for a specific table"),
		mcp.WithString("table_name",
			mcp.Required(),
			mcp.Description("The name of the table to describe"),
		),
	)
	if describeTableTool.Name != "describe_table" {
		t.Errorf("expected tool name to be describe_table, got %s", describeTableTool.Name)
	}

	// Verify we can define execute_query tool with string property
	executeQueryTool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an arbitrary raw SQL query against the database"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The SQL query to execute"),
		),
	)
	if executeQueryTool.Name != "execute_query" {
		t.Errorf("expected tool name to be execute_query, got %s", executeQueryTool.Name)
	}

	// Test registering them on the server
	s.AddTool(listTablesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("[]"), nil
	})
	s.AddTool(describeTableTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("[]"), nil
	})
	s.AddTool(executeQueryTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("[]"), nil
	})
}
