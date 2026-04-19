package rtk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeFakeRTK writes a shell script named "rtk" into a fresh dir,
// makes it executable, and prepends that dir to PATH for the test.
// The script echoes `stdout`, writes `stderr` to stderr, and exits
// with `exitCode`. It also writes the invocation's argv to a sidecar
// file so assertions can inspect the flags we passed.
func makeFakeRTK(t *testing.T, stdout, stderr string, exitCode int) (argvLog string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake rtk uses POSIX shell; skipping on Windows")
	}
	dir := t.TempDir()
	argvLog = filepath.Join(dir, "argv.log")
	script := `#!/bin/sh
# Record argv (one arg per line) so tests can inspect the invocation.
for a in "$@"; do printf '%s\n' "$a"; done > "` + argvLog + `"
# Emit stdout/stderr verbatim.
printf '%s' ` + shellQuote(stdout) + `
printf '%s' ` + shellQuote(stderr) + ` >&2
exit ` + itoa(exitCode) + `
`
	p := filepath.Join(dir, "rtk")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake rtk: %v", err)
	}
	// Prepend dir to PATH — exec.LookPath hits our fake first.
	orig := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+orig)
	return argvLog
}

func shellQuote(s string) string {
	// Minimal single-quote wrap; embedded single quotes get escaped.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func readArgv(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argv log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	// When the log is empty (no args) Split returns [""]; normalise.
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func TestGain_SummaryOnly(t *testing.T) {
	const out = `{
  "summary": {
    "total_commands": 42,
    "total_input": 123456,
    "total_output": 7890,
    "total_saved": 115566,
    "avg_savings_pct": 93.6,
    "total_time_ms": 5000,
    "avg_time_ms": 119
  }
}`
	log := makeFakeRTK(t, out, "", 0)

	got, err := Gain(context.Background())
	if err != nil {
		t.Fatalf("Gain: %v", err)
	}
	if got.Summary.TotalCommands != 42 {
		t.Errorf("TotalCommands = %d, want 42", got.Summary.TotalCommands)
	}
	if got.Summary.AvgSavingsPct != 93.6 {
		t.Errorf("AvgSavingsPct = %v, want 93.6", got.Summary.AvgSavingsPct)
	}
	if len(got.Daily) != 0 || len(got.Weekly) != 0 || len(got.Monthly) != 0 {
		t.Errorf("expected empty breakdowns, got daily=%d weekly=%d monthly=%d",
			len(got.Daily), len(got.Weekly), len(got.Monthly))
	}

	argv := readArgv(t, log)
	wantPrefix := []string{"gain", "--format", "json"}
	for i, want := range wantPrefix {
		if i >= len(argv) || argv[i] != want {
			t.Fatalf("argv = %v, want prefix %v", argv, wantPrefix)
		}
	}
	if len(argv) != len(wantPrefix) {
		t.Errorf("extra args: %v", argv[len(wantPrefix):])
	}
}

func TestGain_WithAll_ParsesBreakdowns(t *testing.T) {
	const out = `{
  "summary": {
    "total_commands": 2, "total_input": 100, "total_output": 20,
    "total_saved": 80, "avg_savings_pct": 80.0,
    "total_time_ms": 100, "avg_time_ms": 50
  },
  "daily": [
    {"date":"2026-04-18","commands":1,"input_tokens":50,"output_tokens":10,
     "saved_tokens":40,"savings_pct":80,"total_time_ms":50,"avg_time_ms":50},
    {"date":"2026-04-19","commands":1,"input_tokens":50,"output_tokens":10,
     "saved_tokens":40,"savings_pct":80,"total_time_ms":50,"avg_time_ms":50}
  ],
  "weekly": [
    {"week_start":"2026-04-13","week_end":"2026-04-19","commands":2,
     "input_tokens":100,"output_tokens":20,"saved_tokens":80,
     "savings_pct":80,"total_time_ms":100,"avg_time_ms":50}
  ],
  "monthly": [
    {"month":"2026-04","commands":2,"input_tokens":100,"output_tokens":20,
     "saved_tokens":80,"savings_pct":80,"total_time_ms":100,"avg_time_ms":50}
  ]
}`
	log := makeFakeRTK(t, out, "", 0)

	got, err := Gain(context.Background(), WithAll(), WithProject("/tmp/proj"))
	if err != nil {
		t.Fatalf("Gain: %v", err)
	}
	if len(got.Daily) != 2 || got.Daily[1].Date != "2026-04-19" {
		t.Errorf("Daily mismatch: %+v", got.Daily)
	}
	if len(got.Weekly) != 1 || got.Weekly[0].WeekEnd != "2026-04-19" {
		t.Errorf("Weekly mismatch: %+v", got.Weekly)
	}
	if len(got.Monthly) != 1 || got.Monthly[0].Month != "2026-04" {
		t.Errorf("Monthly mismatch: %+v", got.Monthly)
	}

	argv := readArgv(t, log)
	want := []string{"gain", "--format", "json", "--all", "--project", "/tmp/proj"}
	if !equalStrings(argv, want) {
		t.Errorf("argv = %v, want %v", argv, want)
	}
}

func TestGain_IndividualBreakdownFlags(t *testing.T) {
	log := makeFakeRTK(t, `{"summary":{"total_commands":0,"total_input":0,"total_output":0,"total_saved":0,"avg_savings_pct":0,"total_time_ms":0,"avg_time_ms":0}}`, "", 0)

	if _, err := Gain(context.Background(), WithDaily(), WithWeekly(), WithMonthly()); err != nil {
		t.Fatalf("Gain: %v", err)
	}
	argv := readArgv(t, log)
	want := []string{"gain", "--format", "json", "--daily", "--weekly", "--monthly"}
	if !equalStrings(argv, want) {
		t.Errorf("argv = %v, want %v", argv, want)
	}
}

func TestGain_AllSupersedesIndividualFlags(t *testing.T) {
	// If callers pass both WithAll and WithDaily, we should emit --all
	// only (mirrors rtk's own behavior where --all is a superset).
	log := makeFakeRTK(t, `{"summary":{"total_commands":0,"total_input":0,"total_output":0,"total_saved":0,"avg_savings_pct":0,"total_time_ms":0,"avg_time_ms":0}}`, "", 0)

	if _, err := Gain(context.Background(), WithDaily(), WithAll()); err != nil {
		t.Fatalf("Gain: %v", err)
	}
	argv := readArgv(t, log)
	for _, a := range argv {
		if a == "--daily" {
			t.Errorf("expected WithAll to suppress --daily, got argv=%v", argv)
		}
	}
	if !containsString(argv, "--all") {
		t.Errorf("expected --all in argv=%v", argv)
	}
}

func TestGain_NonZeroExit_ReturnsGainError(t *testing.T) {
	makeFakeRTK(t, "", "boom: no tracking db", 2)

	_, err := Gain(context.Background())
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	var ge *GainError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GainError, got %T: %v", err, err)
	}
	if !strings.Contains(ge.Error(), "boom: no tracking db") {
		t.Errorf("error should surface stderr, got %q", ge.Error())
	}
	if ge.Unwrap() == nil {
		t.Error("GainError.Unwrap() should be non-nil")
	}
}

func TestGain_EmptyStdout_IsError(t *testing.T) {
	makeFakeRTK(t, "", "", 0)
	_, err := Gain(context.Background())
	if err == nil {
		t.Fatal("expected error on empty stdout")
	}
	var ge *GainError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GainError, got %T: %v", err, err)
	}
}

func TestGain_BadJSON_IsError(t *testing.T) {
	makeFakeRTK(t, "not json", "", 0)
	_, err := Gain(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
	var ge *GainError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GainError, got %T: %v", err, err)
	}
	if !strings.Contains(ge.Error(), "decode") {
		t.Errorf("expected decode in error, got %q", ge.Error())
	}
}

func TestGain_NotInstalled_ReturnsSentinel(t *testing.T) {
	// Point PATH at an empty directory so `rtk` cannot be resolved.
	t.Setenv("PATH", t.TempDir())

	_, err := Gain(context.Background())
	if !errors.Is(err, ErrRTKNotInstalled) {
		t.Fatalf("want ErrRTKNotInstalled, got %v", err)
	}
}

func TestGain_WithGainBinary(t *testing.T) {
	dir := t.TempDir()
	// Rename script to "rtk-custom" to prove WithGainBinary overrides lookup.
	path := filepath.Join(dir, "rtk-custom")
	script := `#!/bin/sh
printf '%s' '{"summary":{"total_commands":1,"total_input":0,"total_output":0,"total_saved":0,"avg_savings_pct":0,"total_time_ms":0,"avg_time_ms":0}}'
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := Gain(context.Background(), WithGainBinary("rtk-custom"))
	if err != nil {
		t.Fatalf("Gain: %v", err)
	}
	if got.Summary.TotalCommands != 1 {
		t.Errorf("TotalCommands = %d, want 1", got.Summary.TotalCommands)
	}
}

func TestTrackingDBPath(t *testing.T) {
	// Force a deterministic HOME so the test is platform-agnostic.
	t.Setenv("HOME", "/tmp/rtkhome")
	// Clear XDG_DATA_HOME so the Linux branch hits the fallback.
	t.Setenv("XDG_DATA_HOME", "")

	got, err := TrackingDBPath()
	if err != nil {
		t.Fatalf("TrackingDBPath: %v", err)
	}
	var want string
	switch runtime.GOOS {
	case "darwin":
		want = "/tmp/rtkhome/Library/Application Support/rtk/tracking.db"
	case "windows":
		// On windows UserHomeDir may ignore HOME. Accept anything ending with rtk/tracking.db.
		if !strings.HasSuffix(got, filepath.Join("rtk", "tracking.db")) {
			t.Errorf("windows path = %q, want suffix rtk/tracking.db", got)
		}
		return
	default:
		want = "/tmp/rtkhome/.local/share/rtk/tracking.db"
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTrackingDBPath_XDG(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG is Linux/BSD only")
	}
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	got, err := TrackingDBPath()
	if err != nil {
		t.Fatalf("TrackingDBPath: %v", err)
	}
	if got != "/xdg/data/rtk/tracking.db" {
		t.Errorf("got %q, want /xdg/data/rtk/tracking.db", got)
	}
}

func TestIsInstalled(t *testing.T) {
	makeFakeRTK(t, "", "", 0)
	if !IsInstalled("") {
		t.Error("IsInstalled(\"\") = false, want true after makeFakeRTK")
	}
	if !IsInstalled("rtk") {
		t.Error("IsInstalled(\"rtk\") = false, want true")
	}
	if IsInstalled("definitely-not-a-real-binary-xyzzy") {
		t.Error("IsInstalled on missing binary = true")
	}
}

// equalStrings reports whether a == b element-wise.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
