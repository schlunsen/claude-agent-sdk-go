package types

import (
	"testing"
)

// TestWithMaxThinkingTokens tests the WithMaxThinkingTokens builder method.
func TestWithMaxThinkingTokens(t *testing.T) {
	opts := NewClaudeAgentOptions()

	// Test setting max thinking tokens
	result := opts.WithMaxThinkingTokens(5000)

	// Verify the method returns the same instance for chaining
	if result != opts {
		t.Error("WithMaxThinkingTokens should return the same instance for chaining")
	}

	// Verify the value is set correctly
	if opts.MaxThinkingTokens == nil {
		t.Fatal("MaxThinkingTokens should not be nil after setting")
	}

	if *opts.MaxThinkingTokens != 5000 {
		t.Errorf("Expected MaxThinkingTokens to be 5000, got %d", *opts.MaxThinkingTokens)
	}
}

// TestWithMaxBudgetUSD tests the WithMaxBudgetUSD builder method.
func TestWithMaxBudgetUSD(t *testing.T) {
	opts := NewClaudeAgentOptions()

	// Test setting max budget
	result := opts.WithMaxBudgetUSD(10.50)

	// Verify the method returns the same instance for chaining
	if result != opts {
		t.Error("WithMaxBudgetUSD should return the same instance for chaining")
	}

	// Verify the value is set correctly
	if opts.MaxBudgetUSD == nil {
		t.Fatal("MaxBudgetUSD should not be nil after setting")
	}

	if *opts.MaxBudgetUSD != 10.50 {
		t.Errorf("Expected MaxBudgetUSD to be 10.50, got %.2f", *opts.MaxBudgetUSD)
	}
}

// TestOptionsChaining tests that the builder methods can be chained together.
func TestOptionsChaining(t *testing.T) {
	opts := NewClaudeAgentOptions().
		WithMaxThinkingTokens(8000).
		WithMaxBudgetUSD(25.00).
		WithModel("claude-3-5-sonnet-20241022").
		WithMaxTurns(10)

	// Verify all values are set correctly
	if opts.MaxThinkingTokens == nil || *opts.MaxThinkingTokens != 8000 {
		t.Error("MaxThinkingTokens not set correctly in chain")
	}

	if opts.MaxBudgetUSD == nil || *opts.MaxBudgetUSD != 25.00 {
		t.Error("MaxBudgetUSD not set correctly in chain")
	}

	if opts.Model == nil || *opts.Model != "claude-3-5-sonnet-20241022" {
		t.Error("Model not set correctly in chain")
	}

	if opts.MaxTurns == nil || *opts.MaxTurns != 10 {
		t.Error("MaxTurns not set correctly in chain")
	}
}

// TestNewClaudeAgentOptions tests that the constructor creates a valid instance.
func TestNewClaudeAgentOptions(t *testing.T) {
	opts := NewClaudeAgentOptions()

	// Verify default values
	if opts == nil {
		t.Fatal("NewClaudeAgentOptions should not return nil")
	}

	// Check that optional fields are nil by default
	if opts.MaxThinkingTokens != nil {
		t.Error("MaxThinkingTokens should be nil by default")
	}

	if opts.MaxBudgetUSD != nil {
		t.Error("MaxBudgetUSD should be nil by default")
	}

	// Check that maps are initialized
	if opts.Env == nil {
		t.Error("Env should be initialized")
	}

	if opts.ExtraArgs == nil {
		t.Error("ExtraArgs should be initialized")
	}
}

// TestWithMaxThinkingTokensZeroValue tests that zero values can be set.
func TestWithMaxThinkingTokensZeroValue(t *testing.T) {
	opts := NewClaudeAgentOptions().WithMaxThinkingTokens(0)

	if opts.MaxThinkingTokens == nil {
		t.Fatal("MaxThinkingTokens should not be nil")
	}

	if *opts.MaxThinkingTokens != 0 {
		t.Errorf("Expected MaxThinkingTokens to be 0, got %d", *opts.MaxThinkingTokens)
	}
}

// TestWithMaxBudgetUSDZeroValue tests that zero budget can be set.
func TestWithMaxBudgetUSDZeroValue(t *testing.T) {
	opts := NewClaudeAgentOptions().WithMaxBudgetUSD(0.0)

	if opts.MaxBudgetUSD == nil {
		t.Fatal("MaxBudgetUSD should not be nil")
	}

	if *opts.MaxBudgetUSD != 0.0 {
		t.Errorf("Expected MaxBudgetUSD to be 0.0, got %.2f", *opts.MaxBudgetUSD)
	}
}

// TestPluginConfig tests PluginConfig type and validation.
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
		if plugin.Path != "/path/to/plugin" {
			t.Errorf("expected Path '/path/to/plugin', got %s", plugin.Path)
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

// TestClaudeAgentOptions_Plugins tests plugin builder methods.
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
		if opts.Plugins[0].Path != "/path/to/plugin" {
			t.Errorf("expected Path '/path/to/plugin', got %s", opts.Plugins[0].Path)
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

	t.Run("multiple plugins via WithLocalPlugin chaining", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		opts.WithLocalPlugin("/path/1").
			WithLocalPlugin("/path/2").
			WithLocalPlugin("/path/3")

		if len(opts.Plugins) != 3 {
			t.Errorf("expected 3 plugins, got %d", len(opts.Plugins))
		}

		// Verify paths
		expectedPaths := []string{"/path/1", "/path/2", "/path/3"}
		for i, plugin := range opts.Plugins {
			if plugin.Path != expectedPaths[i] {
				t.Errorf("plugin[%d].Path = %s, want %s", i, plugin.Path, expectedPaths[i])
			}
		}
	})

	t.Run("empty plugins by default", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		if opts.Plugins == nil {
			t.Error("Plugins should not be nil")
		}
		if len(opts.Plugins) != 0 {
			t.Errorf("expected 0 plugins by default, got %d", len(opts.Plugins))
		}
	})
}
