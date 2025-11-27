# Implementation Plan: Python SDK Parity Updates

**Goal:** Bring Go SDK up to parity with Python SDK v0.1.6
**Target Version:** Go SDK v0.2.3
**Estimated Total Effort:** 3-4 hours

---

## Task 1: Fix Empty System Prompt Default Behavior

### Priority: HIGH (Bug Fix)
### Estimated Time: 30 minutes
### Branch: `fix/empty-system-prompt-default`

### Problem
When `SystemPrompt` is `nil`, Go SDK doesn't pass any `--system-prompt` flag to the CLI. Python SDK now explicitly passes `--system-prompt ""` to ensure no default prompt is used.

### Current Behavior (Go)
```go
// subprocess_cli.go line 289-295
if t.options != nil && t.options.SystemPrompt != nil {
    if promptStr, ok := t.options.SystemPrompt.(string); ok {
        args = append(args, "--system-prompt", promptStr)
    }
}
// If SystemPrompt is nil, nothing is passed
```

### Expected Behavior (Python)
```python
# subprocess_cli.py line 100-111
if self._options.system_prompt is None:
    cmd.extend(["--system-prompt", ""])  # Explicit empty string
elif isinstance(self._options.system_prompt, str):
    cmd.extend(["--system-prompt", self._options.system_prompt])
```

### Implementation Steps

#### 1.1 Update subprocess_cli.go
**File:** `internal/transport/subprocess_cli.go`
**Method:** `buildCommandArgs()`

**Changes:**
```go
// Replace lines 289-295 with:
if t.options != nil {
    if t.options.SystemPrompt == nil {
        // Default to empty system prompt when not specified
        args = append(args, "--system-prompt", "")
        t.logger.Debug("Setting empty system prompt (default)")
    } else if promptStr, ok := t.options.SystemPrompt.(string); ok {
        args = append(args, "--system-prompt", promptStr)
        t.logger.Debug("Setting system prompt: %s", promptStr)
    } else if preset, ok := t.options.SystemPrompt.(types.SystemPromptPreset); ok {
        // Handle preset case (existing logic)
        if preset.Append != nil {
            args = append(args, "--append-system-prompt", *preset.Append)
            t.logger.Debug("Appending to system prompt preset: %s", *preset.Append)
        }
    }
} else {
    // No options provided, use empty system prompt
    args = append(args, "--system-prompt", "")
    t.logger.Debug("Setting empty system prompt (no options)")
}
```

#### 1.2 Add Unit Tests
**File:** `internal/transport/transport_test.go`

**Add test cases:**
```go
func TestBuildCommandArgs_SystemPrompt(t *testing.T) {
    tests := []struct {
        name           string
        systemPrompt   interface{}
        wantFlag       bool
        wantValue      string
    }{
        {
            name:         "nil system prompt should pass empty string",
            systemPrompt: nil,
            wantFlag:     true,
            wantValue:    "",
        },
        {
            name:         "empty string system prompt",
            systemPrompt: "",
            wantFlag:     true,
            wantValue:    "",
        },
        {
            name:         "custom system prompt",
            systemPrompt: "You are a helpful assistant",
            wantFlag:     true,
            wantValue:    "You are a helpful assistant",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            opts := types.NewClaudeAgentOptions()
            if tt.systemPrompt != nil {
                opts.WithSystemPrompt(tt.systemPrompt)
            }

            transport := NewSubprocessCLITransport(
                "/usr/local/bin/claude",
                "",
                nil,
                log.NewLogger(false),
                "",
                opts,
            )

            args := transport.buildCommandArgs()

            // Find --system-prompt flag
            foundFlag := false
            foundValue := ""
            for i, arg := range args {
                if arg == "--system-prompt" && i+1 < len(args) {
                    foundFlag = true
                    foundValue = args[i+1]
                    break
                }
            }

            if foundFlag != tt.wantFlag {
                t.Errorf("wantFlag = %v, got %v", tt.wantFlag, foundFlag)
            }

            if foundValue != tt.wantValue {
                t.Errorf("wantValue = %q, got %q", tt.wantValue, foundValue)
            }
        })
    }
}
```

#### 1.3 Update Documentation
**File:** `README.md`

Add note to system prompt section:
```markdown
### System Prompt

By default, the SDK uses an empty system prompt. You can customize it:

\`\`\`go
// Default: empty system prompt (no Claude Code defaults)
options := types.NewClaudeAgentOptions()

// Custom system prompt
options.WithSystemPrompt("You are a helpful coding assistant")

// Empty system prompt (explicit)
options.WithSystemPrompt("")
\`\`\`
```

#### 1.4 Testing Checklist
- [ ] Test with nil SystemPrompt
- [ ] Test with empty string SystemPrompt
- [ ] Test with actual prompt string
- [ ] Test with SystemPromptPreset
- [ ] All existing tests still pass
- [ ] Integration test with real CLI

---

## Task 2: Add Plugin Support

### Priority: MEDIUM (New Feature)
### Estimated Time: 2.5 hours
### Branch: `feat/plugin-support`

### Overview
Add support for loading custom Claude Code plugins via the `--plugin-dir` CLI flag.

### Implementation Steps

#### 2.1 Add PluginConfig Type
**File:** `types/options.go`

**Add after AgentDefinition (line 29):**
```go
// PluginConfig represents a Claude Code plugin configuration.
// Currently only local plugins are supported.
type PluginConfig struct {
	Type string `json:"type"` // "local" - plugin type
	Path string `json:"path"` // Absolute or relative path to plugin directory
}

// NewPluginConfig creates a new PluginConfig with validation.
func NewPluginConfig(pluginType, path string) (*PluginConfig, error) {
	if pluginType != "local" {
		return nil, fmt.Errorf("unsupported plugin type: %s (only 'local' is supported)", pluginType)
	}
	if path == "" {
		return nil, fmt.Errorf("plugin path cannot be empty")
	}
	return &PluginConfig{
		Type: pluginType,
		Path: path,
	}, nil
}

// NewLocalPluginConfig creates a new local plugin configuration.
// This is a convenience function for the most common plugin type.
func NewLocalPluginConfig(path string) *PluginConfig {
	return &PluginConfig{
		Type: "local",
		Path: path,
	}
}
```

#### 2.2 Add Plugins Field to ClaudeAgentOptions
**File:** `types/options.go`

**Add to ClaudeAgentOptions struct (around line 140):**
```go
// Plugin configurations for custom plugins
Plugins []PluginConfig `json:"plugins,omitempty"`
```

**Update NewClaudeAgentOptions() (around line 152):**
```go
func NewClaudeAgentOptions() *ClaudeAgentOptions {
	return &ClaudeAgentOptions{
		AllowedTools:           []string{},
		DisallowedTools:        []string{},
		Env:                    make(map[string]string),
		ExtraArgs:              make(map[string]*string),
		ContinueConversation:   false,
		ForkSession:            false,
		IncludePartialMessages: false,
		Plugins:                []PluginConfig{}, // Add this line
	}
}
```

#### 2.3 Add Builder Methods
**File:** `types/options.go`

**Add before WithCanUseTool() (around line 355):**
```go
// WithPlugins sets the plugin configurations.
func (o *ClaudeAgentOptions) WithPlugins(plugins []PluginConfig) *ClaudeAgentOptions {
	o.Plugins = plugins
	return o
}

// WithPlugin adds a single plugin configuration.
func (o *ClaudeAgentOptions) WithPlugin(plugin PluginConfig) *ClaudeAgentOptions {
	o.Plugins = append(o.Plugins, plugin)
	return o
}

// WithLocalPlugin adds a local plugin by path (convenience method).
func (o *ClaudeAgentOptions) WithLocalPlugin(path string) *ClaudeAgentOptions {
	o.Plugins = append(o.Plugins, *NewLocalPluginConfig(path))
	return o
}
```

#### 2.4 Implement CLI Argument Generation
**File:** `internal/transport/subprocess_cli.go`

**Add after budget args (around line 340):**
```go
// Add plugin directories
if t.options != nil && len(t.options.Plugins) > 0 {
    for _, plugin := range t.options.Plugins {
        if plugin.Type == "local" {
            args = append(args, "--plugin-dir", plugin.Path)
            t.logger.Debug("Adding plugin directory: %s", plugin.Path)
        } else {
            // This shouldn't happen if NewPluginConfig is used, but handle it anyway
            t.logger.Warning("Skipping unsupported plugin type: %s", plugin.Type)
        }
    }
}
```

#### 2.5 Add Unit Tests
**File:** `types/options_test.go`

**Add test cases:**
```go
func TestPluginConfig(t *testing.T) {
	t.Run("NewLocalPluginConfig", func(t *testing.T) {
		plugin := NewLocalPluginConfig("/path/to/plugin")
		if plugin.Type != "local" {
			t.Errorf("expected Type 'local', got %s", plugin.Type)
		}
		if plugin.Path != "/path/to/plugin" {
			t.Errorf("expected Path '/path/to/plugin', got %s", plugin.Path)
		}
	})

	t.Run("NewPluginConfig with valid type", func(t *testing.T) {
		plugin, err := NewPluginConfig("local", "/path/to/plugin")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if plugin.Type != "local" {
			t.Errorf("expected Type 'local', got %s", plugin.Type)
		}
	})

	t.Run("NewPluginConfig with invalid type", func(t *testing.T) {
		_, err := NewPluginConfig("remote", "/path/to/plugin")
		if err == nil {
			t.Error("expected error for unsupported plugin type")
		}
	})

	t.Run("NewPluginConfig with empty path", func(t *testing.T) {
		_, err := NewPluginConfig("local", "")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})
}

func TestClaudeAgentOptions_Plugins(t *testing.T) {
	t.Run("WithPlugins", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		plugins := []PluginConfig{
			*NewLocalPluginConfig("/path/to/plugin1"),
			*NewLocalPluginConfig("/path/to/plugin2"),
		}
		opts.WithPlugins(plugins)

		if len(opts.Plugins) != 2 {
			t.Errorf("expected 2 plugins, got %d", len(opts.Plugins))
		}
	})

	t.Run("WithPlugin", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		plugin := *NewLocalPluginConfig("/path/to/plugin")
		opts.WithPlugin(plugin)

		if len(opts.Plugins) != 1 {
			t.Errorf("expected 1 plugin, got %d", len(opts.Plugins))
		}
		if opts.Plugins[0].Path != "/path/to/plugin" {
			t.Errorf("expected Path '/path/to/plugin', got %s", opts.Plugins[0].Path)
		}
	})

	t.Run("WithLocalPlugin", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		opts.WithLocalPlugin("/path/to/plugin")

		if len(opts.Plugins) != 1 {
			t.Errorf("expected 1 plugin, got %d", len(opts.Plugins))
		}
		if opts.Plugins[0].Type != "local" {
			t.Errorf("expected Type 'local', got %s", opts.Plugins[0].Type)
		}
	})

	t.Run("multiple plugins via WithPlugin", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		opts.WithPlugin(*NewLocalPluginConfig("/path/1")).
			WithPlugin(*NewLocalPluginConfig("/path/2")).
			WithPlugin(*NewLocalPluginConfig("/path/3"))

		if len(opts.Plugins) != 3 {
			t.Errorf("expected 3 plugins, got %d", len(opts.Plugins))
		}
	})
}
```

**File:** `internal/transport/transport_test.go`

**Add command args test:**
```go
func TestBuildCommandArgs_Plugins(t *testing.T) {
	tests := []struct {
		name        string
		plugins     []types.PluginConfig
		wantFlags   int // Number of --plugin-dir flags expected
	}{
		{
			name:      "no plugins",
			plugins:   []types.PluginConfig{},
			wantFlags: 0,
		},
		{
			name: "single plugin",
			plugins: []types.PluginConfig{
				*types.NewLocalPluginConfig("/path/to/plugin"),
			},
			wantFlags: 1,
		},
		{
			name: "multiple plugins",
			plugins: []types.PluginConfig{
				*types.NewLocalPluginConfig("/path/to/plugin1"),
				*types.NewLocalPluginConfig("/path/to/plugin2"),
			},
			wantFlags: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := types.NewClaudeAgentOptions().WithPlugins(tt.plugins)

			transport := NewSubprocessCLITransport(
				"/usr/local/bin/claude",
				"",
				nil,
				log.NewLogger(false),
				"",
				opts,
			)

			args := transport.buildCommandArgs()

			// Count --plugin-dir flags
			count := 0
			for _, arg := range args {
				if arg == "--plugin-dir" {
					count++
				}
			}

			if count != tt.wantFlags {
				t.Errorf("expected %d --plugin-dir flags, got %d", tt.wantFlags, count)
			}
		})
	}
}
```

#### 2.6 Create Example
**File:** `examples/with_plugins/main.go`

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	sdk "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

func main() {
	// Check for required environment variable
	if os.Getenv("CLAUDE_API_KEY") == "" {
		log.Fatal("CLAUDE_API_KEY environment variable must be set")
	}

	// Create options with a local plugin
	// This assumes you have a Claude Code plugin directory at ./my-plugin
	options := types.NewClaudeAgentOptions().
		WithLocalPlugin("./examples/plugins/demo-plugin").
		WithVerbose(true)

	// You can also add multiple plugins
	// options.WithLocalPlugin("/path/to/plugin1").
	//         WithLocalPlugin("/path/to/plugin2")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Execute query with plugin support
	messages, err := sdk.Query(ctx, "Use the greet command from my demo plugin", options)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// Stream messages
	fmt.Println("=== Claude Response ===\n")
	for msg := range messages {
		switch m := msg.(type) {
		case *types.UserMsg:
			fmt.Printf("[User] %s\n", m.Content)

		case *types.AssistantMsg:
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextContent); ok {
					fmt.Printf("[Assistant] %s\n", textBlock.Text)
				}
			}

		case *types.ResultMsg:
			fmt.Printf("\n=== Query Complete ===\n")
			fmt.Printf("Session ID: %s\n", m.SessionID)
			fmt.Printf("Duration: %dms\n", m.DurationMS)
			fmt.Printf("Cost: $%.4f\n", m.TotalCostUSD)
		}
	}
}
```

**File:** `examples/plugins/demo-plugin/.claude-plugin/plugin.json`

```json
{
  "name": "demo-plugin",
  "version": "1.0.0",
  "description": "A demo plugin for testing",
  "commands": {
    "greet": "commands/greet.md"
  }
}
```

**File:** `examples/plugins/demo-plugin/commands/greet.md`

```markdown
# Greet Command

Say hello to the user in a friendly way.

Example usage:
/greet
```

#### 2.7 Update Documentation
**File:** `README.md`

Add new section:
```markdown
### Plugin Support

Claude Code supports custom plugins to extend functionality. You can load local plugins:

\`\`\`go
options := types.NewClaudeAgentOptions().
    WithLocalPlugin("./my-plugin").
    WithLocalPlugin("./another-plugin")

messages, err := sdk.Query(ctx, "Your prompt here", options)
\`\`\`

**Plugin Structure:**
\`\`\`
my-plugin/
├── .claude-plugin/
│   └── plugin.json
└── commands/
    └── mycommand.md
\`\`\`

See [examples/with_plugins](examples/with_plugins/main.go) for a complete example.
```

#### 2.8 Testing Checklist
- [ ] Test with no plugins
- [ ] Test with single plugin
- [ ] Test with multiple plugins
- [ ] Test with invalid plugin type (should be handled gracefully)
- [ ] Test example with demo plugin
- [ ] All existing tests still pass
- [ ] Integration test with real CLI and plugin

---

## Task 3: Update Version and Changelog

### Priority: LOW (Housekeeping)
### Estimated Time: 15 minutes
### Branch: Same as final task branch

### Steps

#### 3.1 Update Version
**File:** `internal/transport/subprocess_cli.go`

Update line 17:
```go
SDKVersion = "0.2.3"
```

#### 3.2 Update go.mod
```bash
# No changes needed - version is managed by git tags
```

#### 3.3 Create CHANGELOG.md Entry
**File:** `CHANGELOG.md` (create if doesn't exist)

```markdown
# Changelog

All notable changes to this project will be documented in this file.

## [0.2.3] - 2025-10-31

### Fixed
- System prompt now defaults to empty string when not specified, matching Python SDK behavior
- This prevents unintended Claude Code default prompts from being used

### Added
- Plugin support: Load custom Claude Code plugins via `WithLocalPlugin()` or `WithPlugins()`
- `PluginConfig` type for plugin configuration
- Example: `examples/with_plugins/main.go` demonstrating plugin usage

### Changed
- Updated to match Python SDK v0.1.6 feature set

## [0.2.2] - 2025-10-XX

(Previous entries...)
```

---

## Execution Order

1. **Day 1:**
   - Task 1: Fix empty system prompt (30 min)
   - Commit, push, create PR for review
   - Task 2: Plugin support (2.5 hours)
   - Commit, push

2. **Day 2:**
   - Review and address feedback
   - Task 3: Version and changelog (15 min)
   - Merge PRs
   - Tag release v0.2.3

---

## Success Criteria

- [ ] All existing tests pass
- [ ] New tests added and passing
- [ ] Examples run successfully
- [ ] Documentation updated
- [ ] Go SDK behavior matches Python SDK v0.1.6 for analyzed features
- [ ] No breaking changes to existing API

---

## Post-Implementation

### 1. Create GitHub Release
Tag: `v0.2.3`
Title: "Python SDK Parity: System Prompt Fix + Plugin Support"

Release notes:
```markdown
## What's New

### Bug Fixes
- **Empty System Prompt Default**: System prompt now defaults to empty string when not specified, preventing unintended Claude Code defaults

### New Features
- **Plugin Support**: Load custom Claude Code plugins to extend functionality
  - New `PluginConfig` type
  - `WithLocalPlugin()` and `WithPlugins()` methods
  - Example: `examples/with_plugins/main.go`

### Compatibility
This release brings the Go SDK up to parity with Python SDK v0.1.6.

## Full Changelog
See [CHANGELOG.md](CHANGELOG.md) for details.
```

### 2. Monitor Python SDK
Set up weekly check for new Python SDK commits:
```bash
cd /Users/schlunsen/projects/claude-agent-sdk-python
git pull
git log --since="1 week ago" --oneline --no-merges
```

---

## Notes

- Keep commits focused and atomic
- Follow existing code style and patterns
- Run `make fmt`, `make lint`, `make test` before committing
- Update tests alongside code changes
- Document all public APIs

---

## Risk Assessment

### Low Risk
- Empty system prompt fix: Simple logic change, well-tested
- Plugin support: Additive feature, no breaking changes

### Mitigation
- Comprehensive unit tests
- Integration tests with real CLI
- Examples to verify functionality
- Backward compatibility maintained
