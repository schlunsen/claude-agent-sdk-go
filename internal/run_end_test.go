package internal

import (
	"context"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

func sessionState(state string, hostOnly bool) *types.SystemMessage {
	data := map[string]interface{}{"state": state}
	if hostOnly {
		data["sdk_host_only"] = true
	}
	return &types.SystemMessage{Type: "system", Subtype: "session_state_changed", Data: data}
}

func resultMsg() *types.ResultMessage {
	return &types.ResultMessage{Type: "result", Subtype: "success"}
}

func isEnded(r *runTracker) bool {
	select {
	case <-r.endedChan():
		return true
	default:
		return false
	}
}

// startRunQuery starts a Query whose CanUseTool makes the run bidirectional,
// with ceiling as CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS.
func startRunQuery(t *testing.T, ceiling string) (*Query, *mockTransport) {
	t.Helper()
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()
	opts.CanUseTool = func(context.Context, string, map[string]interface{}, types.ToolPermissionContext) (interface{}, error) {
		return nil, nil
	}
	opts.Env = map[string]string{runEndCeilingEnv: ceiling}
	q := NewQuery(ctx, transport, opts, log.NewLogger(false), false)
	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = q.Stop(stopCtx)
	})
	return q, transport
}

// waitInputEnded runs WaitForRunEndAndEndInput and reports whether it closed
// stdin within d.
func waitInputEnded(q *Query, transport *mockTransport, d time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_ = q.WaitForRunEndAndEndInput(ctx)
	return transport.isInputEnded()
}

func TestRunEndCeiling(t *testing.T) {
	t.Setenv(runEndCeilingEnv, "1500")
	tests := []struct {
		name string
		env  map[string]string
		want time.Duration
	}{
		{"inherited", nil, 1500 * time.Millisecond},
		{"options override", map[string]string{runEndCeilingEnv: "250"}, 250 * time.Millisecond},
		{"zero is no limit", map[string]string{runEndCeilingEnv: "0"}, 0},
		{"negative", map[string]string{runEndCeilingEnv: "-5"}, DefaultRunEndCeiling},
		{"not an integer", map[string]string{runEndCeilingEnv: "1e6"}, DefaultRunEndCeiling},
		{"capped", map[string]string{runEndCeilingEnv: "99999999999"}, maxRunEndCeilingMS * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RunEndCeiling(tt.env); got != tt.want {
				t.Errorf("RunEndCeiling() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunEndCeilingDefault(t *testing.T) {
	t.Setenv(runEndCeilingEnv, "")
	if got := RunEndCeiling(map[string]string{}); got != DefaultRunEndCeiling {
		t.Errorf("RunEndCeiling(empty value) = %v, want default", got)
	}
}

// The frames the SDK asked for are hidden; frames the caller opted into
// (no sdk_host_only) still reach them.
func TestSessionStateHostOnlyFramesHidden(t *testing.T) {
	r := newRunTracker(false, time.Minute)
	if !r.observe(sessionState("running", true)) {
		t.Error("sdk_host_only frame was not hidden")
	}
	if r.observe(sessionState("running", false)) {
		t.Error("caller's session_state_changed frame was hidden")
	}
	if r.observe(&types.SystemMessage{Type: "system", Subtype: "init"}) {
		t.Error("unrelated system message was hidden")
	}
}

// Nothing needs stdin after the prompt, so it closes at once.
func TestWaitForRunEndWithoutBidirectionalNeeds(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	q := NewQuery(ctx, transport, types.NewClaudeAgentOptions(), log.NewLogger(false), false)
	if !waitInputEnded(q, transport, time.Second) {
		t.Fatal("stdin not closed without bidirectional needs")
	}
}

// The bug from Python SDK #1190: a subagent settled just before the result,
// so the CLI still owes a follow-up turn and reports "running". stdin must
// stay open until "idle".
func TestRunWaitsForIdleAfterResult(t *testing.T) {
	q, transport := startRunQuery(t, "60000")
	r := q.run

	r.observe(sessionState("running", true))
	r.observe(resultMsg())
	if isEnded(r) {
		t.Fatal("run ended at a result while the CLI still reported running")
	}
	if waitInputEnded(q, transport, 50*time.Millisecond) {
		t.Fatal("stdin closed before idle")
	}

	// The follow-up turn runs and ends; then the CLI goes idle.
	r.observe(&types.AssistantMessage{Type: "assistant"})
	r.observe(resultMsg())
	if isEnded(r) {
		t.Fatal("run ended before idle")
	}
	r.observe(sessionState("idle", true))
	if !waitInputEnded(q, transport, time.Second) {
		t.Fatal("stdin not closed after idle")
	}
}

// "idle" before the result: the result then ends the run.
func TestRunIdleBeforeResult(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.observe(sessionState("running", true))
	r.observe(sessionState("idle", true))
	if isEnded(r) {
		t.Fatal("run ended at idle before any result")
	}
	r.observe(resultMsg())
	if !isEnded(r) {
		t.Fatal("run not ended at the result after idle")
	}
}

// A CLI that sends no state: the first result with nothing in flight ends it.
func TestRunWithoutSessionStateEndsAtResult(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.observe(resultMsg())
	if !isEnded(r) {
		t.Fatal("run not ended at the result from a CLI without session state")
	}
}

// A background agent still in flight at the result keeps the run open (#1088);
// a background shell does not.
func TestRunInflightAgentKeepsRunOpen(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.observe(&types.TaskStartedMessage{Type: "task_started", TaskID: "sh", TaskType: "local_bash"})
	r.observe(&types.TaskStartedMessage{Type: "task_started", TaskID: "a1", TaskType: "local_agent"})
	r.observe(resultMsg())
	if isEnded(r) {
		t.Fatal("run ended with a background agent in flight")
	}
	r.observe(&types.TaskNotificationMessage{Type: "task_notification", TaskID: "a1", Status: "completed"})
	r.observe(resultMsg())
	if !isEnded(r) {
		t.Fatal("run not ended once the agent settled")
	}
}

func TestRunTaskUpdatedTerminalSettles(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.observe(&types.TaskStartedMessage{Type: "task_started", TaskID: "a1", TaskType: "local_workflow"})
	r.observe(&types.TaskUpdatedMessage{Type: "system", Subtype: "task_updated", TaskID: "a1", Status: "running"})
	r.observe(resultMsg())
	if isEnded(r) {
		t.Fatal("run ended with a running workflow")
	}
	r.observe(&types.TaskUpdatedMessage{Type: "system", Subtype: "task_updated", TaskID: "a1", Status: "killed"})
	r.observe(resultMsg())
	if !isEnded(r) {
		t.Fatal("run not ended once the workflow was killed")
	}
}

// The ceiling ends a run the CLI keeps reporting as running with no new turn.
func TestRunCeilingEndsRun(t *testing.T) {
	r := newRunTracker(true, 30*time.Millisecond)
	r.observe(sessionState("running", true))
	r.observe(resultMsg())
	select {
	case <-r.endedChan():
	case <-time.After(2 * time.Second):
		t.Fatal("ceiling did not end the run")
	}
}

// The ceiling counts only the wait between turns: a turn under way, or a
// request the SDK is answering, stops it.
func TestRunCeilingStoppedByTurnAndRequest(t *testing.T) {
	r := newRunTracker(true, 30*time.Millisecond)
	r.observe(sessionState("running", true))
	r.observe(resultMsg())
	r.observe(&types.StreamEvent{Type: "stream_event"})
	time.Sleep(100 * time.Millisecond)
	if isEnded(r) {
		t.Fatal("ceiling ended the run during a turn")
	}

	r.observe(resultMsg())
	r.observe(sessionState("requires_action", true))
	time.Sleep(100 * time.Millisecond)
	if isEnded(r) {
		t.Fatal("ceiling ended the run while a request was being answered")
	}
}

// A subagent's own messages are not main-thread turn activity.
func TestRunSubagentMessagesDoNotReopen(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.observe(resultMsg())
	if !isEnded(r) {
		t.Fatal("run not ended at the result")
	}
	parent := "toolu_1"
	r.observe(&types.AssistantMessage{Type: "assistant", ParentToolUseID: &parent})
	if !isEnded(r) {
		t.Fatal("a subagent message reopened the run")
	}
	r.observe(&types.AssistantMessage{Type: "assistant"})
	if isEnded(r) {
		t.Fatal("a main-thread turn did not reopen the run")
	}
}

// Once stdin is closed the run stays ended.
func TestRunFinalDoesNotReopen(t *testing.T) {
	r := newRunTracker(true, time.Minute)
	r.finish()
	r.observe(sessionState("running", true))
	if !isEnded(r) {
		t.Fatal("run reopened after it was final")
	}
}

// Stopping the query releases a waiter, so it can't stall with no CLI left
// to report idle.
func TestWaitForRunEndReleasedByStop(t *testing.T) {
	q, _ := startRunQuery(t, "0")
	done := make(chan struct{})
	go func() {
		_ = q.WaitForRunEndAndEndInput(context.Background())
		close(done)
	}()

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = q.Stop(stopCtx)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waiter not released by Stop")
	}
}

// Host-only frames never reach the consumer channel.
func TestRouteMessageHidesHostOnlyFrames(t *testing.T) {
	q, transport := startRunQuery(t, "0")
	transport.sendMessage(sessionState("running", true))
	transport.sendMessage(&types.AssistantMessage{Type: "assistant"})

	select {
	case msg := <-q.GetMessages(context.Background()):
		if _, ok := msg.(*types.AssistantMessage); !ok {
			t.Fatalf("first delivered message = %T, want *types.AssistantMessage", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message delivered")
	}
}
