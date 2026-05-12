package types

import (
	"sync"
	"time"
)

// SessionStoreFlushMode controls when transcript-mirror entries are flushed to a SessionStore.
type SessionStoreFlushMode string

const (
	// SessionStoreFlushBatched buffers entries and flushes once per turn or when
	// the pending buffer exceeds 500 entries / 1 MiB.
	SessionStoreFlushBatched SessionStoreFlushMode = "batched"

	// SessionStoreFlushEager triggers a background flush after every transcript_mirror
	// frame for near-real-time delivery.
	SessionStoreFlushEager SessionStoreFlushMode = "eager"
)

// SessionKey identifies a session transcript or subagent transcript in a store.
type SessionKey struct {
	// ProjectKey is a caller-defined scope. Default: sanitized cwd.
	ProjectKey string `json:"project_key"`

	// SessionID is the unique identifier for the session (UUID format).
	SessionID string `json:"session_id"`

	// Subpath is omitted for the main transcript; set for subagent files
	// (e.g., "subagents/agent-{id}").
	Subpath string `json:"subpath,omitempty"`
}

// SessionListSubkeysKey is the key argument to SessionStore.ListSubkeys (no subpath).
type SessionListSubkeysKey struct {
	ProjectKey string `json:"project_key"`
	SessionID  string `json:"session_id"`
}

// SessionStoreEntry represents one JSONL transcript line as observed by a SessionStore adapter.
// Additional fields are opaque JSON — adapters must pass them through.
type SessionStoreEntry map[string]interface{}

// SessionStoreListEntry is an entry returned by SessionStore.ListSessions.
type SessionStoreListEntry struct {
	// SessionID is the unique identifier for the session.
	SessionID string `json:"session_id"`

	// Mtime is the last-modified time in Unix epoch milliseconds.
	Mtime int64 `json:"mtime"`
}

// SessionSummaryEntry is an incrementally-maintained session summary.
type SessionSummaryEntry struct {
	// SessionID is the unique identifier for the session.
	SessionID string `json:"session_id"`

	// Mtime is the storage write time of the sidecar, in Unix epoch milliseconds.
	Mtime int64 `json:"mtime"`

	// Data is opaque SDK-owned summary state. Persist verbatim; do not interpret.
	Data map[string]interface{} `json:"data"`
}

// SDKSessionInfo contains session metadata returned by ListSessions.
type SDKSessionInfo struct {
	SessionID    string  `json:"session_id"`
	Summary      string  `json:"summary"`
	LastModified int64   `json:"last_modified"`
	FileSize     *int64  `json:"file_size,omitempty"`
	CustomTitle  *string `json:"custom_title,omitempty"`
	FirstPrompt  *string `json:"first_prompt,omitempty"`
	GitBranch    *string `json:"git_branch,omitempty"`
	CWD          *string `json:"cwd,omitempty"`
	Tag          *string `json:"tag,omitempty"`
	CreatedAt    *int64  `json:"created_at,omitempty"`
}

// SessionMessage represents a user or assistant message from a session transcript.
type SessionMessage struct {
	Type            string      `json:"type"` // "user" or "assistant"
	UUID            string      `json:"uuid"`
	SessionID       string      `json:"session_id"`
	Message         interface{} `json:"message"`
	ParentToolUseID *string     `json:"parent_tool_use_id,omitempty"`
}

// SessionStore is the interface for mirroring session transcripts to external storage.
// Implementations must be safe for concurrent use.
//
// Required methods:
//   - Append: Mirror a batch of transcript entries
//   - Load: Load a full session for resume
//
// Optional methods (may return nil/empty for simple implementations):
//   - ListSessions: List sessions for a project_key
//   - ListSessionSummaries: Return incrementally-maintained summaries
//   - Delete: Delete a session (cascades to subkeys)
//   - ListSubkeys: List all subpath keys under a session
type SessionStore interface {
	// Append mirrors a batch of transcript entries to the store.
	// The key identifies the session; entries are the JSONL lines to persist.
	// Implementations should be idempotent when entries contain UUID fields.
	Append(key SessionKey, entries []SessionStoreEntry) error

	// Load loads a full session for resume. Returns nil if the session doesn't exist.
	Load(key SessionKey) ([]SessionStoreEntry, error)

	// ListSessions lists sessions for a project_key.
	// Returns session IDs with their last-modified times.
	ListSessions(projectKey string) ([]SessionStoreListEntry, error)

	// ListSessionSummaries returns incrementally-maintained summaries for all sessions.
	ListSessionSummaries(projectKey string) ([]SessionSummaryEntry, error)

	// Delete deletes a session. Cascades to subkeys (subagent transcripts).
	Delete(key SessionKey) error

	// ListSubkeys lists all subpath keys under a session.
	ListSubkeys(key SessionListSubkeysKey) ([]string, error)
}

// InMemorySessionStore is an in-memory SessionStore implementation for testing and development.
// It implements all SessionStore methods and provides test helper methods.
//
// This is not suitable for production use — data is lost when the process exits.
type InMemorySessionStore struct {
	mu        sync.RWMutex
	store     map[string][]SessionStoreEntry // composite key -> entries
	mtimes    map[string]int64               // composite key -> mtime
	summaries map[string]SessionSummaryEntry // "project_key:session_id" -> summary
}

// NewInMemorySessionStore creates a new InMemorySessionStore.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		store:     make(map[string][]SessionStoreEntry),
		mtimes:    make(map[string]int64),
		summaries: make(map[string]SessionSummaryEntry),
	}
}

// compositeKey builds a storage key from a SessionKey.
func compositeKey(key SessionKey) string {
	k := key.ProjectKey + ":" + key.SessionID
	if key.Subpath != "" {
		k += ":" + key.Subpath
	}
	return k
}

// summaryKey builds a summary key from project_key and session_id.
func summaryKey(projectKey, sessionID string) string {
	return projectKey + ":" + sessionID
}

// Append mirrors a batch of transcript entries to the in-memory store.
func (s *InMemorySessionStore) Append(key SessionKey, entries []SessionStoreEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ck := compositeKey(key)
	s.store[ck] = append(s.store[ck], entries...)
	s.mtimes[ck] = time.Now().UnixMilli()
	return nil
}

// Load loads a full session from the in-memory store.
// Returns nil if the session doesn't exist.
func (s *InMemorySessionStore) Load(key SessionKey) ([]SessionStoreEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ck := compositeKey(key)
	entries, exists := s.store[ck]
	if !exists {
		return nil, nil
	}

	// Return a copy to prevent mutation
	result := make([]SessionStoreEntry, len(entries))
	copy(result, entries)
	return result, nil
}

// ListSessions lists sessions for a project_key.
func (s *InMemorySessionStore) ListSessions(projectKey string) ([]SessionStoreListEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := projectKey + ":"
	seen := make(map[string]bool)
	var result []SessionStoreListEntry

	for ck, mtime := range s.mtimes {
		if len(ck) <= len(prefix) {
			continue
		}
		if ck[:len(prefix)] != prefix {
			continue
		}

		// Extract session_id (second component)
		rest := ck[len(prefix):]
		sessionID := rest
		// If there's a subpath, only take the session_id part
		for i := 0; i < len(rest); i++ {
			if rest[i] == ':' {
				sessionID = rest[:i]
				break
			}
		}

		if seen[sessionID] {
			continue
		}
		seen[sessionID] = true

		result = append(result, SessionStoreListEntry{
			SessionID: sessionID,
			Mtime:     mtime,
		})
	}

	return result, nil
}

// ListSessionSummaries returns incrementally-maintained summaries for all sessions.
func (s *InMemorySessionStore) ListSessionSummaries(projectKey string) ([]SessionSummaryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := projectKey + ":"
	var result []SessionSummaryEntry

	for sk, summary := range s.summaries {
		if len(sk) >= len(prefix) && sk[:len(prefix)] == prefix {
			result = append(result, summary)
		}
	}

	return result, nil
}

// Delete deletes a session and all its subkeys from the in-memory store.
func (s *InMemorySessionStore) Delete(key SessionKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete exact key
	ck := compositeKey(key)
	delete(s.store, ck)
	delete(s.mtimes, ck)

	// Cascade: delete all subkeys
	prefix := key.ProjectKey + ":" + key.SessionID + ":"
	for k := range s.store {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.store, k)
			delete(s.mtimes, k)
		}
	}

	// Delete summary
	sk := summaryKey(key.ProjectKey, key.SessionID)
	delete(s.summaries, sk)

	return nil
}

// ListSubkeys lists all subpath keys under a session.
func (s *InMemorySessionStore) ListSubkeys(key SessionListSubkeysKey) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := key.ProjectKey + ":" + key.SessionID + ":"
	var result []string

	for ck := range s.store {
		if len(ck) > len(prefix) && ck[:len(prefix)] == prefix {
			subpath := ck[len(prefix):]
			result = append(result, subpath)
		}
	}

	return result, nil
}

// Size returns the number of stored sessions (main transcripts only).
// This is a test helper method.
func (s *InMemorySessionStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.store)
}

// Clear removes all stored data. This is a test helper method.
func (s *InMemorySessionStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = make(map[string][]SessionStoreEntry)
	s.mtimes = make(map[string]int64)
	s.summaries = make(map[string]SessionSummaryEntry)
}

// GetEntries returns all entries for a key. This is a test helper method.
func (s *InMemorySessionStore) GetEntries(key SessionKey) []SessionStoreEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ck := compositeKey(key)
	entries, exists := s.store[ck]
	if !exists {
		return nil
	}

	result := make([]SessionStoreEntry, len(entries))
	copy(result, entries)
	return result
}
