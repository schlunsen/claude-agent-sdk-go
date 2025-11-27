# Go SDK vs Python SDK Comparison Analysis

**Analysis Date:** October 31, 2025
**Python SDK Commits Analyzed:** Last 6 weeks (since ~Sept 15, 2025)
**Go SDK Current State:** v0.2.2

## Recent Python SDK Changes (Last 6 Weeks)

### 1. ✅ CLI Path Discovery - ~/.claude/local/claude (Commit 5a4cc2f)
**Status:** ✅ **ALREADY IMPLEMENTED** in Go SDK

Python added `~/.claude/local/claude` to CLI discovery locations.
- **Go Implementation:** Already in `internal/transport/cli_discovery.go` line 41
- **No action needed**

### 2. ✅ max_budget_usd Option (Commit ae800c5)
**Status:** ✅ **ALREADY IMPLEMENTED** in Go SDK

Adds budget limiting to prevent excessive API costs.
- **Python:** `max_budget_usd` field + `--max-budget-usd` flag
- **Go Implementation:** Already in `types/options.go` line 112 and `subprocess_cli.go` line 337-340
- **No action needed**

### 3. ✅ max_thinking_tokens Option (Commit 7be296f)
**Status:** ✅ **ALREADY IMPLEMENTED** in Go SDK

Controls maximum tokens for extended thinking blocks.
- **Python:** `max_thinking_tokens` field + `--max-thinking-tokens` flag
- **Go Implementation:** Already in `types/options.go` line 111 and `subprocess_cli.go` line 331-334
- **No action needed**

### 4. ⚠️ Empty System Prompt by Default (Commit 841f8c0)
**Status:** ⚠️ **PARTIALLY MISSING** - Bug Fix Needed

**Issue:** When `system_prompt` is `None`/`nil`, Python SDK now passes `--system-prompt ""` to the CLI.

**Current Go Behavior:**
```go
// In subprocess_cli.go line 289-295
if t.options != nil && t.options.SystemPrompt != nil {
    if promptStr, ok := t.options.SystemPrompt.(string); ok {
        args = append(args, "--system-prompt", promptStr)
    }
}
```
- **Problem:** When `SystemPrompt` is `nil`, Go SDK doesn't pass any flag
- **Expected:** Should pass `--system-prompt ""` when `nil`

**Why This Matters:**
The CLI has a default system prompt that includes Claude Code specific instructions. Passing empty string ensures no default prompt is used unless explicitly set.

**Python Implementation:**
```python
# subprocess_cli.py line 100-111
if self._options.system_prompt is None:
    cmd.extend(["--system-prompt", ""])
elif isinstance(self._options.system_prompt, str):
    cmd.extend(["--system-prompt", self._options.system_prompt])
else:
    # Handle preset case...
```

**Action Required:** Fix system prompt handling to match Python behavior

---

### 5. ❌ Plugin Support (Commit c595763)
**Status:** ❌ **COMPLETELY MISSING** - New Feature Needed

**Feature:** Support for loading custom Claude Code plugins.

**Python Implementation:**
```python
# types.py line 409-417
class SdkPluginConfig(TypedDict):
    """SDK plugin configuration.

    Currently only local plugins are supported via the 'local' type.
    """
    type: Literal["local"]
    path: str

# ClaudeAgentOptions line 557
plugins: list[SdkPluginConfig] = field(default_factory=list)
```

**CLI Integration:**
```python
# subprocess_cli.py line 198-204
if self._options.plugins:
    for plugin in self._options.plugins:
        if plugin["type"] == "local":
            cmd.extend(["--plugin-dir", plugin["path"]])
        else:
            raise ValueError(f"Unsupported plugin type: {plugin['type']}")
```

**Go Implementation Needed:**
1. Add `PluginConfig` type to `types/options.go`:
   ```go
   type PluginConfig struct {
       Type string `json:"type"` // "local"
       Path string `json:"path"`
   }
   ```

2. Add `Plugins` field to `ClaudeAgentOptions`:
   ```go
   Plugins []PluginConfig `json:"plugins,omitempty"`
   ```

3. Add command line argument building in `subprocess_cli.go`:
   ```go
   // Add plugin directories
   if t.options != nil && len(t.options.Plugins) > 0 {
       for _, plugin := range t.options.Plugins {
           if plugin.Type == "local" {
               args = append(args, "--plugin-dir", plugin.Path)
               t.logger.Debug("Adding plugin directory: %s", plugin.Path)
           } else {
               return nil, fmt.Errorf("unsupported plugin type: %s", plugin.Type)
           }
       }
   }
   ```

4. Add builder method to `ClaudeAgentOptions`:
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
   ```

**Action Required:** Implement plugin support

---

## Summary

| Feature | Python SDK | Go SDK | Status | Priority |
|---------|-----------|--------|--------|----------|
| ~/.claude/local/claude path | ✅ | ✅ | Complete | - |
| max_budget_usd | ✅ | ✅ | Complete | - |
| max_thinking_tokens | ✅ | ✅ | Complete | - |
| Empty system prompt default | ✅ | ⚠️ | Bug Fix Needed | **High** |
| Plugin support | ✅ | ❌ | Missing | **Medium** |

## Implementation Plan

### Phase 1: Bug Fix - Empty System Prompt (Priority: HIGH)
**Estimated effort:** 30 minutes
**Files to modify:**
- `internal/transport/subprocess_cli.go` (buildCommandArgs method)

**Changes:**
Update system prompt handling to always pass `--system-prompt` flag, using empty string when nil.

---

### Phase 2: Feature - Plugin Support (Priority: MEDIUM)
**Estimated effort:** 2-3 hours
**Files to modify:**
- `types/options.go` (add PluginConfig type and fields)
- `internal/transport/subprocess_cli.go` (add plugin CLI args)
- `examples/with_plugins/main.go` (new example)

**Changes:**
1. Define PluginConfig type
2. Add Plugins field to ClaudeAgentOptions
3. Add builder methods (WithPlugins, WithPlugin)
4. Implement CLI argument generation
5. Add tests
6. Create example

---

## Other Findings

### Go SDK Unique Features
The Go SDK has some features not analyzed in Python SDK:
- Permission bypass options (DangerouslySkipPermissions)
- Fork session support
- Resume session support
- QueryWithContent for structured content

These appear to be feature-complete and don't need changes.

### Code Quality Notes
- Go SDK is well-structured with clear separation of concerns
- Python SDK uses more dynamic typing (TypedDict) while Go uses static structs
- Both SDKs follow similar architectural patterns
- Go SDK has better type safety overall

---

## Recommendations

1. **Immediate:** Fix empty system prompt bug (breaking change potential)
2. **Soon:** Add plugin support for feature parity
3. **Ongoing:** Monitor Python SDK commits weekly for new features
4. **Consider:** Set up automated comparison checks in CI

---

## Testing Strategy

### For Empty System Prompt Fix:
1. Test with nil SystemPrompt - should pass `--system-prompt ""`
2. Test with empty string - should pass `--system-prompt ""`
3. Test with actual prompt - should pass `--system-prompt "value"`
4. Test with preset - should work as before

### For Plugin Support:
1. Test with single plugin
2. Test with multiple plugins
3. Test with invalid plugin type (should error)
4. Test with non-existent path (CLI should handle error)
5. Integration test with actual plugin directory

---

## References

- Python SDK: `/Users/schlunsen/projects/claude-agent-sdk-python`
- Python commits analyzed: `git log --since="6 weeks ago" --oneline --no-merges`
- Go SDK current version: v0.2.2
- Python SDK current version: v0.1.6
