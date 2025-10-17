#!/bin/bash
# User Prompt Logger Hook for Claude Code
#
# This hook captures user prompts submitted to Claude Code and stores them
# in the CCT analytics database for tracking and analysis.
#
# Hook Type: UserPromptSubmit
# Input: JSON on stdin with session_id, cwd, prompt
# Output: Silent (no stdout/stderr unless error)

set -euo pipefail

# Read JSON from stdin
INPUT=$(cat)

# Parse JSON fields using jq if available, otherwise use grep/sed
if command -v jq &> /dev/null; then
    SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty')
    CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
else
    # Fallback to basic parsing (less robust)
    SESSION_ID=$(echo "$INPUT" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
    PROMPT=$(echo "$INPUT" | grep -o '"prompt":"[^"]*"' | cut -d'"' -f4)
    CWD=$(echo "$INPUT" | grep -o '"cwd":"[^"]*"' | cut -d'"' -f4)
fi

# Validate required fields
if [[ -z "$SESSION_ID" ]] || [[ -z "$PROMPT" ]] || [[ -z "$CWD" ]]; then
    # Silent failure - don't block Claude Code
    exit 0
fi

# Get git branch from working directory
GIT_BRANCH=""
if [[ -d "$CWD/.git" ]]; then
    GIT_BRANCH=$(cd "$CWD" && git branch --show-current 2>/dev/null || echo "")
fi

# Generate friendly session name from session_id
# Use a list of 25 South Park character names and hash the session_id to pick one
SESSION_NAMES=(
    "Cartman"
    "Stan"
    "Kyle"
    "Kenny"
    "Butters"
    "Randy"
    "Tweek"
    "Craig"
    "Token"
    "Wendy"
    "Sheila"
    "Sharon"
    "Chef"
    "Mr-Garrison"
    "Mr-Mackey"
    "Jimmy"
    "Timmy"
    "Bebe"
    "Clyde"
    "Ike"
    "PC-Principal"
    "Towelie"
    "Mr-Hankey"
    "Big-Gay-Al"
    "Satan"
)

# Generate a numeric hash from session_id to pick a name (modulo 25)
# Use first 8 characters of session_id to get a stable hash
if command -v cksum &> /dev/null; then
    HASH=$(echo -n "$SESSION_ID" | cksum | cut -d' ' -f1)
    INDEX=$((HASH % 25))
else
    # Fallback: use character values
    HASH=0
    for ((i=0; i<${#SESSION_ID} && i<8; i++)); do
        CHAR="${SESSION_ID:$i:1}"
        ASCII=$(printf '%d' "'$CHAR")
        HASH=$((HASH + ASCII))
    done
    INDEX=$((HASH % 25))
fi

SESSION_NAME="${SESSION_NAMES[$INDEX]}"

# Extract model information from environment variables
MODEL_NAME="${ANTHROPIC_MODEL:-}"
MODEL_PROVIDER="${ANTHROPIC_BASE_URL:-https://api.anthropic.com}"

# Analytics server endpoint (default port, HTTPS by default)
# Use CCT_ANALYTICS_URL if set, otherwise default to https://localhost:3333
ANALYTICS_URL="${CCT_ANALYTICS_URL:-https://localhost:3333/api/prompts}"

# Read API key from .secret file if it exists
# Default to ~/.claude/analytics/.secret
API_KEY_FILE="${CCT_API_KEY_FILE:-$HOME/.claude/analytics/.secret}"
API_KEY=""
if [[ -f "$API_KEY_FILE" ]]; then
    API_KEY=$(cat "$API_KEY_FILE")
fi

# Build JSON payload
if command -v jq &> /dev/null; then
    # Use jq for proper JSON encoding
    PAYLOAD=$(jq -n \
        --arg session "$SESSION_ID" \
        --arg sessionName "$SESSION_NAME" \
        --arg prompt "$PROMPT" \
        --arg cwd "$CWD" \
        --arg branch "$GIT_BRANCH" \
        --arg modelProvider "$MODEL_PROVIDER" \
        --arg modelName "$MODEL_NAME" \
        '{session_id: $session, session_name: $sessionName, prompt: $prompt, cwd: $cwd, branch: $branch, model_provider: $modelProvider, model_name: $modelName}')
else
    # Fallback: basic JSON (less robust, but works for most cases)
    # Note: This doesn't escape special characters properly
    PAYLOAD=$(cat <<EOF
{
  "session_id": "$SESSION_ID",
  "session_name": "$SESSION_NAME",
  "prompt": "$PROMPT",
  "cwd": "$CWD",
  "branch": "$GIT_BRANCH",
  "model_provider": "$MODEL_PROVIDER",
  "model_name": "$MODEL_NAME"
}
EOF
)
fi

# POST to analytics server (run in background to not slow down Claude Code)
# Use curl if available, otherwise try wget
if command -v curl &> /dev/null; then
    # Build curl command with authentication if API key exists
    if [[ -n "$API_KEY" ]]; then
        curl -X POST "$ANALYTICS_URL" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $API_KEY" \
            -k \
            -d "$PAYLOAD" \
            &> /dev/null &
    else
        # No API key - try without authentication (will fail if auth is enabled)
        curl -X POST "$ANALYTICS_URL" \
            -H "Content-Type: application/json" \
            -k \
            -d "$PAYLOAD" \
            &> /dev/null &
    fi
elif command -v wget &> /dev/null; then
    # Build wget command with authentication if API key exists
    if [[ -n "$API_KEY" ]]; then
        wget --quiet --post-data="$PAYLOAD" \
            --header="Content-Type: application/json" \
            --header="Authorization: Bearer $API_KEY" \
            --no-check-certificate \
            -O /dev/null \
            "$ANALYTICS_URL" \
            &> /dev/null &
    else
        # No API key - try without authentication
        wget --quiet --post-data="$PAYLOAD" \
            --header="Content-Type: application/json" \
            --no-check-certificate \
            -O /dev/null \
            "$ANALYTICS_URL" \
            &> /dev/null &
    fi
fi

# Exit successfully (don't block Claude Code)
exit 0
