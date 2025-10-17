package types

import (
	"encoding/json"
	"testing"
)

// TestPermissionModeConstants tests that permission mode constants are defined correctly.
func TestPermissionModeConstants(t *testing.T) {
	modes := []PermissionMode{
		PermissionModeDefault,
		PermissionModeAcceptEdits,
		PermissionModePlan,
		PermissionModeBypassPermissions,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Error("permission mode should not be empty")
		}
	}
}

// TestPermissionUpdateMarshaling tests JSON marshaling of PermissionUpdate.
func TestPermissionUpdateMarshaling(t *testing.T) {
	behavior := PermissionBehaviorAllow
	update := &PermissionUpdate{
		Type: "addRules",
		Rules: []PermissionRuleValue{
			{
				ToolName:    "Bash",
				RuleContent: stringPtr("allow ls command"),
			},
		},
		Behavior: &behavior,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("failed to marshal PermissionUpdate: %v", err)
	}

	var decoded PermissionUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal PermissionUpdate: %v", err)
	}

	if decoded.Type != update.Type {
		t.Errorf("type doesn't match")
	}
	if len(decoded.Rules) != len(update.Rules) {
		t.Errorf("rules length doesn't match")
	}
}

// TestSDKControlPermissionRequest tests JSON marshaling of SDKControlPermissionRequest.
func TestSDKControlPermissionRequest(t *testing.T) {
	req := &SDKControlPermissionRequest{
		Subtype:  "can_use_tool",
		ToolName: "Bash",
		Input: map[string]interface{}{
			"command": "ls -la",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SDKControlPermissionRequest: %v", err)
	}

	var decoded SDKControlPermissionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SDKControlPermissionRequest: %v", err)
	}

	if decoded.ToolName != req.ToolName {
		t.Errorf("tool name doesn't match")
	}
}

// TestHookEventConstants tests that hook event constants are defined correctly.
func TestHookEventConstants(t *testing.T) {
	events := []HookEvent{
		HookEventPreToolUse,
		HookEventPostToolUse,
		HookEventUserPromptSubmit,
		HookEventStop,
		HookEventSubagentStop,
		HookEventPreCompact,
	}

	for _, event := range events {
		if event == "" {
			t.Error("hook event should not be empty")
		}
	}
}

// TestPreToolUseHookInput tests JSON marshaling of PreToolUseHookInput.
func TestPreToolUseHookInput(t *testing.T) {
	input := &PreToolUseHookInput{
		BaseHookInput: BaseHookInput{
			SessionID:      "session-123",
			TranscriptPath: "/path/to/transcript",
			CWD:            "/home/user",
		},
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput: map[string]interface{}{
			"command": "echo hello",
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal PreToolUseHookInput: %v", err)
	}

	var decoded PreToolUseHookInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal PreToolUseHookInput: %v", err)
	}

	if decoded.ToolName != input.ToolName {
		t.Errorf("tool name doesn't match")
	}
}

// Helper function to create a string pointer.
func stringPtr(s string) *string {
	return &s
}
