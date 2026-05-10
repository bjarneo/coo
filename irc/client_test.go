package irc

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMessage(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		max   int
		wantN int
		joins bool // true if rejoining with a space yields the original
	}{
		{"short fits", "hello world", 100, 1, true},
		{"exact boundary", strings.Repeat("a", 100), 100, 1, false},
		{"two-line word boundary", "hello world " + strings.Repeat("x", 90), 50, 3, false},
		{"long single word forces hard cut", strings.Repeat("a", 250), 100, 3, false},
		{"empty", "", 100, 1, true},
		{"max zero defaults to single chunk", "anything", 0, 1, true},
		{"trim leading spaces between chunks", "abc " + strings.Repeat("x", 50) + "  defghij", 30, 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitMessage(tc.text, tc.max)
			if len(got) != tc.wantN {
				t.Fatalf("splitMessage(%q, %d) returned %d chunks, want %d: %q",
					tc.text, tc.max, len(got), tc.wantN, got)
			}
			for _, c := range got {
				if tc.max > 0 && len(c) > tc.max {
					t.Fatalf("chunk %q exceeds max %d", c, tc.max)
				}
			}
			if tc.joins {
				if joined := strings.Join(got, " "); joined != tc.text {
					t.Fatalf("rejoin mismatch: got %q want %q", joined, tc.text)
				}
			}
		})
	}
}

func TestIsValidChannelName(t *testing.T) {
	cases := map[string]bool{
		"#archlinux":    true,
		"#a":            true,
		"&local":        true,
		"+modeless":     true,
		"!safe":         true,
		"":              false,
		"#":             false,
		"archlinux":     false, // missing prefix
		"#with space":   false,
		"#with,comma":   false,
		"#bell\x07":     false,
		"#new\nline":    false,
		strings.Repeat("#", 51): false, // too long
	}
	for in, want := range cases {
		if got := IsValidChannelName(in); got != want {
			t.Errorf("IsValidChannelName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSplitMessageUTF8Safe(t *testing.T) {
	// "日本語" is 9 bytes (3 runes × 3 bytes). With max=4 we must cut on a
	// rune boundary so each chunk is independently valid UTF-8.
	for _, c := range splitMessage("日本語", 4) {
		if !utf8.ValidString(c) {
			t.Errorf("chunk %q is not valid UTF-8", c)
		}
	}
}

func TestStripParenthetical(t *testing.T) {
	cases := map[string]string{
		"Connection reset by peer (10.0.0.5)": "Connection reset by peer",
		"Closing link (Excess flood)":         "Closing link",
		"plain quit":                          "plain quit",
		"((nested) parens)":                   "",
		"":                                    "",
	}
	for in, want := range cases {
		if got := stripParenthetical(in); got != want {
			t.Errorf("stripParenthetical(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsValidNick(t *testing.T) {
	cases := map[string]bool{
		"alice":      true,
		"bob_42":     true,
		"":           false,
		"with space": false,
		"a,b":        false,
		"hash#yes":   false,
		"colon:":     false,
	}
	for in, want := range cases {
		if got := IsValidNick(in); got != want {
			t.Errorf("IsValidNick(%q) = %v, want %v", in, got, want)
		}
	}
}
