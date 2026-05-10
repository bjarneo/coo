package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"coo/ui"
)

// LineKind mirrors model.LineKind without creating an import cycle. Callers
// pass the int directly.
type LineKind int

const (
	KindChat LineKind = iota
	KindAction
	KindJoin
	KindPart
	KindNick
	KindNotice
	KindServer
	KindSystem
	KindError
	KindSelf
)

// Line is the data needed to render one row.
type Line struct {
	Time time.Time
	Kind LineKind
	Nick string
	Text string
}

// Render returns a wrapped, colored block for a buffer's lines, taking
// scrollOff into account (0 = pinned to bottom).
func Render(width, height int, lines []Line, scrollOff int, selfNick string) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	selfNickLower := strings.ToLower(selfNick)

	timeStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle)
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	textStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	joinStyle := lipgloss.NewStyle().Foreground(ui.ColorJoin)
	partStyle := lipgloss.NewStyle().Foreground(ui.ColorPart)
	errorStyle := lipgloss.NewStyle().Foreground(ui.ColorError)
	noticeStyle := lipgloss.NewStyle().Foreground(ui.ColorMention)
	selfStyle := lipgloss.NewStyle().Foreground(ui.ColorNickSelf).Bold(true)
	actionStyle := lipgloss.NewStyle().Foreground(ui.ColorMention).Italic(true)
	mentionTextStyle := lipgloss.NewStyle().Foreground(ui.ColorMention).Bold(true)

	rendered := make([]string, 0, len(lines))
	for _, l := range lines {
		ts := timeStyle.Render(l.Time.Format("15:04"))
		var body string
		switch l.Kind {
		case KindJoin:
			body = joinStyle.Render(fmt.Sprintf("→ %s %s", l.Nick, l.Text))
		case KindPart:
			body = partStyle.Render(fmt.Sprintf("← %s %s", l.Nick, l.Text))
		case KindNick:
			body = dimStyle.Render(fmt.Sprintf("%s %s", l.Nick, l.Text))
		case KindAction:
			body = actionStyle.Render(fmt.Sprintf("* %s %s", l.Nick, l.Text))
		case KindNotice:
			body = noticeStyle.Render(fmt.Sprintf("-%s- %s", l.Nick, l.Text))
		case KindServer:
			body = dimStyle.Render(l.Text)
		case KindSystem:
			body = dimStyle.Italic(true).Render(l.Text)
		case KindError:
			body = errorStyle.Render(l.Text)
		case KindSelf:
			body = selfStyle.Render("<"+l.Nick+">") + " " + textStyle.Render(l.Text)
		default:
			nickStyle := lipgloss.NewStyle().Foreground(ui.NickColor(l.Nick)).Bold(true)
			text := l.Text
			if selfNickLower != "" && strings.Contains(strings.ToLower(text), selfNickLower) {
				body = nickStyle.Render("<"+l.Nick+">") + " " + mentionTextStyle.Render(text)
			} else {
				body = nickStyle.Render("<"+l.Nick+">") + " " + textStyle.Render(text)
			}
		}
		row := ts + " " + body
		if lipgloss.Width(row) > width {
			row = lipgloss.NewStyle().Width(width).Render(row)
		}
		rendered = append(rendered, strings.Split(row, "\n")...)
	}

	// Apply scroll offset (counted from the bottom).
	if scrollOff < 0 {
		scrollOff = 0
	}
	end := len(rendered) - scrollOff
	if end < 0 {
		end = 0
	}
	start := end - height
	if start < 0 {
		start = 0
	}
	if end > len(rendered) {
		end = len(rendered)
	}
	if start > end {
		start = end
	}
	view := rendered[start:end]
	for len(view) < height {
		view = append([]string{""}, view...)
	}
	if len(view) > height {
		view = view[len(view)-height:]
	}
	return strings.Join(view, "\n")
}

