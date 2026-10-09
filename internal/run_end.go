package internal

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// A result frame ends one turn, not necessarily the run. A background agent
// that finishes just before the result still wakes the session for a follow-up
// turn, and that turn's hook, permission and SDK MCP requests need stdin. The
// CLI reports session_state_changed "running" while such a turn is owed and
// "idle" once none is, so the SDK keeps stdin open until "idle" (Python SDK
// #1190). A CLI that reports no state falls back to the first result with no
// tracked background agent in flight (#1088).

const (
	// runEndCeilingEnv is the CLI's own wait for background work once stdin
	// is closed; the SDK bounds its wait for "idle" by the same value.
	runEndCeilingEnv = "CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS"
	// DefaultRunEndCeiling is the CLI's default for runEndCeilingEnv.
	DefaultRunEndCeiling = 10 * time.Minute
	// maxRunEndCeilingMS is the longest ceiling honored (~24.8 days), as in
	// the TypeScript SDK, whose timers cannot run longer.
	maxRunEndCeilingMS = 1<<31 - 1

	sessionStateIdle           = "idle"
	sessionStateRequiresAction = "requires_action"
)

// deferringTaskTypes are the task types whose completion wakes the parent for
// a follow-up turn. A background shell may never reach a terminal status, so
// it is not tracked; the CLI's own post-close cleanup bounds it instead.
var deferringTaskTypes = map[string]bool{
	"local_agent":    true,
	"local_workflow": true,
}

// RunEndCeiling reads CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS as the CLI will see
// it: optionsEnv (ClaudeAgentOptions.Env) overrides the inherited environment.
// 0 means no limit. Anything that is not a plain non-negative integer falls
// back to the CLI's default of 10 minutes.
func RunEndCeiling(optionsEnv map[string]string) time.Duration {
	raw, ok := optionsEnv[runEndCeilingEnv]
	if !ok {
		raw, ok = os.LookupEnv(runEndCeilingEnv)
	}
	if !ok {
		return DefaultRunEndCeiling
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms < 0 {
		return DefaultRunEndCeiling
	}
	if ms > maxRunEndCeilingMS {
		ms = maxRunEndCeilingMS
	}
	return time.Duration(ms) * time.Millisecond
}

// runTracker follows the run's lifecycle from the CLI's frames so stdin is
// closed only once no further turn is owed.
type runTracker struct {
	mu sync.Mutex

	// bidirectional is set when the CLI may send control requests that need
	// a reply (hooks, CanUseTool, SDK MCP servers). Without them nothing
	// needs stdin after the prompt, so a result ends the run.
	bidirectional bool
	ceiling       time.Duration

	// ended is closed when the run is over. Work the CLI takes up after the
	// run ended swaps in a fresh channel (reopen).
	ended       chan struct{}
	endedClosed bool
	// final is set once stdin is closed or the reader is gone: the run then
	// stays ended, since nothing can wait on a reopened one.
	final bool

	resultReceived bool
	// turnInProgress: a main-thread turn has started and its result has not
	// arrived. The ceiling counts only the wait between turns.
	turnInProgress bool
	// sessionState is the CLI's latest session_state_changed state, or ""
	// while it sends none (a CLI too old to honor SDKReadsSessionStateEnv).
	sessionState string
	inflight     map[string]struct{}

	ceilingTimer *time.Timer
	// ceilingGen tells a timer that fired after it was cleared or re-armed
	// to stand down.
	ceilingGen int
}

func newRunTracker(bidirectional bool, ceiling time.Duration) *runTracker {
	return &runTracker{
		bidirectional: bidirectional,
		ceiling:       ceiling,
		ended:         make(chan struct{}),
		inflight:      make(map[string]struct{}),
	}
}

// observe updates the run state from a message on its way to the consumer and
// reports whether the message should be hidden from the consumer.
func (r *runTracker) observe(msg types.Message) (hide bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch m := msg.(type) {
	case *types.TaskStartedMessage:
		if m.TaskID != "" && deferringTaskTypes[m.TaskType] {
			r.inflight[m.TaskID] = struct{}{}
		}
	case *types.TaskNotificationMessage:
		r.settleTask(m.TaskID)
	case *types.TaskUpdatedMessage:
		if types.IsTerminalTaskStatus(m.Status) {
			r.settleTask(m.TaskID)
		}
	case *types.SystemMessage:
		if m.Subtype != "session_state_changed" {
			return false
		}
		state, _ := m.Data["state"].(string)
		r.onSessionState(state)
		// The CLI sent these only because the transport asked for them; the
		// caller did not opt in.
		hostOnly, _ := m.Data["sdk_host_only"].(bool)
		return hostOnly
	case *types.ResultMessage:
		r.resultReceived = true
		r.turnInProgress = false
		switch {
		case r.sessionState == "" || r.sessionState == sessionStateIdle || !r.bidirectional:
			// Some hosts send "idle" just before the result. Without state
			// events the result is all there is to go on.
			r.maybeEndRun()
		case r.sessionState != sessionStateRequiresAction:
			// While the SDK is still answering a request the ceiling waits
			// for the "running" that follows.
			r.armCeiling()
		}
	case *types.AssistantMessage:
		if m.ParentToolUseID == nil {
			r.onMainTurnActivity()
		}
	case *types.StreamEvent:
		if m.ParentToolUseID == nil {
			r.onMainTurnActivity()
		}
	}
	return false
}

// settleTask clears a finished task. Once the last tracked agent settles, the
// wait between turns starts over.
func (r *runTracker) settleTask(taskID string) {
	if _, ok := r.inflight[taskID]; !ok {
		return
	}
	delete(r.inflight, taskID)
	if len(r.inflight) == 0 {
		r.rearmCeilingBetweenTurns()
	}
}

// onMainTurnActivity: a main-thread turn is under way, so the ceiling stops
// and the run reopens even if the ceiling ended it while no state changed.
func (r *runTracker) onMainTurnActivity() {
	r.turnInProgress = true
	r.reopen()
	r.clearCeiling()
}

func (r *runTracker) onSessionState(state string) {
	r.sessionState = state
	if state == sessionStateIdle {
		if r.resultReceived {
			r.maybeEndRun()
		}
		return
	}
	// Work the CLI took up after the run ended (a finished background task
	// woke it) reopens the run until the next "idle".
	r.reopen()
	if state == sessionStateRequiresAction {
		// The host is answering a request; stdin must outlast it.
		r.clearCeiling()
	} else {
		r.rearmCeilingBetweenTurns()
	}
}

// maybeEndRun ends the run unless a tracked background agent is still in
// flight: it may still need stdin for hook and SDK MCP requests (#1088), and
// its completion wakes the parent for a later result or "idle".
func (r *runTracker) maybeEndRun() {
	if len(r.inflight) > 0 {
		return
	}
	r.endRun()
}

func (r *runTracker) endRun() {
	r.clearCeiling()
	if !r.endedClosed {
		close(r.ended)
		r.endedClosed = true
	}
}

func (r *runTracker) reopen() {
	if r.endedClosed && !r.final {
		r.ended = make(chan struct{})
		r.endedClosed = false
	}
}

// armCeiling ends the run anyway once the ceiling passes with no new turn.
// The CLI's own background-wait ceiling only counts once stdin is closed, so
// without this, work that never finishes would hold "running", and stdin,
// open forever.
func (r *runTracker) armCeiling() {
	r.clearCeiling()
	if r.ceiling <= 0 || r.endedClosed || r.final || r.turnInProgress || !r.bidirectional {
		return
	}
	gen := r.ceilingGen
	r.ceilingTimer = time.AfterFunc(r.ceiling, func() { r.ceilingFired(gen) })
}

// rearmCeilingBetweenTurns restarts the ceiling if the run is between turns,
// past a result, with the CLI still reporting work.
func (r *runTracker) rearmCeilingBetweenTurns() {
	if r.resultReceived && r.sessionState != "" &&
		r.sessionState != sessionStateIdle && r.sessionState != sessionStateRequiresAction {
		r.armCeiling()
	}
}

func (r *runTracker) ceilingFired(gen int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gen != r.ceilingGen {
		return
	}
	r.ceilingTimer = nil
	// A tracked background agent may still need stdin; the ceiling starts
	// over once it settles.
	if len(r.inflight) > 0 {
		return
	}
	r.endRun()
}

func (r *runTracker) clearCeiling() {
	r.ceilingGen++
	if r.ceilingTimer != nil {
		r.ceilingTimer.Stop()
		r.ceilingTimer = nil
	}
}

// finish marks the run over for good: stdin is closed or the reader is gone.
func (r *runTracker) finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.final = true
	r.endRun()
}

// endedChan returns the channel closed when the current run ends.
func (r *runTracker) endedChan() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ended
}

// WaitForRunEndAndEndInput closes stdin once the run is over. With hooks,
// CanUseTool or SDK MCP servers configured it first waits for the CLI to report
// "idle" after a result (or, from a CLI that reports no session state, for the
// first result with no tracked background agent in flight); between turns the
// wait is bounded by CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS. Without them stdin
// is closed at once: the CLI finishes the run, background work included, and
// exits on its own. Returns ctx.Err() if ctx ends or the query stops first,
// leaving stdin open.
func (q *Query) WaitForRunEndAndEndInput(ctx context.Context) error {
	if q.run.bidirectional {
		q.logger.Debug("Waiting for the run to end before closing stdin")
		select {
		case <-q.run.endedChan():
		case <-ctx.Done():
			return ctx.Err()
		case <-q.ctx.Done():
			return q.ctx.Err()
		}
	}
	q.run.finish()
	return q.transport.EndInput(ctx)
}
