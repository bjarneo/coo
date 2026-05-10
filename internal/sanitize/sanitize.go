// Package sanitize cleans untrusted text coming off the wire so it can be
// rendered safely in a TUI. IRC servers can deliver ANSI escape sequences,
// mIRC color codes, BELLs, and bare control bytes that would corrupt the
// terminal or impersonate UI chrome.
package sanitize

import (
	"strings"
	"unicode/utf8"
)

// Text returns s with the following stripped or replaced:
//   - bare control bytes (0x00–0x1F, 0x7F) except space → replaced with space
//   - tab (0x09) → replaced with space (tabs misalign columnar UI)
//   - ANSI CSI sequences `ESC [ … final-byte` (final in 0x40–0x7E)
//   - ANSI OSC sequences `ESC ] … BEL` or `ESC ] … ESC \`
//   - mIRC color codes: 0x03 followed by up to 2 digits, optional ',' + 2 digits
//   - mIRC formatting codes: 0x02 (bold) 0x1D (italic) 0x1F (underline)
//     0x16 (reverse) 0x0F (reset) 0x11 (monospace) 0x1E (strikethrough)
//
// Non-ASCII UTF-8 is preserved. Invalid UTF-8 bytes are replaced with `?`.
func Text(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == 0x1B && i+1 < len(s) && s[i+1] == '[':
			// CSI: ESC [ ... <final 0x40-0x7E>
			j := i + 2
			for j < len(s) {
				if s[j] >= 0x40 && s[j] <= 0x7E {
					j++
					break
				}
				j++
			}
			i = j
		case c == 0x1B && i+1 < len(s) && s[i+1] == ']':
			// OSC: ESC ] ... BEL  or  ESC ] ... ESC \
			j := i + 2
			for j < len(s) {
				if s[j] == 0x07 {
					j++
					break
				}
				if s[j] == 0x1B && j+1 < len(s) && s[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j
		case c == 0x1B:
			// Other ESC sequences: skip ESC + next byte conservatively.
			i += 2
			if i > len(s) {
				i = len(s)
			}
		case c == 0x03:
			// mIRC color: 0x03 [<fg-digit>{0,2}] [, <bg-digit>{0,2}]
			j := i + 1
			j = consumeDigits(s, j, 2)
			if j < len(s) && s[j] == ',' {
				if k := consumeDigits(s, j+1, 2); k > j+1 {
					j = k
				}
			}
			i = j
		case c == 0x02 || c == 0x0F || c == 0x11 || c == 0x16 || c == 0x1D || c == 0x1E || c == 0x1F:
			// mIRC formatting toggles: drop.
			i++
		case c < 0x20 || c == 0x7F:
			// Other C0 controls (incl. tab, BELL, NUL, CR, LF) → space.
			b.WriteByte(' ')
			i++
		case c < 0x80:
			b.WriteByte(c)
			i++
		default:
			// Multi-byte UTF-8 — preserve if valid, otherwise replace.
			r, sz := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && sz == 1 {
				b.WriteByte('?')
				i++
			} else {
				b.WriteString(s[i : i+sz])
				i += sz
			}
		}
	}
	// Note: spaces (incl. trailing) are preserved — Insert(" ") for the
	// input field must round-trip through this function unchanged.
	return b.String()
}

// Nick returns the nickname portion of an IRC source. Inputs may be a bare
// nick or a "nick!user@host" hostmask; only the nick is returned, ensuring
// user@host info is never leaked to the UI.
func Nick(source string) string {
	source = Text(source)
	if i := strings.IndexAny(source, "!@"); i >= 0 {
		return source[:i]
	}
	return source
}

// Input returns s with control bytes and ANSI escapes stripped, suitable for
// applying to a user-typed input field where pasted content may include
// terminal escapes or null bytes.
func Input(s string) string {
	return Text(s)
}

func consumeDigits(s string, start, max int) int {
	n := 0
	for i := start; i < len(s) && n < max; i++ {
		if s[i] < '0' || s[i] > '9' {
			break
		}
		n++
	}
	return start + n
}
