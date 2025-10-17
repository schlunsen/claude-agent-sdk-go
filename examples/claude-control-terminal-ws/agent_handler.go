package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
	"golang.org/x/net/websocket"
)

// QueryRequest represents a query message from the WebSocket client
type QueryRequest struct {
	Prompt string `json:"prompt"`
}

// ResponseMessage represents a response message sent to the WebSocket client
type ResponseMessage struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
	Error   string      `json:"error,omitempty"`
}

// AgentHandler manages WebSocket connections and Claude Agent SDK integration
type AgentHandler struct {
	config *Config
	mu     sync.Mutex
	active int
}

// NewAgentHandler creates a new agent handler with the given config
func NewAgentHandler(config *Config) *AgentHandler {
	return &AgentHandler{
		config: config,
		active: 0,
	}
}

// HandleWebSocket handles WebSocket connections for Claude queries
func (h *AgentHandler) HandleWebSocket(ws *websocket.Conn) {
	defer ws.Close()

	// Check concurrent session limit
	h.mu.Lock()
	if h.active >= h.config.MaxConcurrentSessions {
		h.mu.Unlock()
		h.sendError(ws, "max concurrent sessions reached")
		return
	}
	h.active++
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.active--
		h.mu.Unlock()
	}()

	log.Printf("WebSocket connection established from %s", ws.Request().RemoteAddr)

	// Main message loop
	for {
		var req QueryRequest
		if err := websocket.JSON.Receive(ws, &req); err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error receiving message: %v", err)
			}
			return
		}

		if req.Prompt == "" {
			h.sendError(ws, "prompt cannot be empty")
			continue
		}

		log.Printf("Received query: %s", req.Prompt)

		// Process the query
		if err := h.processQuery(ws, req.Prompt); err != nil {
			log.Printf("Error processing query: %v", err)
			h.sendError(ws, fmt.Sprintf("query failed: %v", err))
		}
	}
}

// processQuery executes a Claude query and streams responses back
func (h *AgentHandler) processQuery(ws *websocket.Conn, prompt string) error {
	ctx := context.Background()

	// Create SDK options
	opts := types.NewClaudeAgentOptions().
		WithModel(h.config.Model).
		WithPermissionMode(types.PermissionModeBypassPermissions)

	// Execute query
	messages, err := claude.Query(ctx, prompt, opts)
	if err != nil {
		return fmt.Errorf("failed to create query: %w", err)
	}

	// Stream responses back to client
	for msg := range messages {
		if err := h.sendMessage(ws, msg); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	return nil
}

// sendMessage sends a Claude message to the WebSocket client
func (h *AgentHandler) sendMessage(ws *websocket.Conn, msg types.Message) error {
	msgType := msg.GetMessageType()

	var resp ResponseMessage
	resp.Type = msgType

	switch msgType {
	case "assistant":
		if assistantMsg, ok := msg.(*types.AssistantMessage); ok {
			// Extract text content
			var textContent []string
			for _, block := range assistantMsg.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					textContent = append(textContent, textBlock.Text)
				}
			}
			resp.Content = map[string]interface{}{
				"text": textContent,
			}
		}

	case "user":
		if userMsg, ok := msg.(*types.UserMessage); ok {
			resp.Content = map[string]interface{}{
				"content": userMsg.Content,
			}
		}

	case "result":
		if resultMsg, ok := msg.(*types.ResultMessage); ok {
			content := map[string]interface{}{
				"success":      true,
				"num_turns":    resultMsg.NumTurns,
				"duration_ms":  resultMsg.DurationMs,
				"is_error":     resultMsg.IsError,
			}
			if resultMsg.TotalCostUSD != nil {
				content["cost_usd"] = *resultMsg.TotalCostUSD
			}
			if resultMsg.Usage != nil {
				content["usage"] = resultMsg.Usage
			}
			resp.Content = content
		}

	case "system":
		if systemMsg, ok := msg.(*types.SystemMessage); ok {
			resp.Content = map[string]interface{}{
				"subtype": systemMsg.Subtype,
				"data":    systemMsg.Data,
			}
		}

	default:
		resp.Content = map[string]interface{}{
			"raw": msg,
		}
	}

	return websocket.JSON.Send(ws, resp)
}

// sendError sends an error message to the WebSocket client
func (h *AgentHandler) sendError(ws *websocket.Conn, errMsg string) {
	resp := ResponseMessage{
		Type:  "error",
		Error: errMsg,
	}
	if err := websocket.JSON.Send(ws, resp); err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

// GetStats returns current handler statistics
func (h *AgentHandler) GetStats() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	return map[string]interface{}{
		"active_sessions": h.active,
		"max_sessions":    h.config.MaxConcurrentSessions,
	}
}

// HealthCheck endpoint handler
func (h *AgentHandler) HealthCheck(ws *websocket.Conn) {
	defer ws.Close()

	stats := h.GetStats()
	statsJSON, _ := json.Marshal(stats)

	websocket.Message.Send(ws, string(statsJSON))
}
