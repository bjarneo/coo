package model

import (
	"strings"
	"testing"
)

func TestInputInsertSanitizes(t *testing.T) {
	var i Input
	i.Insert("hello\x1b[31mworld\x1b[0m\x07")
	// ANSI escape sequences are dropped; BEL becomes a space (so it can't
	// truncate later sanitized content silently).
	if i.Value != "helloworld " {
		t.Errorf("Insert didn't sanitize ANSI/BEL correctly: got %q", i.Value)
	}
}

func TestInputAcceptsSpace(t *testing.T) {
	// Regression: a single space must round-trip through sanitize.Input;
	// otherwise typing space after "/msg" gets eaten and you can't reach
	// the username argument.
	var i Input
	i.Insert("/msg")
	i.Insert(" ")
	i.Insert("alice")
	if i.Value != "/msg alice" {
		t.Fatalf("space-after-slash-cmd was eaten: got %q", i.Value)
	}
}

func TestInputInsertCappedAtMaxLen(t *testing.T) {
	var i Input
	big := strings.Repeat("a", MaxInputLen+500)
	i.Insert(big)
	if len(i.Value) != MaxInputLen {
		t.Errorf("expected len %d, got %d", MaxInputLen, len(i.Value))
	}
}

func TestInputInsertTruncatesOnRuneBoundary(t *testing.T) {
	var i Input
	// Build a string that overshoots MaxInputLen by 2 bytes mid-rune.
	prefix := strings.Repeat("a", MaxInputLen-1)
	i.Insert(prefix + "日") // adds 3 bytes; only 1 byte fits
	if len(i.Value) != MaxInputLen-1 {
		t.Fatalf("expected len %d (rune-truncated), got %d (%q)",
			MaxInputLen-1, len(i.Value), i.Value)
	}
}

func TestInputBackspace(t *testing.T) {
	var i Input
	i.Insert("café")
	i.Backspace()
	if i.Value != "caf" {
		t.Errorf("expected 'caf' after backspace of multi-byte rune, got %q", i.Value)
	}
}
