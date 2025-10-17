package internal

// Test fixtures for message parser testing.
// These represent realistic JSON messages from the Claude Code CLI.

var (
	// User messages
	userMessageSimple = []byte(`{
		"type": "user",
		"content": "Hello Claude"
	}`)

	userMessageComplex = []byte(`{
		"type": "user",
		"content": [
			{
				"type": "text",
				"text": "Here is the tool result:"
			},
			{
				"type": "tool_result",
				"tool_use_id": "toolu_123",
				"content": "Operation completed successfully"
			}
		],
		"parent_tool_use_id": "parent_456"
	}`)

	//nolint:unused
	userMessageWithToolResult = []byte(`{
		"type": "user",
		"content": [
			{
				"type": "tool_result",
				"tool_use_id": "toolu_789",
				"content": "42",
				"is_error": false
			}
		]
	}`)

	// Assistant messages
	assistantMessageText = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Hi there! How can I help you today?"
			}
		],
		"model": "claude-sonnet-4-5-20250929"
	}`)

	assistantMessageToolUse = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "text",
				"text": "I'll calculate that for you."
			},
			{
				"type": "tool_use",
				"id": "toolu_calculator_1",
				"name": "calculator",
				"input": {
					"expression": "2 + 2"
				}
			}
		],
		"model": "claude-sonnet-4-5-20250929"
	}`)

	assistantMessageThinking = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "thinking",
				"thinking": "Let me analyze this problem step by step...",
				"signature": "sig_abc123"
			},
			{
				"type": "text",
				"text": "Based on my analysis..."
			}
		],
		"model": "claude-sonnet-4-5-20250929",
		"parent_tool_use_id": "parent_tool_xyz"
	}`)

	assistantMessageMixed = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "thinking",
				"thinking": "I need to use the bash tool",
				"signature": "sig_def456"
			},
			{
				"type": "text",
				"text": "Running command..."
			},
			{
				"type": "tool_use",
				"id": "toolu_bash_1",
				"name": "bash",
				"input": {
					"command": "ls -la"
				}
			}
		],
		"model": "claude-sonnet-4-5-20250929"
	}`)

	// System messages
	systemMessageMetadata = []byte(`{
		"type": "system",
		"subtype": "metadata",
		"data": {
			"session_id": "sess_abc123",
			"version": "1.0.0"
		}
	}`)

	systemMessageWarning = []byte(`{
		"type": "system",
		"subtype": "warning",
		"data": {
			"message": "API rate limit approaching",
			"current_usage": 80,
			"limit": 100
		}
	}`)

	// Result messages
	resultMessageSuccess = []byte(`{
		"type": "result",
		"subtype": "success",
		"duration_ms": 1234,
		"duration_api_ms": 987,
		"is_error": false,
		"num_turns": 3,
		"session_id": "sess_result_123",
		"total_cost_usd": 0.0045,
		"usage": {
			"input_tokens": 150,
			"output_tokens": 75,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens": 0
		},
		"result": "Task completed successfully"
	}`)

	resultMessageError = []byte(`{
		"type": "result",
		"subtype": "error",
		"duration_ms": 567,
		"duration_api_ms": 234,
		"is_error": true,
		"num_turns": 1,
		"session_id": "sess_error_456",
		"result": "An error occurred during processing"
	}`)

	// Stream events
	streamEventMessageStart = []byte(`{
		"type": "stream_event",
		"uuid": "evt_uuid_123",
		"session_id": "sess_stream_789",
		"event": {
			"type": "message_start",
			"message": {
				"id": "msg_abc",
				"type": "message",
				"role": "assistant",
				"content": [],
				"model": "claude-sonnet-4-5-20250929"
			}
		}
	}`)

	streamEventContentBlockDelta = []byte(`{
		"type": "stream_event",
		"uuid": "evt_uuid_456",
		"session_id": "sess_stream_789",
		"event": {
			"type": "content_block_delta",
			"index": 0,
			"delta": {
				"type": "text_delta",
				"text": "Hello"
			}
		},
		"parent_tool_use_id": "parent_stream_xyz"
	}`)

	streamEventMessageDelta = []byte(`{
		"type": "stream_event",
		"uuid": "evt_uuid_789",
		"session_id": "sess_stream_789",
		"event": {
			"type": "message_delta",
			"delta": {
				"stop_reason": "end_turn",
				"stop_sequence": null
			},
			"usage": {
				"output_tokens": 42
			}
		}
	}`)

	// Individual content blocks
	textBlockJSON = []byte(`{
		"type": "text",
		"text": "This is a text block"
	}`)

	thinkingBlockJSON = []byte(`{
		"type": "thinking",
		"thinking": "Let me think about this...",
		"signature": "sig_thinking_123"
	}`)

	toolUseBlockJSON = []byte(`{
		"type": "tool_use",
		"id": "toolu_block_123",
		"name": "calculator",
		"input": {
			"operation": "add",
			"a": 10,
			"b": 20
		}
	}`)

	toolResultBlockJSON = []byte(`{
		"type": "tool_result",
		"tool_use_id": "toolu_result_456",
		"content": "The result is 30"
	}`)

	toolResultBlockJSONWithError = []byte(`{
		"type": "tool_result",
		"tool_use_id": "toolu_error_789",
		"content": "Command failed with exit code 1",
		"is_error": true
	}`)

	toolResultBlockJSONComplex = []byte(`{
		"type": "tool_result",
		"tool_use_id": "toolu_complex_999",
		"content": [
			{
				"type": "text",
				"text": "Multi-part result"
			}
		],
		"is_error": false
	}`)

	// Invalid/malformed messages for error testing
	invalidJSONMalformed = []byte(`{
		"type": "user",
		"content": "Missing closing brace"`)

	invalidJSONMissingType = []byte(`{
		"content": "No type field"
	}`)

	invalidJSONUnknownType = []byte(`{
		"type": "unknown_message_type",
		"data": {}
	}`)

	invalidJSONEmptyBytes = []byte(``)

	invalidJSONNullType = []byte(`{
		"type": null,
		"content": "Type is null"
	}`)

	invalidJSONNumberType = []byte(`{
		"type": 123,
		"content": "Type is a number"
	}`)

	// Messages with extra fields (forward compatibility)
	userMessageExtraFields = []byte(`{
		"type": "user",
		"content": "Hello",
		"future_field": "should be ignored",
		"another_unknown_field": {
			"nested": "data"
		}
	}`)

	assistantMessageExtraFields = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Response",
				"extra_block_field": "ignored"
			}
		],
		"model": "claude-sonnet-4-5-20250929",
		"new_api_feature": true
	}`)

	// Content blocks with errors
	contentBlockMissingType = []byte(`{
		"text": "No type field"
	}`)

	contentBlockUnknownType = []byte(`{
		"type": "unknown_block_type",
		"data": "something"
	}`)

	contentBlockMalformed = []byte(`{
		"type": "text",
		"text": "Unclosed`)

	// Multiple content blocks for batch parsing
	multipleContentBlocks = []string{
		`{"type": "text", "text": "First block"}`,
		`{"type": "thinking", "thinking": "Second block", "signature": "sig_2"}`,
		`{"type": "tool_use", "id": "tool_3", "name": "test", "input": {}}`,
	}

	//nolint:unused
	emptyContentBlocks = []string{}

	// User message variants for comprehensive testing
	userMessageOnlyText = []byte(`{
		"type": "user",
		"content": "Simple text content"
	}`)

	userMessageContentBlocks = []byte(`{
		"type": "user",
		"content": [
			{
				"type": "text",
				"text": "Part 1"
			},
			{
				"type": "text",
				"text": "Part 2"
			}
		]
	}`)

	// Assistant message with all block types
	assistantMessageAllBlocks = []byte(`{
		"type": "assistant",
		"content": [
			{
				"type": "thinking",
				"thinking": "Analyzing request",
				"signature": "sig_all"
			},
			{
				"type": "text",
				"text": "I'll help with that"
			},
			{
				"type": "tool_use",
				"id": "toolu_all_1",
				"name": "bash",
				"input": {"command": "echo test"}
			}
		],
		"model": "claude-sonnet-4-5-20250929"
	}`)
)
