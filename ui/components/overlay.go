package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"coo/ui"
)

// NameHit is a click target for one nick row in the NamesList overlay.
type NameHit struct {
	StartX, EndX int
	Y            int
	Nick         string
}

// Group is a heading + key/desc rows for the keymap overlay.
type Group struct {
	Title    string
	Bindings [][2]string // {keys, desc}
}

// overlayLayout returns the inner content width/height available to overlay
// bodies after subtracting border (2) + padding (2 horizontal, 1 vertical) +
// outer screen margin (4 horizontal, 2 vertical).
func overlayLayout(width, height int) (innerW, innerH int) {
	innerW = width - 4 - 2 - 4 // screen margin + border + padding
	if innerW < 20 {
		innerW = 20
	}
	if innerW > 80 {
		innerW = 80
	}
	innerH = height - 2 - 2 - 2 // screen margin + border + padding
	if innerH < 4 {
		innerH = 4
	}
	return innerW, innerH
}

func boxStyle(w int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorAccent).
		Padding(1, 2).
		Width(w)
}

// Keymap renders a centered modal listing all key bindings, with scroll
// support when content exceeds the available height.
func Keymap(width, height int, groups []Group, scroll int) string {
	innerW, innerH := overlayLayout(width, height)

	titleStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle).Italic(true)
	moreStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)

	// Flatten all groups into a single row stream.
	rows := make([]string, 0, 64)
	for i, g := range groups {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, titleStyle.Render(g.Title))
		for _, b := range g.Bindings {
			rows = append(rows, "  "+padRight(keyStyle.Render(b[0]), 28)+descStyle.Render(b[1]))
		}
	}

	// Reserve 2 lines for the bottom hint+spacer.
	visible := innerH - 2
	if visible < 1 {
		visible = 1
	}
	scroll, body := windowRows(rows, scroll, visible)

	// Top/bottom overflow markers consume one visible row each when active.
	hasUp := scroll > 0
	hasDown := scroll+visible < len(rows)
	if hasUp {
		body[0] = moreStyle.Render("▲ more above")
	}
	if hasDown {
		body[len(body)-1] = moreStyle.Render("▼ more below")
	}

	hint := hintStyle.Render("? or esc to close · ↑/↓ to scroll")
	content := strings.Join(body, "\n") + "\n\n" + hint

	box := boxStyle(innerW).Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// ThemePicker renders a centered list of theme names. The window slides to
// keep idx visible regardless of list length.
func ThemePicker(width, height int, names []string, idx int) string {
	innerW, innerH := overlayLayout(width, height)

	titleStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)
	idleStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle).Italic(true)
	moreStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)

	// Title (1) + blank (1) + hint+spacer (2) = 4 lines reserved.
	visible := innerH - 4
	if visible < 1 {
		visible = 1
	}

	rows := make([]string, len(names))
	for i, n := range names {
		if i == idx {
			rows[i] = "▸ " + activeStyle.Render(n)
		} else {
			rows[i] = "  " + idleStyle.Render(n)
		}
	}
	scroll := scrollToShow(idx, visible, len(rows))
	_, body := windowRows(rows, scroll, visible)

	hasUp := scroll > 0
	hasDown := scroll+visible < len(rows)
	if hasUp {
		body[0] = moreStyle.Render("▲ more above")
	}
	if hasDown {
		body[len(body)-1] = moreStyle.Render("▼ more below")
	}

	content := titleStyle.Render("Theme") + "\n\n" + strings.Join(body, "\n") + "\n\n" +
		hintStyle.Render("↑/↓ choose · enter apply · esc cancel")

	box := boxStyle(innerW).Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// windowRows returns up to height rows starting from clamped(scroll). The
// returned slice is exactly height long, padded with empty strings if rows
// are shorter than the window.
func windowRows(rows []string, scroll, height int) (int, []string) {
	if height <= 0 {
		return 0, []string{}
	}
	if scroll < 0 {
		scroll = 0
	}
	max := len(rows) - height
	if max < 0 {
		max = 0
	}
	if scroll > max {
		scroll = max
	}
	end := scroll + height
	if end > len(rows) {
		end = len(rows)
	}
	out := make([]string, height)
	copy(out, rows[scroll:end])
	return scroll, out
}

// scrollToShow returns a scroll offset that keeps idx within a window of
// the given height.
func scrollToShow(idx, height, total int) int {
	if height <= 0 || total <= height {
		return 0
	}
	scroll := idx - height/2
	if scroll < 0 {
		scroll = 0
	}
	if scroll > total-height {
		scroll = total - height
	}
	return scroll
}

// MaxKeymapScroll returns the max valid scroll offset for the given keymap
// content and screen size, so the model can clamp scroll input.
func MaxKeymapScroll(width, height int, groups []Group) int {
	_, innerH := overlayLayout(width, height)
	visible := innerH - 2
	if visible < 1 {
		visible = 1
	}
	rowCount := 0
	for i, g := range groups {
		if i > 0 {
			rowCount++ // blank separator
		}
		rowCount++ // title
		rowCount += len(g.Bindings)
	}
	if rowCount <= visible {
		return 0
	}
	return rowCount - visible
}

func padRight(s string, w int) string {
	d := w - lipgloss.Width(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

// NamesList renders a centered modal listing channel participants and
// returns click ranges for each visible nick. The currently-selected row
// (idx) is rendered with the active style; the visible window auto-scrolls
// to keep idx in view. Mode-prefix characters (@, +, etc.) are shown but
// stripped from Hit.Nick so clicking opens a query buffer with the bare
// nickname.
func NamesList(width, height int, channel string, names []string, idx int) (string, []NameHit) {
	innerW, innerH := overlayLayout(width, height)

	titleStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	nickStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)
	opStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorSubtle).Italic(true)
	moreStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)

	// Reserve: 1 title + 1 blank + 1 blank + 1 hint = 4 chrome lines
	visible := innerH - 4
	if visible < 1 {
		visible = 1
	}

	if idx < 0 {
		idx = 0
	}
	if idx >= len(names) && len(names) > 0 {
		idx = len(names) - 1
	}

	rows := make([]string, len(names))
	for i, n := range names {
		prefix, bare := SplitModePrefix(n)
		ns := nickStyle
		marker := "  "
		if i == idx {
			ns = activeStyle
			marker = "▸ "
		}
		if prefix != "" {
			rows[i] = marker + opStyle.Render(prefix) + ns.Render(bare)
		} else {
			rows[i] = marker + ns.Render(bare)
		}
	}
	scroll := scrollToShow(idx, visible, len(rows))
	_, body := windowRows(rows, scroll, visible)

	hasUp := scroll > 0
	hasDown := scroll+visible < len(rows)
	if hasUp {
		body[0] = moreStyle.Render("▲ more above")
	}
	if hasDown {
		body[len(body)-1] = moreStyle.Render("▼ more below")
	}

	title := titleStyle.Render(fmt.Sprintf("%s — %d users", channel, len(names)))
	hint := hintStyle.Render("↑/↓ select · enter or click to chat · esc close")
	content := title + "\n\n" + strings.Join(body, "\n") + "\n\n" + hint

	box := boxStyle(innerW).Render(content)
	placed := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)

	// Compute screen coordinates so click X/Y can be mapped back to a nick.
	contentLines := strings.Count(content, "\n") + 1
	boxH := contentLines + 4 // border 2 + padding 2
	boxW := innerW + 6       // padding 4 + border 2
	topY := (height - boxH) / 2
	if topY < 0 {
		topY = 0
	}
	leftX := (width - boxW) / 2
	if leftX < 0 {
		leftX = 0
	}
	// Inside the box: border(1) + padding(1) → start of content. Title is
	// row 0, blank row 1, names start at row 2.
	namesStartY := topY + 2 + 2
	contentStartX := leftX + 1 + 2 // border 1 + padding 2

	hits := make([]NameHit, 0, len(body))
	for i := 0; i < len(body); i++ {
		if hasUp && i == 0 {
			continue
		}
		if hasDown && i == len(body)-1 {
			continue
		}
		rowIdx := scroll + i
		if rowIdx >= len(names) || rowIdx < 0 {
			continue
		}
		_, bare := SplitModePrefix(names[rowIdx])
		if bare == "" {
			continue
		}
		// Row layout is "  <prefix><nick>" — count rune widths to compute
		// the click range over the entire prefix+nick span (so clicking a
		// `@` still selects the user).
		startX := contentStartX + 2
		endX := startX + lipgloss.Width(names[rowIdx])
		hits = append(hits, NameHit{
			StartX: startX, EndX: endX,
			Y:    namesStartY + i,
			Nick: bare,
		})
	}
	return placed, hits
}

// SplitModePrefix peels leading mode-prefix chars (@, +, %, &, ~) off a NAMES
// list entry and returns (prefix, nickname).
func SplitModePrefix(name string) (prefix, bare string) {
	for len(name) > 0 && strings.ContainsRune("@+%&~", rune(name[0])) {
		prefix += string(name[0])
		name = name[1:]
	}
	return prefix, name
}
