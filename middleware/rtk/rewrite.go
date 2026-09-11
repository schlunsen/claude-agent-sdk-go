package rtk

import (
	"maps"
	"path/filepath"
	"strings"
)

// wrapperPrograms are commands that delegate to another program and may
// appear before the real command we want to wrap with rtk.
var wrapperPrograms = map[string]struct{}{
	"sudo":    {},
	"env":     {},
	"time":    {},
	"nice":    {},
	"ionice":  {},
	"stdbuf":  {},
	"command": {},
	"exec":    {},
}

// rewrite walks a shell command line and prefixes each pipeline segment's
// leading program with the configured rtk binary when that program is in
// the allow-list (and not blocked).
//
// It is intentionally a lexical, best-effort rewriter: it understands
// simple top-level separators (&&, ||, ;, |) while tracking single and
// double quotes, backslash escapes, backtick command substitution, and
// parenthesis nesting (covering both subshells "(cmd)" and modern
// "$(cmd)" substitution). Operators that appear inside any of those
// contexts are not treated as segment separators, so pipelines embedded
// in command substitutions, subshells, or quoted strings are preserved
// intact.
//
// The rewriter still does not emulate a full shell parser: heredocs,
// process substitutions "<(...)" / ">(...)", and ANSI-C quoting "$'...'"
// are only partially understood. For those corner cases we err on the
// side of *not* rewriting rather than corrupting the command.
//
// rewrite returns the new command string and a bool indicating whether
// anything changed.
func (c *config) rewrite(cmd string) (string, bool) {
	if strings.TrimSpace(cmd) == "" {
		return cmd, false
	}

	segments, seps := splitSegments(cmd)
	changed := false

	for i, seg := range segments {
		newSeg, ok := c.rewriteSegment(seg)
		if ok {
			segments[i] = newSeg
			changed = true
		}
	}

	if !changed {
		return cmd, false
	}
	return joinSegments(segments, seps), true
}

// rewriteSegment rewrites a single pipeline segment (no top-level
// separators inside). It walks past env-var assignments, known wrappers
// (sudo / env / time / nice / ionice / stdbuf / command / exec) and
// their flag/value pairs until it lands on what looks like the real
// program, then — if that program is allow-listed — prefixes the
// configured rtk binary immediately before it.
func (c *config) rewriteSegment(seg string) (string, bool) {
	trimmed, leading, trailing := trimSurroundingSpace(seg)
	if trimmed == "" {
		return seg, false
	}

	tokens := tokenize(trimmed)
	if len(tokens) == 0 {
		return seg, false
	}
	// Already prefixed: do nothing.
	if basename(tokens[0]) == basename(c.binary) {
		return seg, false
	}

	// Find the real program. We walk through the token list honouring
	// three kinds of leading noise:
	//
	//   env-assignments   FOO=bar
	//   wrapper programs  sudo, env, time, nice, ionice, stdbuf, command, exec
	//   flags             tokens starting with "-", plus (heuristically) the
	//                     token that follows a flag when it is not itself a
	//                     flag, env-assignment, wrapper, or allow-listed
	//                     program — i.e. it looks like a flag value
	//                     (handles "sudo -u user", "nice -n 10", etc.)
	//
	// Any other token terminates the walk and is treated as the program.
	idx := 0
	justConsumedFlag := false
	sawWrapper := false

	for idx < len(tokens) {
		tok := tokens[idx]
		if isEnvAssignment(tok) {
			idx++
			justConsumedFlag = false
			continue
		}
		name := basename(tok)
		if _, isWrap := wrapperPrograms[name]; isWrap {
			idx++
			justConsumedFlag = false
			sawWrapper = true
			continue
		}
		if strings.HasPrefix(tok, "-") {
			idx++
			justConsumedFlag = true
			continue
		}
		// Heuristic: if the previous token was a flag belonging to a
		// wrapper (e.g. `sudo -u <user>`), the current token is most
		// likely that flag's value unless it's an allow-listed program
		// in its own right. Only apply this when we've actually seen a
		// wrapper — otherwise a bare `-x foo` would silently swallow
		// arbitrary arguments.
		if justConsumedFlag && sawWrapper {
			if _, isCmd := c.commands[name]; !isCmd {
				idx++
				justConsumedFlag = false
				continue
			}
		}
		break
	}
	if idx >= len(tokens) {
		return seg, false
	}

	prog := basename(tokens[idx])
	if prog == "" {
		return seg, false
	}
	if _, blocked := c.blocked[prog]; blocked {
		return seg, false
	}
	if _, ok := c.commands[prog]; !ok {
		return seg, false
	}

	before := strings.Join(tokens[:idx], " ")
	rest := strings.Join(tokens[idx:], " ")

	prefix := c.binary
	if c.ultraCompact {
		prefix += " -u"
	}

	var rebuilt string
	switch before {
	case "":
		rebuilt = prefix + " " + rest
	default:
		rebuilt = before + " " + prefix + " " + rest
	}
	return leading + rebuilt + trailing, true
}

// shellScan tracks quote, escape, backtick, and parenthesis state while
// iterating over a byte string. It is shared between splitSegments and
// tokenize so both observe the same notion of "inside a nested context,
// don't treat operators/whitespace specially".
type shellScan struct {
	inSingle, inDouble bool
	inBacktick         bool
	escape             bool
	parenDepth         int
}

// nested reports whether the scanner is currently inside quotes, a
// backtick command substitution, or any parenthesised group (including
// $(...) command substitutions and (subshells)).
func (s *shellScan) nested() bool {
	return s.inSingle || s.inDouble || s.inBacktick || s.parenDepth > 0 || s.escape
}

// step advances the scan state by one byte from src at position i and
// returns (consumeBytes, handled). When handled is true the caller
// should write nothing itself (the byte has been accounted for in
// state). When handled is false the caller may treat the byte as a
// candidate operator or ordinary content.
//
// The caller always writes the raw byte into its builder; this helper
// only maintains state.
func (s *shellScan) step(src string, i int) {
	ch := src[i]
	if s.escape {
		s.escape = false
		return
	}
	switch {
	case ch == '\\' && !s.inSingle:
		s.escape = true
	case ch == '\'' && !s.inDouble && !s.inBacktick:
		s.inSingle = !s.inSingle
	case ch == '"' && !s.inSingle && !s.inBacktick:
		s.inDouble = !s.inDouble
	case ch == '`' && !s.inSingle:
		s.inBacktick = !s.inBacktick
	case ch == '(' && !s.inSingle:
		// Both "$(...)" and "(...)" bump depth. We don't need to
		// distinguish them; the matching ')' will decrement.
		s.parenDepth++
	case ch == ')' && !s.inSingle && s.parenDepth > 0:
		s.parenDepth--
	}
}

// splitSegments splits a command line on top-level shell operators
// (&&, ||, ;, |) and returns the segments together with the separators
// that appeared between them (in order). Operators are ignored when the
// scanner is inside quotes, backticks, or any parenthesised group.
func splitSegments(cmd string) (segments []string, seps []string) {
	var cur strings.Builder
	var sc shellScan

	flush := func() {
		segments = append(segments, cur.String())
		cur.Reset()
	}

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		// Operators only matter when unnested and not currently
		// mid-escape. Note: step() flips `escape` -> false when it
		// processes the escaped byte, so we capture nesting *before*
		// calling step so the current byte itself still counts as
		// escaped.
		nested := sc.nested()
		sc.step(cmd, i)

		if !nested {
			if (ch == '&' || ch == '|') && i+1 < len(cmd) && cmd[i+1] == ch {
				flush()
				seps = append(seps, string([]byte{ch, ch}))
				// Advance state for the second operator byte, then skip.
				sc.step(cmd, i+1)
				i++
				continue
			}
			if ch == ';' {
				flush()
				seps = append(seps, ";")
				continue
			}
			if ch == '|' {
				flush()
				seps = append(seps, "|")
				continue
			}
		}
		cur.WriteByte(ch)
	}
	flush()
	return segments, seps
}

func joinSegments(segments, seps []string) string {
	var b strings.Builder
	for i, s := range segments {
		b.WriteString(s)
		if i < len(seps) {
			b.WriteString(seps[i])
		}
	}
	return b.String()
}

// tokenize produces a whitespace tokenization that honours quotes,
// escapes, backticks, and parenthesis nesting. Quote characters are
// kept in the returned tokens so the segment can be round-tripped.
// Whitespace inside $(...), (...) or `...` does NOT split tokens.
func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	var sc shellScan

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for i := 0; i < len(s); i++ {
		ch := s[i]
		nested := sc.nested()
		sc.step(s, i)

		if !nested && (ch == ' ' || ch == '\t') {
			flush()
			continue
		}
		cur.WriteByte(ch)
	}
	flush()
	return tokens
}

// trimSurroundingSpace splits s into (middle, leading, trailing) where
// leading+middle+trailing == s, middle has no outer spaces/tabs, and the
// spans are byte-exact so the segment can be reassembled without loss.
func trimSurroundingSpace(s string) (middle, leading, trailing string) {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end], s[:start], s[end:]
}

func basename(p string) string {
	if p == "" {
		return ""
	}
	// Strip any surrounding quotes left by tokenize.
	p = strings.Trim(p, "'\"")
	return filepath.Base(p)
}

// isEnvAssignment reports whether tok is a POSIX-shell-style variable
// assignment (NAME=VALUE). Identifier bytes must be ASCII [A-Za-z_][A-Za-z0-9_]*;
// any other byte — including all non-ASCII UTF-8 continuation bytes —
// fails the check, which is the behaviour we want.
func isEnvAssignment(tok string) bool {
	if tok == "" {
		return false
	}
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i := 0; i < eq; i++ {
		ch := tok[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
		case ch >= 'a' && ch <= 'z':
		case ch >= '0' && ch <= '9' && i > 0:
		case ch == '_':
		default:
			return false
		}
	}
	return true
}

// cloneInput is a tiny helper that exists so rtk.go can remain free of
// the `maps` import.
func cloneInput(m map[string]interface{}) map[string]interface{} {
	return maps.Clone(m)
}
