package rtk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GainExport mirrors the top-level object rtk emits from
// `rtk gain --format json`. The Daily/Weekly/Monthly slices are only
// populated when the corresponding flag (WithDaily/WithWeekly/...) is set.
//
// Upstream source: rtk-ai/rtk src/analytics/gain.rs ExportData.
type GainExport struct {
	Summary GainSummary  `json:"summary"`
	Daily   []DayStats   `json:"daily,omitempty"`
	Weekly  []WeekStats  `json:"weekly,omitempty"`
	Monthly []MonthStats `json:"monthly,omitempty"`
}

// GainSummary is the aggregate section of rtk gain JSON.
type GainSummary struct {
	TotalCommands int64   `json:"total_commands"`
	TotalInput    int64   `json:"total_input"`
	TotalOutput   int64   `json:"total_output"`
	TotalSaved    int64   `json:"total_saved"`
	AvgSavingsPct float64 `json:"avg_savings_pct"`
	TotalTimeMs   uint64  `json:"total_time_ms"`
	AvgTimeMs     uint64  `json:"avg_time_ms"`
}

// DayStats is one bucket in GainExport.Daily. Date is "YYYY-MM-DD".
type DayStats struct {
	Date         string  `json:"date"`
	Commands     int64   `json:"commands"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	SavedTokens  int64   `json:"saved_tokens"`
	SavingsPct   float64 `json:"savings_pct"`
	TotalTimeMs  uint64  `json:"total_time_ms"`
	AvgTimeMs    uint64  `json:"avg_time_ms"`
}

// WeekStats is one bucket in GainExport.Weekly.
// WeekStart/WeekEnd are "YYYY-MM-DD".
type WeekStats struct {
	WeekStart    string  `json:"week_start"`
	WeekEnd      string  `json:"week_end"`
	Commands     int64   `json:"commands"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	SavedTokens  int64   `json:"saved_tokens"`
	SavingsPct   float64 `json:"savings_pct"`
	TotalTimeMs  uint64  `json:"total_time_ms"`
	AvgTimeMs    uint64  `json:"avg_time_ms"`
}

// MonthStats is one bucket in GainExport.Monthly. Month is "YYYY-MM".
type MonthStats struct {
	Month        string  `json:"month"`
	Commands     int64   `json:"commands"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	SavedTokens  int64   `json:"saved_tokens"`
	SavingsPct   float64 `json:"savings_pct"`
	TotalTimeMs  uint64  `json:"total_time_ms"`
	AvgTimeMs    uint64  `json:"avg_time_ms"`
}

// gainConfig holds resolved options for Gain().
type gainConfig struct {
	binary  string
	project string
	daily   bool
	weekly  bool
	monthly bool
	all     bool
	env     []string
}

// GainOption customises a Gain() call.
type GainOption func(*gainConfig)

// WithGainBinary overrides the rtk binary path (defaults to "rtk").
func WithGainBinary(path string) GainOption {
	return func(c *gainConfig) {
		if path != "" {
			c.binary = path
		}
	}
}

// WithProject filters stats to commands whose project_path matches the
// supplied path (rtk's `--project` / `-p` flag).
func WithProject(path string) GainOption {
	return func(c *gainConfig) { c.project = path }
}

// WithDaily includes the Daily breakdown in the export.
func WithDaily() GainOption { return func(c *gainConfig) { c.daily = true } }

// WithWeekly includes the Weekly breakdown in the export.
func WithWeekly() GainOption { return func(c *gainConfig) { c.weekly = true } }

// WithMonthly includes the Monthly breakdown in the export.
func WithMonthly() GainOption { return func(c *gainConfig) { c.monthly = true } }

// WithAll includes all breakdowns (daily, weekly, monthly) in one call.
// This is rtk's `--all` flag.
func WithAll() GainOption { return func(c *gainConfig) { c.all = true } }

// WithGainEnv appends extra environment variables (KEY=value) to the
// rtk subprocess. Callers who need to disable telemetry can pass
// WithGainEnv("RTK_TELEMETRY_DISABLED=1"). The host environment is
// inherited; entries here are appended last and therefore win.
func WithGainEnv(kv ...string) GainOption {
	return func(c *gainConfig) {
		for _, e := range kv {
			if e != "" {
				c.env = append(c.env, e)
			}
		}
	}
}

// ErrRTKNotInstalled is returned when the rtk binary cannot be resolved
// on PATH (or the path supplied to WithGainBinary does not exist).
var ErrRTKNotInstalled = errors.New("rtk: binary not found on PATH")

// Gain runs `rtk gain --format json` with the supplied options and
// decodes the result into a GainExport. Callers who only want the
// top-level summary can pass no options; to populate Daily/Weekly/...
// use WithDaily/WithAll/etc.
//
// Returns ErrRTKNotInstalled if the rtk binary is missing. Other
// non-zero exits are surfaced as *GainError with Stderr populated.
func Gain(ctx context.Context, opts ...GainOption) (*GainExport, error) {
	cfg := &gainConfig{binary: "rtk"}
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}

	// Resolve the binary early so we can return a clean sentinel on the
	// common "rtk not installed" case, instead of a generic exec error.
	if _, err := exec.LookPath(cfg.binary); err != nil {
		return nil, ErrRTKNotInstalled
	}

	args := []string{"gain", "--format", "json"}
	if cfg.all {
		args = append(args, "--all")
	} else {
		if cfg.daily {
			args = append(args, "--daily")
		}
		if cfg.weekly {
			args = append(args, "--weekly")
		}
		if cfg.monthly {
			args = append(args, "--monthly")
		}
	}
	if cfg.project != "" {
		args = append(args, "--project", cfg.project)
	}

	cmd := exec.CommandContext(ctx, cfg.binary, args...)
	if len(cfg.env) > 0 {
		cmd.Env = append(os.Environ(), cfg.env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, &GainError{
			Args:   append([]string{cfg.binary}, args...),
			Err:    err,
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}

	// rtk writes pretty-printed JSON with a trailing newline. json.Unmarshal
	// is tolerant of leading/trailing whitespace so no trim is strictly
	// necessary, but bytes.TrimSpace makes the error messages nicer if
	// the schema ever drifts.
	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil, &GainError{
			Args:   append([]string{cfg.binary}, args...),
			Err:    fmt.Errorf("empty output"),
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}
	var out GainExport
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &GainError{
			Args:   append([]string{cfg.binary}, args...),
			Err:    fmt.Errorf("decode rtk gain json: %w", err),
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}
	return &out, nil
}

// GainError is returned when `rtk gain` fails or emits output we can't
// decode. It preserves the stderr tail so callers can log something
// useful.
type GainError struct {
	Args   []string // resolved argv, useful for "run it yourself" debugging
	Err    error    // underlying exec or decode error
	Stderr string   // trimmed stderr from rtk
}

func (e *GainError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("rtk gain failed (%v): %s", e.Err, e.Stderr)
	}
	return fmt.Sprintf("rtk gain failed: %v", e.Err)
}

func (e *GainError) Unwrap() error { return e.Err }

// TrackingDBPath returns the platform-specific location of rtk's
// tracking SQLite database. The path is computed; it does NOT check
// that the file exists. Layout mirrors rtk's src/core/tracking.rs.
//
//   macOS:   $HOME/Library/Application Support/rtk/tracking.db
//   Linux:   $XDG_DATA_HOME/rtk/tracking.db   (falls back to $HOME/.local/share/rtk/tracking.db)
//   Windows: %APPDATA%/rtk/tracking.db
//
// Returns an empty string and an error if the home directory cannot
// be resolved.
func TrackingDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("rtk: resolve home: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "rtk", "tracking.db"), nil
	case "windows":
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "rtk", "tracking.db"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "rtk", "tracking.db"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "rtk", "tracking.db"), nil
		}
		return filepath.Join(home, ".local", "share", "rtk", "tracking.db"), nil
	}
}

// IsInstalled reports whether the rtk binary can be resolved on PATH.
// This is a thin wrapper around exec.LookPath for callers who want a
// cheap health check without attempting a Gain() call. Pass the empty
// string to check for "rtk" specifically.
func IsInstalled(binary string) bool {
	if binary == "" {
		binary = "rtk"
	}
	_, err := exec.LookPath(binary)
	return err == nil
}
