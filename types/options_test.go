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

// TestWithBetas tests the WithBetas builder method.
func TestWithBetas(t *testing.T) {
	t.Run("WithBetas sets multiple beta flags", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		betas := []string{"context-1m-2025-08-07"}

		result := opts.WithBetas(betas)

		// Verify the method returns the same instance for chaining
		if result != opts {
			t.Error("WithBetas should return the same instance for chaining")
		}

		// Verify the values are set correctly
		if len(opts.Betas) != 1 {
			t.Errorf("expected 1 beta, got %d", len(opts.Betas))
		}

		if opts.Betas[0] != "context-1m-2025-08-07" {
			t.Errorf("expected beta 'context-1m-2025-08-07', got %s", opts.Betas[0])
		}
	})

	t.Run("WithBetas empty list", func(t *testing.T) {
		opts := NewClaudeAgentOptions().WithBetas([]string{})

		if len(opts.Betas) != 0 {
			t.Errorf("expected 0 betas, got %d", len(opts.Betas))
		}
	})

	t.Run("WithBetas replaces existing betas", func(t *testing.T) {
		opts := NewClaudeAgentOptions().
			WithBeta("beta-1").
			WithBeta("beta-2").
			WithBetas([]string{"beta-3", "beta-4"})

		if len(opts.Betas) != 2 {
			t.Errorf("expected 2 betas after WithBetas, got %d", len(opts.Betas))
		}

		if opts.Betas[0] != "beta-3" || opts.Betas[1] != "beta-4" {
			t.Errorf("expected betas [beta-3, beta-4], got %v", opts.Betas)
		}
	})
}

// TestWithBeta tests the WithBeta builder method.
func TestWithBeta(t *testing.T) {
	t.Run("WithBeta adds single beta flag", func(t *testing.T) {
		opts := NewClaudeAgentOptions()

		result := opts.WithBeta("context-1m-2025-08-07")

		// Verify the method returns the same instance for chaining
		if result != opts {
			t.Error("WithBeta should return the same instance for chaining")
		}

		// Verify the value is set correctly
		if len(opts.Betas) != 1 {
			t.Errorf("expected 1 beta, got %d", len(opts.Betas))
		}

		if opts.Betas[0] != "context-1m-2025-08-07" {
			t.Errorf("expected beta 'context-1m-2025-08-07', got %s", opts.Betas[0])
		}
	})

	t.Run("WithBeta multiple calls accumulate", func(t *testing.T) {
		opts := NewClaudeAgentOptions().
			WithBeta("beta-1").
			WithBeta("beta-2").
			WithBeta("beta-3")

		if len(opts.Betas) != 3 {
			t.Errorf("expected 3 betas, got %d", len(opts.Betas))
		}

		expectedBetas := []string{"beta-1", "beta-2", "beta-3"}
		for i, beta := range opts.Betas {
			if beta != expectedBetas[i] {
				t.Errorf("beta[%d] = %s, expected %s", i, beta, expectedBetas[i])
			}
		}
	})

	t.Run("empty betas by default", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		if opts.Betas == nil {
			t.Error("Betas should not be nil")
		}
		if len(opts.Betas) != 0 {
			t.Errorf("expected 0 betas by default, got %d", len(opts.Betas))
		}
	})
}

// TestSubagentExecutionModeConstants tests the SubagentExecutionMode enum values.
func TestSubagentExecutionModeConstants(t *testing.T) {
	tests := []struct {
		mode     SubagentExecutionMode
		expected string
	}{
		{SubagentExecutionModeSequential, "sequential"},
		{SubagentExecutionModeParallel, "parallel"},
		{SubagentExecutionModeAuto, "auto"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("SubagentExecutionMode = %q, expected %q", string(tt.mode), tt.expected)
		}
	}
}

// TestMultiInvocationModeConstants tests the MultiInvocationMode enum values.
func TestMultiInvocationModeConstants(t *testing.T) {
	tests := []struct {
		mode     MultiInvocationMode
		expected string
	}{
		{MultiInvocationModeSequential, "sequential"},
		{MultiInvocationModeParallel, "parallel"},
		{MultiInvocationModeError, "error"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("MultiInvocationMode = %q, expected %q", string(tt.mode), tt.expected)
		}
	}
}

// TestSubagentErrorHandlingConstants tests the SubagentErrorHandling enum values.
func TestSubagentErrorHandlingConstants(t *testing.T) {
	tests := []struct {
		mode     SubagentErrorHandling
		expected string
	}{
		{SubagentErrorHandlingFailFast, "fail_fast"},
		{SubagentErrorHandlingContinue, "continue"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("SubagentErrorHandling = %q, expected %q", string(tt.mode), tt.expected)
		}
	}
}

// TestNewSubagentExecutionConfig tests the NewSubagentExecutionConfig constructor.
func TestNewSubagentExecutionConfig(t *testing.T) {
	t.Run("creates config with sensible defaults", func(t *testing.T) {
		config := NewSubagentExecutionConfig()

		if config.MultiInvocation != MultiInvocationModeSequential {
			t.Errorf("expected MultiInvocation to be sequential, got %s", config.MultiInvocation)
		}

		if config.MaxConcurrent != 3 {
			t.Errorf("expected MaxConcurrent to be 3, got %d", config.MaxConcurrent)
		}

		if config.ErrorHandling != SubagentErrorHandlingContinue {
			t.Errorf("expected ErrorHandling to be continue, got %s", config.ErrorHandling)
		}
	})

	t.Run("can be customized after creation", func(t *testing.T) {
		config := NewSubagentExecutionConfig()
		config.MultiInvocation = MultiInvocationModeParallel
		config.MaxConcurrent = 5
		config.ErrorHandling = SubagentErrorHandlingFailFast

		if config.MultiInvocation != MultiInvocationModeParallel {
			t.Errorf("expected MultiInvocation to be parallel, got %s", config.MultiInvocation)
		}

		if config.MaxConcurrent != 5 {
			t.Errorf("expected MaxConcurrent to be 5, got %d", config.MaxConcurrent)
		}

		if config.ErrorHandling != SubagentErrorHandlingFailFast {
			t.Errorf("expected ErrorHandling to be fail_fast, got %s", config.ErrorHandling)
		}
	})
}

// TestAgentDefinitionWithExecutionControl tests AgentDefinition with new execution control fields.
func TestAgentDefinitionWithExecutionControl(t *testing.T) {
	t.Run("agent with execution mode", func(t *testing.T) {
		mode := SubagentExecutionModeParallel
		agent := AgentDefinition{
			Description:   "Test agent",
			Prompt:        "Test prompt",
			ExecutionMode: &mode,
		}

		if agent.ExecutionMode == nil {
			t.Fatal("ExecutionMode should not be nil")
		}

		if *agent.ExecutionMode != SubagentExecutionModeParallel {
			t.Errorf("expected ExecutionMode to be parallel, got %s", *agent.ExecutionMode)
		}
	})

	t.Run("agent with timeout", func(t *testing.T) {
		timeout := 30.5
		agent := AgentDefinition{
			Description: "Test agent",
			Prompt:      "Test prompt",
			Timeout:     &timeout,
		}

		if agent.Timeout == nil {
			t.Fatal("Timeout should not be nil")
		}

		if *agent.Timeout != 30.5 {
			t.Errorf("expected Timeout to be 30.5, got %f", *agent.Timeout)
		}
	})

	t.Run("agent with max turns", func(t *testing.T) {
		maxTurns := 5
		agent := AgentDefinition{
			Description: "Test agent",
			Prompt:      "Test prompt",
			MaxTurns:    &maxTurns,
		}

		if agent.MaxTurns == nil {
			t.Fatal("MaxTurns should not be nil")
		}

		if *agent.MaxTurns != 5 {
			t.Errorf("expected MaxTurns to be 5, got %d", *agent.MaxTurns)
		}
	})

	t.Run("agent with all execution control fields", func(t *testing.T) {
		mode := SubagentExecutionModeSequential
		timeout := 60.0
		maxTurns := 10
		agent := AgentDefinition{
			Description:   "Full agent",
			Prompt:        "Full prompt",
			Tools:         []string{"Read", "Write"},
			ExecutionMode: &mode,
			Timeout:       &timeout,
			MaxTurns:      &maxTurns,
		}

		if agent.ExecutionMode == nil || *agent.ExecutionMode != SubagentExecutionModeSequential {
			t.Errorf("ExecutionMode mismatch")
		}

		if agent.Timeout == nil || *agent.Timeout != 60.0 {
			t.Errorf("Timeout mismatch")
		}

		if agent.MaxTurns == nil || *agent.MaxTurns != 10 {
			t.Errorf("MaxTurns mismatch")
		}
	})
}

// TestWithSubagentExecution tests the WithSubagentExecution builder method.
func TestWithSubagentExecution(t *testing.T) {
	t.Run("sets subagent execution config", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		config := NewSubagentExecutionConfig()
		config.MaxConcurrent = 5

		result := opts.WithSubagentExecution(config)

		// Verify the method returns the same instance for chaining
		if result != opts {
			t.Error("WithSubagentExecution should return the same instance for chaining")
		}

		// Verify the value is set
		if opts.SubagentExecution == nil {
			t.Fatal("SubagentExecution should not be nil after setting")
		}

		if opts.SubagentExecution.MaxConcurrent != 5 {
			t.Errorf("expected MaxConcurrent to be 5, got %d", opts.SubagentExecution.MaxConcurrent)
		}
	})

	t.Run("replaces existing config", func(t *testing.T) {
		opts := NewClaudeAgentOptions()

		config1 := NewSubagentExecutionConfig()
		config1.MaxConcurrent = 2
		opts.WithSubagentExecution(config1)

		config2 := NewSubagentExecutionConfig()
		config2.MaxConcurrent = 8
		opts.WithSubagentExecution(config2)

		if opts.SubagentExecution.MaxConcurrent != 8 {
			t.Errorf("expected MaxConcurrent to be 8 after replacement, got %d", opts.SubagentExecution.MaxConcurrent)
		}
	})

	t.Run("method chaining works", func(t *testing.T) {
		config := NewSubagentExecutionConfig()
		config.MultiInvocation = MultiInvocationModeParallel

		opts := NewClaudeAgentOptions().
			WithModel("claude-opus-4-5-latest").
			WithSubagentExecution(config).
			WithAgent("test", AgentDefinition{
				Description: "Test",
				Prompt:      "Test",
			})

		if opts.SubagentExecution == nil {
			t.Fatal("SubagentExecution should be set")
		}

		if opts.SubagentExecution.MultiInvocation != MultiInvocationModeParallel {
			t.Errorf("expected MultiInvocation to be parallel")
		}

		if opts.Model == nil || *opts.Model != "claude-opus-4-5-latest" {
			t.Errorf("Model should be set to claude-opus-4-5-latest")
		}

		if _, ok := opts.Agents["test"]; !ok {
			t.Errorf("Agent 'test' should be set")
		}
	})
}

// TestWithOutputFormat tests the WithOutputFormat builder method.
func TestWithOutputFormat(t *testing.T) {
	t.Run("sets output format with schema", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":  map[string]interface{}{"type": "string"},
				"count": map[string]interface{}{"type": "number"},
			},
			"required": []string{"name", "count"},
		}

		result := opts.WithOutputFormat(schema)

		// Verify the method returns the same instance for chaining
		if result != opts {
			t.Error("WithOutputFormat should return the same instance for chaining")
		}

		// Verify OutputFormat is set
		if opts.OutputFormat == nil {
			t.Fatal("OutputFormat should not be nil after setting")
		}

		// Verify Type is "json_schema"
		if opts.OutputFormat.Type != "json_schema" {
			t.Errorf("expected Type to be 'json_schema', got %s", opts.OutputFormat.Type)
		}

		// Verify Schema is set correctly
		if opts.OutputFormat.Schema == nil {
			t.Fatal("OutputFormat.Schema should not be nil")
		}

		schemaMap, ok := opts.OutputFormat.Schema.(map[string]interface{})
		if !ok {
			t.Fatal("OutputFormat.Schema should be a map[string]interface{}")
		}

		if schemaMap["type"] != "object" {
			t.Errorf("expected schema type 'object', got %v", schemaMap["type"])
		}
	})

	t.Run("method chaining works", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{"type": "string"},
			},
			"required": []string{"result"},
		}

		opts := NewClaudeAgentOptions().
			WithModel("claude-opus-4-5-latest").
			WithOutputFormat(schema).
			WithMaxTurns(10)

		if opts.OutputFormat == nil {
			t.Fatal("OutputFormat should be set")
		}

		if opts.OutputFormat.Type != "json_schema" {
			t.Errorf("expected Type to be 'json_schema'")
		}

		if opts.Model == nil || *opts.Model != "claude-opus-4-5-latest" {
			t.Errorf("Model should be set")
		}

		if opts.MaxTurns == nil || *opts.MaxTurns != 10 {
			t.Errorf("MaxTurns should be set")
		}
	})

	t.Run("replaces existing output format", func(t *testing.T) {
		opts := NewClaudeAgentOptions()

		schema1 := map[string]interface{}{"type": "string"}
		opts.WithOutputFormat(schema1)

		schema2 := map[string]interface{}{"type": "number"}
		opts.WithOutputFormat(schema2)

		if opts.OutputFormat.Schema != schema2 {
			t.Errorf("expected Schema to be replaced")
		}
	})

	t.Run("nil output format by default", func(t *testing.T) {
		opts := NewClaudeAgentOptions()
		if opts.OutputFormat != nil {
			t.Errorf("expected OutputFormat to be nil by default")
		}
	})
}

// TestNewOutputFormat tests the NewOutputFormat constructor.
func TestNewOutputFormat(t *testing.T) {
	t.Run("creates output format with json_schema type", func(t *testing.T) {
		schema := map[string]interface{}{"type": "string"}
		of := NewOutputFormat(schema)

		if of.Type != "json_schema" {
			t.Errorf("expected Type to be 'json_schema', got %s", of.Type)
		}

		if of.Schema != schema {
			t.Errorf("expected Schema to be the provided schema")
		}
	})

	t.Run("works with complex schemas", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":  map[string]interface{}{"type": "string"},
							"value": map[string]interface{}{"type": "number"},
						},
					},
				},
			},
		}

		of := NewOutputFormat(schema)

		if of.Type != "json_schema" {
			t.Error("Type should be json_schema")
		}

		if of.Schema == nil {
			t.Fatal("Schema should not be nil")
		}
	})
}
