package transport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// envReportingCLI writes a stand-in CLI that reports the session-state
// variable it sees as a system message, then echoes stdin until EOF.
func envReportingCLI(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	path := filepath.Join(t.TempDir(), "env-cli")
	script := "#!/bin/sh\n" +
		"printf '{\"type\":\"system\",\"subtype\":\"env\",\"value\":\"%s\"}\\n' \"$" + SDKReadsSessionStateEnv + "\"\n" +
		"exec cat\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stand-in CLI: %v", err)
	}
	return path
}

func reportedSessionStateEnv(t *testing.T, env map[string]string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tr := NewSubprocessCLITransport(envReportingCLI(t), "", env, log.NewLogger(false), "", nil)
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = tr.Close(ctx) }()

	select {
	case msg := <-tr.ReadMessages(ctx):
		sys, ok := msg.(*types.SystemMessage)
		if !ok {
			t.Fatalf("got %T, want *types.SystemMessage", msg)
		}
		value, _ := sys.Data["value"].(string)
		return value
	case <-ctx.Done():
		t.Fatal("stand-in CLI reported nothing")
		return ""
	}
}

// The transport asks the CLI for the session-state frames Query reads.
func TestSessionStateEnvSetByDefault(t *testing.T) {
	t.Setenv(SDKReadsSessionStateEnv, "")
	_ = os.Unsetenv(SDKReadsSessionStateEnv)
	if got := reportedSessionStateEnv(t, nil); got != "1" {
		t.Errorf("%s = %q, want \"1\"", SDKReadsSessionStateEnv, got)
	}
}

// A caller's own value wins, from either the options or the environment.
func TestSessionStateEnvCallerChooses(t *testing.T) {
	t.Setenv(SDKReadsSessionStateEnv, "")
	_ = os.Unsetenv(SDKReadsSessionStateEnv)
	if got := reportedSessionStateEnv(t, map[string]string{SDKReadsSessionStateEnv: "0"}); got != "0" {
		t.Errorf("with options env: %s = %q, want \"0\"", SDKReadsSessionStateEnv, got)
	}

	t.Setenv(SDKReadsSessionStateEnv, "0")
	if got := reportedSessionStateEnv(t, nil); got != "0" {
		t.Errorf("with inherited env: %s = %q, want \"0\"", SDKReadsSessionStateEnv, got)
	}
}

// EndInput closes stdin and lets the CLI exit on its own, which ends the
// message stream; writes after it fail.
func TestEndInputLetsCLIExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tr := NewSubprocessCLITransport(envReportingCLI(t), "", nil, log.NewLogger(false), "", nil)
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = tr.Close(ctx) }()

	if err := tr.Write(ctx, `{"type":"system","subtype":"echo"}`); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tr.EndInput(ctx); err != nil {
		t.Fatalf("EndInput: %v", err)
	}
	if err := tr.EndInput(ctx); err != nil {
		t.Fatalf("second EndInput: %v", err)
	}
	if err := tr.Write(ctx, `{"type":"system"}`); err == nil {
		t.Error("Write after EndInput succeeded")
	}

	msgs := tr.ReadMessages(ctx)
	got := 0
	for {
		select {
		case _, ok := <-msgs:
			if !ok {
				if got != 2 {
					t.Errorf("got %d messages before the stream ended, want 2", got)
				}
				return
			}
			got++
		case <-ctx.Done():
			t.Fatal("message stream did not end after EndInput")
		}
	}
}
