package types

import (
	"testing"
)

// TestUnmarshalSystemTaskStarted verifies that system/task_started messages
// (the format the CLI actually emits) are routed to TaskStartedMessage.
func TestUnmarshalSystemTaskStarted(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"task_started","task_id":"task-1","session_id":"sess-1"}`)
	msg, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage() failed: %v", err)
	}
	started, ok := msg.(*TaskStartedMessage)
	if !ok {
		t.Fatalf("expected *TaskStartedMessage, got %T", msg)
	}
	if started.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", started.TaskID, "task-1")
	}
	if started.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", started.SessionID, "sess-1")
	}
}

// TestUnmarshalSystemTaskProgress verifies routing of system/task_progress.
func TestUnmarshalSystemTaskProgress(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"task_progress","task_id":"task-1","session_id":"sess-1","data":{"step":1}}`)
	msg, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage() failed: %v", err)
	}
	if _, ok := msg.(*TaskProgressMessage); !ok {
		t.Fatalf("expected *TaskProgressMessage, got %T", msg)
	}
}

// TestUnmarshalSystemTaskNotification verifies routing of system/task_notification.
func TestUnmarshalSystemTaskNotification(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"task_notification","task_id":"task-1","status":"completed","session_id":"sess-1"}`)
	msg, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage() failed: %v", err)
	}
	notif, ok := msg.(*TaskNotificationMessage)
	if !ok {
		t.Fatalf("expected *TaskNotificationMessage, got %T", msg)
	}
	if notif.Status != TaskNotificationStatusCompleted {
		t.Errorf("Status = %q, want %q", notif.Status, TaskNotificationStatusCompleted)
	}
}

// TestUnmarshalSystemTaskUpdated verifies that system/task_updated messages are
// exposed as typed TaskUpdatedMessage with Status derived from the patch.
func TestUnmarshalSystemTaskUpdated(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		wantTaskID   string
		wantStatus   string
		wantTerminal bool
	}{
		{
			name:         "terminal killed status",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-1","session_id":"sess-1","uuid":"u-1","patch":{"status":"killed","end_time":123}}`,
			wantTaskID:   "task-1",
			wantStatus:   TaskUpdatedStatusKilled,
			wantTerminal: true,
		},
		{
			name:         "terminal completed status",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-2","patch":{"status":"completed"}}`,
			wantTaskID:   "task-2",
			wantStatus:   TaskUpdatedStatusCompleted,
			wantTerminal: true,
		},
		{
			name:         "non-terminal running status",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-3","patch":{"status":"running"}}`,
			wantTaskID:   "task-3",
			wantStatus:   TaskUpdatedStatusRunning,
			wantTerminal: false,
		},
		{
			name:         "patch without status is non-terminal",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-4","patch":{"end_time":456}}`,
			wantTaskID:   "task-4",
			wantStatus:   "",
			wantTerminal: false,
		},
		{
			name:         "missing patch does not fail",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-5"}`,
			wantTaskID:   "task-5",
			wantStatus:   "",
			wantTerminal: false,
		},
		{
			name:         "non-string patch status is ignored",
			json:         `{"type":"system","subtype":"task_updated","task_id":"task-6","patch":{"status":42}}`,
			wantTaskID:   "task-6",
			wantStatus:   "",
			wantTerminal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := UnmarshalMessage([]byte(tt.json))
			if err != nil {
				t.Fatalf("UnmarshalMessage() failed: %v", err)
			}
			updated, ok := msg.(*TaskUpdatedMessage)
			if !ok {
				t.Fatalf("expected *TaskUpdatedMessage, got %T", msg)
			}
			if updated.TaskID != tt.wantTaskID {
				t.Errorf("TaskID = %q, want %q", updated.TaskID, tt.wantTaskID)
			}
			if updated.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", updated.Status, tt.wantStatus)
			}
			if updated.IsTerminal() != tt.wantTerminal {
				t.Errorf("IsTerminal() = %v, want %v", updated.IsTerminal(), tt.wantTerminal)
			}
		})
	}
}

// TestUnmarshalTopLevelTaskUpdated verifies the top-level task_updated type is
// also routed (forward/backward compatibility with the system-wrapped form).
func TestUnmarshalTopLevelTaskUpdated(t *testing.T) {
	data := []byte(`{"type":"task_updated","subtype":"task_updated","task_id":"task-1","patch":{"status":"failed"}}`)
	msg, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage() failed: %v", err)
	}
	updated, ok := msg.(*TaskUpdatedMessage)
	if !ok {
		t.Fatalf("expected *TaskUpdatedMessage, got %T", msg)
	}
	if updated.Status != TaskUpdatedStatusFailed {
		t.Errorf("Status = %q, want %q", updated.Status, TaskUpdatedStatusFailed)
	}
	if !updated.IsTerminal() {
		t.Error("IsTerminal() = false, want true")
	}
}

// TestUnmarshalSystemNonTaskSubtypeUnaffected verifies that other system
// subtypes still parse as SystemMessage.
func TestUnmarshalSystemNonTaskSubtypeUnaffected(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"init","data":{"session_id":"sess-1"}}`)
	msg, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage() failed: %v", err)
	}
	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}
	if !sys.IsInit() {
		t.Error("IsInit() = false, want true")
	}
}

// TestIsTerminalTaskStatus covers both lifecycle vocabularies.
func TestIsTerminalTaskStatus(t *testing.T) {
	terminal := []string{"completed", "failed", "stopped", "killed"}
	for _, s := range terminal {
		if !IsTerminalTaskStatus(s) {
			t.Errorf("IsTerminalTaskStatus(%q) = false, want true", s)
		}
	}
	nonTerminal := []string{"pending", "running", "paused", "", "unknown"}
	for _, s := range nonTerminal {
		if IsTerminalTaskStatus(s) {
			t.Errorf("IsTerminalTaskStatus(%q) = true, want false", s)
		}
	}
}
