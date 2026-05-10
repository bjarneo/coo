package model

import (
	"unicode/utf8"

	"coo/internal/sanitize"
)

// MaxInputLen caps the length of the input field to defend against pasted
// blobs (megabyte clipboards) that would balloon memory and stall layout.
const MaxInputLen = 8192

// Input is a minimal single-line text input with cursor.
type Input struct {
	Value  string
	Cursor int // byte position
}

// Insert places text at the cursor, sanitizing control bytes and ANSI
// escapes from pasted content. Inserts that would exceed MaxInputLen are
// truncated rather than silently dropped — the user sees as much of their
// paste as the field can hold.
func (i *Input) Insert(s string) {
	s = sanitize.Input(s)
	if s == "" {
		return
	}
	room := MaxInputLen - len(i.Value)
	if room <= 0 {
		return
	}
	if len(s) > room {
		cut := room
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
		if s == "" {
			return
		}
	}
	i.Value = i.Value[:i.Cursor] + s + i.Value[i.Cursor:]
	i.Cursor += len(s)
}


// Backspace deletes the rune left of the cursor.
func (i *Input) Backspace() {
	if i.Cursor == 0 {
		return
	}
	_, sz := utf8.DecodeLastRuneInString(i.Value[:i.Cursor])
	i.Value = i.Value[:i.Cursor-sz] + i.Value[i.Cursor:]
	i.Cursor -= sz
}

// Delete removes the rune right of the cursor.
func (i *Input) Delete() {
	if i.Cursor >= len(i.Value) {
		return
	}
	_, sz := utf8.DecodeRuneInString(i.Value[i.Cursor:])
	i.Value = i.Value[:i.Cursor] + i.Value[i.Cursor+sz:]
}

// Left moves cursor one rune left.
func (i *Input) Left() {
	if i.Cursor == 0 {
		return
	}
	_, sz := utf8.DecodeLastRuneInString(i.Value[:i.Cursor])
	i.Cursor -= sz
}

// Right moves cursor one rune right.
func (i *Input) Right() {
	if i.Cursor >= len(i.Value) {
		return
	}
	_, sz := utf8.DecodeRuneInString(i.Value[i.Cursor:])
	i.Cursor += sz
}

// Home moves cursor to start.
func (i *Input) Home() { i.Cursor = 0 }

// End moves cursor to end.
func (i *Input) End() { i.Cursor = len(i.Value) }

// Reset clears the input.
func (i *Input) Reset() { i.Value = ""; i.Cursor = 0 }

// IsEmpty reports whether the input has no content.
func (i *Input) IsEmpty() bool { return i.Value == "" }
