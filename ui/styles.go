// Package ui contains color variables and rendering helpers shared across
// the Bubble Tea model.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"coo/theme"
)

// Color slots used throughout the UI. Changed at runtime by ApplyTheme.
var (
	ColorAccent  color.Color = lipgloss.ANSIColor(11) // active channel, highlights
	ColorText    color.Color = lipgloss.ANSIColor(15) // primary text
	ColorDim     color.Color = lipgloss.ANSIColor(7)  // muted text
	ColorSubtle  color.Color = lipgloss.ANSIColor(8)  // separators, hints
	ColorJoin    color.Color = lipgloss.ANSIColor(10) // joins, success
	ColorPart    color.Color = lipgloss.ANSIColor(11) // parts, warnings
	ColorError   color.Color = lipgloss.ANSIColor(9)  // errors, mentions
	ColorMention color.Color = lipgloss.ANSIColor(13) // your nick highlighted in chat
	ColorNickSelf color.Color = lipgloss.ANSIColor(14)
)

// ApplyTheme switches palette to t. Empty hex fields fall back to ANSI.
func ApplyTheme(t theme.Theme) {
	if t.IsDefault() {
		ColorAccent = lipgloss.ANSIColor(11)
		ColorText = lipgloss.ANSIColor(15)
		ColorDim = lipgloss.ANSIColor(7)
		ColorSubtle = lipgloss.ANSIColor(8)
		ColorJoin = lipgloss.ANSIColor(10)
		ColorPart = lipgloss.ANSIColor(11)
		ColorError = lipgloss.ANSIColor(9)
		ColorMention = lipgloss.ANSIColor(13)
		ColorNickSelf = lipgloss.ANSIColor(14)
		return
	}
	ColorAccent = lipgloss.Color(t.Accent)
	ColorText = lipgloss.Color(t.BrightFG)
	ColorDim = lipgloss.Color(t.FG)
	ColorSubtle = lipgloss.Color(t.FG)
	ColorJoin = lipgloss.Color(t.Green)
	ColorPart = lipgloss.Color(t.Yellow)
	ColorError = lipgloss.Color(t.Red)
	ColorMention = lipgloss.Color(t.Yellow)
	ColorNickSelf = lipgloss.Color(t.Accent)
}

var nickPalette = [...]color.Color{
	lipgloss.ANSIColor(2), lipgloss.ANSIColor(3), lipgloss.ANSIColor(4),
	lipgloss.ANSIColor(5), lipgloss.ANSIColor(6), lipgloss.ANSIColor(10),
	lipgloss.ANSIColor(11), lipgloss.ANSIColor(12), lipgloss.ANSIColor(13),
	lipgloss.ANSIColor(14),
}

// NickColor picks a deterministic color for nick by FNV-hashing the bytes.
func NickColor(nick string) color.Color {
	if nick == "" {
		return ColorText
	}
	var h uint32 = 2166136261
	for i := 0; i < len(nick); i++ {
		h ^= uint32(nick[i])
		h *= 16777619
	}
	return nickPalette[int(h)%len(nickPalette)]
}
