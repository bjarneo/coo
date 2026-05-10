package model

import (
	"time"
)

// LineKind classifies a buffer entry so the view can color it consistently.
type LineKind int

const (
	LineChat LineKind = iota
	LineAction
	LineJoin
	LinePart
	LineNick
	LineNotice
	LineServer
	LineSystem
	LineError
	LineSelf
)

// Line is a single rendered row in a channel buffer.
type Line struct {
	Time time.Time
	Kind LineKind
	Nick string
	Text string
}

// Buffer is a per-channel ring of lines plus unread/highlight tracking and
// a vertical scroll offset (0 = pinned to bottom).
type Buffer struct {
	Name         string
	IsServer     bool
	Lines        []Line
	Unread       int
	Highlighted  bool
	ScrollOff    int
	Topic        string
	TopicSetBy   string
	Names        []string
	pendingNames []string
}

// IsChannel reports whether the buffer name is a channel (#x or &x).
func (b *Buffer) IsChannel() bool {
	if b.Name == "" || b.IsServer {
		return false
	}
	c := b.Name[0]
	return c == '#' || c == '&'
}

// IsPM reports whether the buffer is a private-message conversation.
func (b *Buffer) IsPM() bool {
	return !b.IsServer && !b.IsChannel()
}

const bufferCap = 5000

// Append adds a line, capping the ring at bufferCap.
//
// When the user has scrolled away from the bottom (ScrollOff > 0), the
// scroll offset is bumped so the visible window stays anchored to the same
// content rather than sliding toward the new line. This is approximate for
// wrapped rows (off by N-1 for an N-row wrap), which is good enough for
// chat where most lines fit on one row.
func (b *Buffer) Append(l Line) {
	b.Lines = append(b.Lines, l)
	if extra := len(b.Lines) - bufferCap; extra > 0 {
		b.Lines = b.Lines[extra:]
		if b.ScrollOff > 0 {
			b.ScrollOff -= extra
			if b.ScrollOff < 0 {
				b.ScrollOff = 0
			}
		}
	}
	if b.ScrollOff > 0 {
		b.ScrollOff++
	}
}

// Clear empties the buffer and resets unread state.
func (b *Buffer) Clear() {
	b.Lines = b.Lines[:0]
	b.Unread = 0
	b.Highlighted = false
	b.ScrollOff = 0
}

// ClampScroll bounds ScrollOff to [0, len(Lines)] so PgUp can't scroll past
// the start of history.
func (b *Buffer) ClampScroll() {
	if b.ScrollOff < 0 {
		b.ScrollOff = 0
	}
	if max := len(b.Lines); b.ScrollOff > max {
		b.ScrollOff = max
	}
}
