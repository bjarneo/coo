// Package components renders reusable UI fragments (channel strip, buffer,
// status line, overlays) used by the model.View.
package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"coo/ui"
)

// Tab is one entry in the channel strip.
type Tab struct {
	Name        string
	Active      bool
	IsServer    bool
	IsPM        bool
	Unread      int
	Highlighted bool
}

// TabHit is a clickable region in the rendered strip. Index is the tab
// position in the original tabs slice, or -1 for the "previous" overflow
// marker and -2 for "next".
type TabHit struct {
	StartX, EndX int
	Index        int
}

const (
	HitPrev = -1
	HitNext = -2
)

// segment is one rendered piece of the strip with its source identity, used
// to convert window contents into TabHit ranges with correct screen x-coords.
type segment struct {
	cell  string
	index int // tab index, HitPrev, or HitNext
}

// Strip renders the horizontal channel strip on the top bar and returns the
// click-target ranges for each visible tab. When the full list of tabs would
// not fit in width, a sliding window is rendered around the active tab with
// `‹N` / `N›` overflow markers.
func Strip(width int, tabs []Tab, network, status string) (string, []TabHit) {
	if width <= 0 {
		width = 80
	}

	activeStyle := lipgloss.NewStyle().
		Foreground(ui.ColorText).
		Background(ui.ColorAccent).
		Padding(0, 1).
		Bold(true)
	idleStyle := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(0, 1)
	highlightStyle := lipgloss.NewStyle().
		Foreground(ui.ColorError).
		Padding(0, 1).
		Bold(true)
	unreadStyle := lipgloss.NewStyle().
		Foreground(ui.ColorAccent).
		Padding(0, 1)
	sepStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle)
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	moreStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)

	right := statusStyle.Render(status + "  " + network)
	leftBudget := width - lipgloss.Width(right) - 1
	if leftBudget < 10 {
		leftBudget = 10
	}

	cells := make([]string, len(tabs))
	for i, t := range tabs {
		label := tabLabel(t)
		switch {
		case t.Active:
			cells[i] = activeStyle.Render(label)
		case t.Highlighted:
			cells[i] = highlightStyle.Render(label)
		case t.Unread > 0:
			cells[i] = unreadStyle.Render(label)
		default:
			cells[i] = idleStyle.Render(label)
		}
	}

	sep := sepStyle.Render("│")
	segments := pickWindow(cells, tabs, leftBudget, sep, moreStyle)

	// Stitch segments + separators, recording x-ranges as we go.
	var b strings.Builder
	hits := make([]TabHit, 0, len(segments))
	x := 0
	for i, s := range segments {
		if i > 0 {
			b.WriteString(sep)
			x += lipgloss.Width(sep)
		}
		w := lipgloss.Width(s.cell)
		hits = append(hits, TabHit{StartX: x, EndX: x + w, Index: s.index})
		b.WriteString(s.cell)
		x += w
	}
	left := b.String()

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := width - leftW - rightW
	if pad < 1 {
		pad = 1
	}
	bar := left + strings.Repeat(" ", pad) + right
	return lipgloss.NewStyle().Width(width).Render(bar), hits
}

func tabLabel(t Tab) string {
	switch {
	case t.IsServer:
		return "*server*"
	case t.IsPM:
		return "@" + t.Name
	default:
		return t.Name
	}
}

// pickWindow returns the visible cells (and overflow markers) as a list of
// segments tagged with the original tab index — used by Strip to map screen
// click positions back to a tab to switch to.
func pickWindow(cells []string, tabs []Tab, budget int, sep string, moreStyle lipgloss.Style) []segment {
	if len(cells) == 0 {
		return nil
	}
	active := 0
	for i, t := range tabs {
		if t.Active {
			active = i
			break
		}
	}
	sepW := lipgloss.Width(sep)

	// Active label alone exceeds budget — degrade with a truncated active.
	if lipgloss.Width(cells[active]) > budget {
		clipped := lipgloss.NewStyle().MaxWidth(budget).Render(cells[active])
		segs := []segment{{cell: clipped, index: active}}
		if active > 0 {
			segs = append([]segment{{cell: moreStyle.Render(fmt.Sprintf("‹%d", active)), index: HitPrev}}, segs...)
		}
		if active < len(cells)-1 {
			segs = append(segs, segment{cell: moreStyle.Render(fmt.Sprintf("%d›", len(cells)-1-active)), index: HitNext})
		}
		return segs
	}

	left, right := active, active
	used := lipgloss.Width(cells[active])
	for {
		grew := false
		if right+1 < len(cells) {
			cost := sepW + lipgloss.Width(cells[right+1])
			if used+cost <= budget {
				used += cost
				right++
				grew = true
			}
		}
		if left-1 >= 0 {
			cost := sepW + lipgloss.Width(cells[left-1])
			if used+cost <= budget {
				used += cost
				left--
				grew = true
			}
		}
		if !grew {
			break
		}
	}

	segs := make([]segment, 0, right-left+3)
	if left > 0 {
		segs = append(segs, segment{cell: moreStyle.Render(fmt.Sprintf("‹%d", left)), index: HitPrev})
	}
	for i := left; i <= right; i++ {
		segs = append(segs, segment{cell: cells[i], index: i})
	}
	if right < len(cells)-1 {
		segs = append(segs, segment{cell: moreStyle.Render(fmt.Sprintf("%d›", len(cells)-1-right)), index: HitNext})
	}
	return segs
}
