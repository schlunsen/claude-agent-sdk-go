package types

import (
	"context"
	"encoding/json"
)

// ToolHandler defines the function signature for MCP tool handlers.
// It receives the context and tool arguments, and returns the tool result or an error.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// Tool represents an MCP tool that can be called by Claude.
// This is used with the MCP server factory to simplify tool registration.
type Tool struct {
	// Name is the unique identifier for this tool (e.g., "add", "greet", "calculator").
	// Must not contain spaces or special characters.
	Name string

	// Description is a human-readable description of what this tool does.
	// This is shown to Claude when deciding whether to use the tool.
	Description string

	// InputSchema is an optional JSON Schema (as a map) describing the tool's input parameters.
	// If nil, no input validation is performed.
	//
	// Example:
	//  map[string]interface{}{
	//    "type": "object",
	//    "properties": map[string]interface{}{
	//      "name": map[string]interface{}{"type": "string"},
	//      "age": map[string]interface{}{"type": "number"},
	//    },
	//    "required": []string{"name"},
	//  }
	InputSchema map[string]interface{}

	// Handler is the function that executes when Claude calls this tool.
	// It receives the tool arguments and should return either:
	// - A map[string]interface{} with the result
	// - A string with the result
	// - An error if tool execution fails
	//
	// The handler should be designed to be idempotent and handle concurrent calls.
	Handler ToolHandler
}

// Validate checks if the Tool configuration is valid.
// Returns an error if the tool is misconfigured.
func (t *Tool) Validate() error {
	if t.Name == "" {
		return NewValidationError("tool name is required")
	}
	if t.Handler == nil {
		return NewValidationError("tool handler is required")
	}
	return nil
}

// SDKMCPServer is a simple MCP server implementation created by the factory function.
// It handles JSON-RPC 2.0 message routing for list_tools and call_tool methods.
type SDKMCPServer struct {
	name    string
	version string
	tools   map[string]*Tool
}

// Name returns the server name.
func (s *SDKMCPServer) Name() string {
	return s.name
}

// Version returns the server version.
func (s *SDKMCPServer) Version() string {
	return s.version
}

// HandleMessage handles incoming JSON-RPC 2.0 messages.
// It routes messages to the appropriate handler (list_tools or call_tool).
func (s *SDKMCPServer) HandleMessage(message map[string]interface{}) (map[string]interface{}, error) {
	// Extract the method field
	method, ok := message["method"].(string)
	if !ok {
		return s.errorResponse(message, -32600, "Invalid Request: missing or non-string method"), nil
	}

	// Route to appropriate handler
	switch method {
	case "initialize":
		return s.handleInitialize(message)
	case "notifications/initialized":
		// Acknowledge initialized notification
		return map[string]interface{}{"jsonrpc": "2.0", "result": map[string]interface{}{}}, nil
	case "tools/list":
		return s.handleListTools(message)
	case "tools/call":
		return s.handleCallTool(message)
	default:
		return s.errorResponse(message, -32601, "Method not found: "+method), nil
	}
}

// handleInitialize handles MCP initialization request.
func (s *SDKMCPServer) handleInitialize(message map[string]interface{}) (map[string]interface{}, error) {
	id := message["id"]
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    s.name,
				"version": s.version,
			},
		},
	}, nil
}

// handleListTools returns the list of available tools.
func (s *SDKMCPServer) handleListTools(message map[string]interface{}) (map[string]interface{}, error) {
	tools := make([]map[string]interface{}, 0, len(s.tools))

	for _, tool := range s.tools {
		toolMap := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}

		if tool.InputSchema != nil {
			toolMap["inputSchema"] = tool.InputSchema
		}

		tools = append(tools, toolMap)
	}

	id := message["id"]
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"tools": tools,
		},
	}, nil
}

// handleCallTool executes a tool and returns its result.
func (s *SDKMCPServer) handleCallTool(message map[string]interface{}) (map[string]interface{}, error) {
	// Extract tool name and arguments
	params, ok := message["params"].(map[string]interface{})
	if !ok {
		return s.errorResponse(message, -32602, "Invalid params: expected object"), nil
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.errorResponse(message, -32602, "Invalid params: missing tool name"), nil
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		args = make(map[string]interface{})
	}

	// Find the tool
	tool, exists := s.tools[toolName]
	if !exists {
		return s.errorResponse(message, -32603, "Tool not found: "+toolName), nil
	}

	// Call the tool handler
	ctx := context.Background()
	result, err := tool.Handler(ctx, args)
	if err != nil {
		return s.errorResponse(message, -32603, "Tool execution failed: "+err.Error()), nil
	}

	// Format the result as content blocks
	id := message["id"]
	content := s.formatResult(result)

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"content": content,
		},
	}, nil
}

// formatResult converts a tool result into MCP content blocks.
func (s *SDKMCPServer) formatResult(result any) []map[string]interface{} {
	contentBlocks := make([]map[string]interface{}, 0)

	switch v := result.(type) {
	case string:
		// String result → text content block
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "text",
			"text": v,
		})
	case map[string]interface{}:
		// Map result → try to format as text or JSON
		if text, ok := v["text"].(string); ok {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		} else {
			// Return as JSON text
			text := formatMapAsJSON(v)
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		}
	case []interface{}:
		// Array result → each item becomes a content block
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				contentBlocks = append(contentBlocks, m)
			}
		}
	default:
		// Unknown type → convert to string
		text := formatAsJSON(v)
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "text",
			"text": text,
		})
	}

	return contentBlocks
}

// errorResponse creates a JSON-RPC error response.
func (s *SDKMCPServer) errorResponse(message map[string]interface{}, code int, errMsg string) map[string]interface{} {
	id := message["id"]
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": errMsg,
		},
	}
}

// formatAsJSON formats a value as JSON string.
func formatAsJSON(v interface{}) string {
	data, _ := marshalToJSON(v)
	return string(data)
}

// formatMapAsJSON formats a map as JSON string.
func formatMapAsJSON(m map[string]interface{}) string {
	data, _ := marshalToJSON(m)
	return string(data)
}

// marshalToJSON is a simple JSON marshaling helper.
func marshalToJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// NewSDKMCPServer creates a new MCP server with the given name and tools.
//
// This factory function simplifies MCP server creation by handling JSON-RPC
// message routing and tool management automatically. Users only need to
// provide tool definitions and handlers.
//
// Example:
//
//	server, err := types.NewSDKMCPServer("calculator",
//	  types.Tool{
//	    Name: "add",
//	    Description: "Add two numbers",
//	    InputSchema: map[string]interface{}{
//	      "type": "object",
//	      "properties": map[string]interface{}{
//	        "a": map[string]interface{}{"type": "number"},
//	        "b": map[string]interface{}{"type": "number"},
//	      },
//	    },
//	    Handler: func(ctx context.Context, args map[string]any) (any, error) {
//	      a, _ := args["a"].(float64)
//	      b, _ := args["b"].(float64)
//	      return map[string]any{"result": a + b}, nil
//	    },
//	  },
//	)
//	if err != nil {
//	  panic(err)
//	}
//
//	opts := types.NewClaudeAgentOptions().
//	  WithMCPServer("calculator", server)
func NewSDKMCPServer(name string, tools ...Tool) (*SDKMCPServer, error) {
	if name == "" {
		return nil, NewValidationError("server name is required")
	}

	if len(tools) == 0 {
		return nil, NewValidationError("at least one tool is required")
	}

	// Build tool map and validate
	toolMap := make(map[string]*Tool)
	for i := range tools {
		tool := &tools[i]

		// Validate tool
		if err := tool.Validate(); err != nil {
			return nil, err
		}

		// Check for duplicate tool names
		if _, exists := toolMap[tool.Name]; exists {
			return nil, NewValidationError("duplicate tool name: " + tool.Name)
		}

		toolMap[tool.Name] = tool
	}

	return &SDKMCPServer{
		name:    name,
		version: "1.0.0", // Default version
		tools:   toolMap,
	}, nil
}
