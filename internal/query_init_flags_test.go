package internal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// initializeRequestFor runs Initialize against a mock transport and returns the
// "request" body of the initialize control request that was written.
func initializeRequestFor(t *testing.T, opts *types.ClaudeAgentOptions) map[string]interface{} {
	t.Helper()
	ctx := context.Background()
	transport := newMockTransport()
	query := NewQuery(ctx, transport, opts, log.NewLogger(false), true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	captured := make(chan map[string]interface{}, 1)
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			for _, data := range transport.getWrittenData() {
				var sent map[string]interface{}
				if err := json.Unmarshal([]byte(data), &sent); err != nil {
					continue
				}
				request, _ := sent["request"].(map[string]interface{})
				if sent["type"] != "control_request" || request["subtype"] != "initialize" {
					continue
				}
				captured <- request
				transport.sendMessage(&types.SystemMessage{
					Type:    "control_response",
					Subtype: "control_response",
					Response: map[string]interface{}{
						"subtype":    "success",
						"request_id": sent["request_id"],
						"response":   map[string]interface{}{},
					},
				})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	if _, err := query.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	select {
	case request := <-captured:
		return request
	case <-time.After(2 * time.Second):
		t.Fatal("initialize request was not written")
		return nil
	}
}

func TestInitializeSendsExcludeDynamicSections(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name         string
		systemPrompt interface{}
		want         interface{} // nil means the key must be absent
	}{
		{"preset true", types.SystemPromptPreset{Type: "preset", Preset: "claude_code", ExcludeDynamicSections: &yes}, true},
		{"preset false", types.SystemPromptPreset{Type: "preset", Preset: "claude_code", ExcludeDynamicSections: &no}, false},
		{"preset pointer", &types.SystemPromptPreset{Type: "preset", Preset: "claude_code", ExcludeDynamicSections: &yes}, true},
		{"preset unset", types.SystemPromptPreset{Type: "preset", Preset: "claude_code"}, nil},
		{"string prompt", "be brief", nil},
		{"no prompt", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := types.NewClaudeAgentOptions()
			opts.SystemPrompt = tt.systemPrompt

			request := initializeRequestFor(t, opts)
			got, present := request["excludeDynamicSections"]
			if tt.want == nil {
				if present {
					t.Fatalf("expected excludeDynamicSections to be absent, got %v", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("excludeDynamicSections = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStampUserMessage(t *testing.T) {
	newMsg := func() map[string]interface{} {
		return map[string]interface{}{
			"type":    "user",
			"message": map[string]interface{}{"role": "user", "content": "@secrets.txt /exit"},
		}
	}

	off := StampUserMessage(newMsg(), false)
	if _, ok := off["client_composed"]; ok {
		t.Error("client_composed should not be set when verbatim is off")
	}

	on := StampUserMessage(newMsg(), true)
	if on["client_composed"] != true {
		t.Errorf("client_composed = %v, want true", on["client_composed"])
	}

	// A caller-supplied value is overwritten when verbatim is on.
	msg := newMsg()
	msg["client_composed"] = false
	if StampUserMessage(msg, true)["client_composed"] != true {
		t.Error("client_composed should be forced to true when verbatim is on")
	}
}
