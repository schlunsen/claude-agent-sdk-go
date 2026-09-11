package internal

import (
	"context"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// When the CLI exits on its own, its transport's message channel closes. The
// consumer channel must then close too, after delivering what was already
// routed. Only Stop used to close it, so ReceiveResponse blocked forever after
// a CLI crash and a consumer had no way to notice the CLI was gone.
func TestMessagesChannelClosesWhenTransportEnds(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	q := NewQuery(ctx, transport, types.NewClaudeAgentOptions(), log.NewLogger(false), true)
	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	transport.messagesChan <- &types.AssistantMessage{Type: "assistant"}
	_ = transport.Close(ctx) // the CLI exited

	msgs := q.GetMessages(ctx)
	got := 0
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-msgs:
			if !ok {
				if got != 1 {
					t.Fatalf("delivered %d messages before closing, want 1", got)
				}
				return
			}
			got++
		case <-deadline:
			t.Fatal("consumer channel still open after the transport ended")
		}
	}
}

// Stop after the loop has already closed the channel must not panic, and
// neither may a second Stop.
func TestStopAfterTransportEndedDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	q := NewQuery(ctx, transport, types.NewClaudeAgentOptions(), log.NewLogger(false), true)
	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = transport.Close(ctx)
	select {
	case <-q.readLoopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("message loop did not stop after the transport ended")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := q.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := q.Stop(stopCtx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}
