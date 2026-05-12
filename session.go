package claude

import (
	"fmt"
	"sort"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// ForkSessionResult contains the result of forking a session.
type ForkSessionResult struct {
	// SessionID is the new session's unique identifier.
	SessionID string `json:"session_id"`
}

// ListSessions lists sessions from a SessionStore for a given project key.
// Results are sorted by last-modified time (newest first).
// Use limit and offset for pagination (0 for limit means no limit).
func ListSessions(store types.SessionStore, projectKey string, limit, offset int) ([]types.SDKSessionInfo, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	entries, err := store.ListSessions(projectKey)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	// Sort by mtime descending (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Mtime > entries[j].Mtime
	})

	// Apply offset
	if offset > 0 {
		if offset >= len(entries) {
			return []types.SDKSessionInfo{}, nil
		}
		entries = entries[offset:]
	}

	// Apply limit
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}

	// Enrich with summary data if available
	summaries, _ := store.ListSessionSummaries(projectKey)
	summaryMap := make(map[string]types.SessionSummaryEntry, len(summaries))
	for _, s := range summaries {
		summaryMap[s.SessionID] = s
	}

	result := make([]types.SDKSessionInfo, 0, len(entries))
	for _, e := range entries {
		info := types.SDKSessionInfo{
			SessionID:    e.SessionID,
			LastModified: e.Mtime,
		}

		// Try to enrich from summary
		if summary, ok := summaryMap[e.SessionID]; ok {
			if s, ok := summary.Data["summary"].(string); ok {
				info.Summary = s
			}
			if t, ok := summary.Data["custom_title"].(string); ok {
				info.CustomTitle = &t
			}
		}

		result = append(result, info)
	}

	return result, nil
}

// GetSessionInfo retrieves metadata for a single session from a SessionStore.
// Returns nil if the session doesn't exist.
func GetSessionInfo(store types.SessionStore, projectKey, sessionID string) (*types.SDKSessionInfo, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	entries, err := store.ListSessions(projectKey)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	for _, e := range entries {
		if e.SessionID == sessionID {
			info := &types.SDKSessionInfo{
				SessionID:    e.SessionID,
				LastModified: e.Mtime,
			}

			// Try to enrich from summary
			summaries, _ := store.ListSessionSummaries(projectKey)
			for _, s := range summaries {
				if s.SessionID == sessionID {
					if str, ok := s.Data["summary"].(string); ok {
						info.Summary = str
					}
					if t, ok := s.Data["custom_title"].(string); ok {
						info.CustomTitle = &t
					}
					break
				}
			}

			return info, nil
		}
	}

	return nil, nil
}

// GetSessionMessages retrieves the conversation messages from a session transcript.
// Messages are returned in chronological order.
// Use limit and offset for pagination (0 for limit means no limit).
func GetSessionMessages(store types.SessionStore, projectKey, sessionID string, limit, offset int) ([]types.SessionMessage, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	key := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}

	rawEntries, err := store.Load(key)
	if err != nil {
		return nil, fmt.Errorf("loading session: %w", err)
	}
	if rawEntries == nil {
		return nil, nil
	}

	var messages []types.SessionMessage
	for _, entry := range rawEntries {
		msg, ok := parseSessionMessage(entry, sessionID)
		if ok {
			messages = append(messages, msg)
		}
	}

	// Apply offset
	if offset > 0 {
		if offset >= len(messages) {
			return []types.SessionMessage{}, nil
		}
		messages = messages[offset:]
	}

	// Apply limit
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}

	return messages, nil
}

// ListSubagents lists all subagent IDs for a session.
func ListSubagents(store types.SessionStore, projectKey, sessionID string) ([]string, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	subkeys, err := store.ListSubkeys(types.SessionListSubkeysKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("listing subkeys: %w", err)
	}

	return subkeys, nil
}

// GetSubagentMessages retrieves conversation messages from a subagent transcript.
// The agentID corresponds to the subpath key (e.g., "subagents/agent-{id}").
func GetSubagentMessages(store types.SessionStore, projectKey, sessionID, agentID string, limit, offset int) ([]types.SessionMessage, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	key := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
		Subpath:    agentID,
	}

	rawEntries, err := store.Load(key)
	if err != nil {
		return nil, fmt.Errorf("loading subagent session: %w", err)
	}
	if rawEntries == nil {
		return nil, nil
	}

	var messages []types.SessionMessage
	for _, entry := range rawEntries {
		msg, ok := parseSessionMessage(entry, sessionID)
		if ok {
			messages = append(messages, msg)
		}
	}

	// Apply offset
	if offset > 0 {
		if offset >= len(messages) {
			return []types.SessionMessage{}, nil
		}
		messages = messages[offset:]
	}

	// Apply limit
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}

	return messages, nil
}

// RenameSession sets a custom title for a session by appending a title entry.
func RenameSession(store types.SessionStore, projectKey, sessionID, title string) error {
	if store == nil {
		return fmt.Errorf("session store is nil")
	}

	entry := types.SessionStoreEntry{
		"type":         "custom_title",
		"custom_title": title,
		"session_id":   sessionID,
	}

	key := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}

	return store.Append(key, []types.SessionStoreEntry{entry})
}

// TagSession sets a tag on a session by appending a tag entry.
// Pass an empty string to clear the tag.
func TagSession(store types.SessionStore, projectKey, sessionID, tag string) error {
	if store == nil {
		return fmt.Errorf("session store is nil")
	}

	entry := types.SessionStoreEntry{
		"type":       "tag",
		"tag":        tag,
		"session_id": sessionID,
	}

	key := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}

	return store.Append(key, []types.SessionStoreEntry{entry})
}

// DeleteSession deletes a session and all its subagent transcripts.
func DeleteSession(store types.SessionStore, projectKey, sessionID string) error {
	if store == nil {
		return fmt.Errorf("session store is nil")
	}

	key := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}

	return store.Delete(key)
}

// ForkSession creates a new session by copying entries from an existing session.
// If upToMessageID is non-empty, only entries up to and including that message are copied.
// If title is non-empty, a custom title entry is appended to the new session.
func ForkSession(store types.SessionStore, projectKey, sessionID, newSessionID string, upToMessageID, title string) (*ForkSessionResult, error) {
	if store == nil {
		return nil, fmt.Errorf("session store is nil")
	}

	srcKey := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}

	entries, err := store.Load(srcKey)
	if err != nil {
		return nil, fmt.Errorf("loading source session: %w", err)
	}
	if entries == nil {
		return nil, fmt.Errorf("source session not found: %s", sessionID)
	}

	// Filter entries if upToMessageID is specified
	if upToMessageID != "" {
		var filtered []types.SessionStoreEntry
		for _, entry := range entries {
			filtered = append(filtered, entry)
			if uuid, ok := entry["uuid"].(string); ok && uuid == upToMessageID {
				break
			}
		}
		entries = filtered
	}

	// Write to new session
	dstKey := types.SessionKey{
		ProjectKey: projectKey,
		SessionID:  newSessionID,
	}

	if err := store.Append(dstKey, entries); err != nil {
		return nil, fmt.Errorf("writing forked session: %w", err)
	}

	// Add custom title if specified
	if title != "" {
		titleEntry := types.SessionStoreEntry{
			"type":         "custom_title",
			"custom_title": title,
			"session_id":   newSessionID,
		}
		if err := store.Append(dstKey, []types.SessionStoreEntry{titleEntry}); err != nil {
			return nil, fmt.Errorf("writing fork title: %w", err)
		}
	}

	return &ForkSessionResult{SessionID: newSessionID}, nil
}

// ProjectKeyForDirectory returns a sanitized project key for a given directory path.
// This is used to derive the default project key when none is specified.
func ProjectKeyForDirectory(dir string) string {
	if dir == "" {
		return "default"
	}
	// Simple sanitization: replace path separators and special chars
	result := make([]byte, 0, len(dir))
	for i := 0; i < len(dir); i++ {
		c := dir[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	return string(result)
}

// parseSessionMessage extracts a SessionMessage from a raw store entry.
// Returns the message and true if the entry is a user or assistant message.
func parseSessionMessage(entry types.SessionStoreEntry, sessionID string) (types.SessionMessage, bool) {
	msgType, _ := entry["type"].(string)
	if msgType != "user" && msgType != "assistant" {
		return types.SessionMessage{}, false
	}

	msg := types.SessionMessage{
		Type:      msgType,
		SessionID: sessionID,
		Message:   entry["message"],
	}

	if uuid, ok := entry["uuid"].(string); ok {
		msg.UUID = uuid
	}

	if parentID, ok := entry["parent_tool_use_id"].(string); ok {
		msg.ParentToolUseID = &parentID
	}

	return msg, true
}
