package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"coo/ui"
	"coo/ui/components"
)

// View renders the current model state.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *Model) render() string {
	w := m.width
	h := m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	strip, hits := components.Strip(w, m.tabs(), m.client.Network(), m.connStatus())
	m.tabHits = hits
	input := m.renderInput(w)
	topic := m.renderTopic(w)

	// strip(1) + topic(0|1) + bufferArea + input(1).
	reserved := 2
	if topic != "" {
		reserved++
	}
	bufferH := h - reserved
	if bufferH < 1 {
		bufferH = 1
	}

	bufferLines := []components.Line{}
	scrollOff := 0
	if b := m.CurrentBuffer(); b != nil {
		bufferLines = make([]components.Line, len(b.Lines))
		for i, l := range b.Lines {
			bufferLines[i] = components.Line{
				Time: l.Time, Kind: components.LineKind(l.Kind),
				Nick: l.Nick, Text: l.Text,
			}
		}
		scrollOff = b.ScrollOff
	}
	body := components.Render(w, bufferH, bufferLines, scrollOff, m.client.Nick())

	parts := []string{strip}
	if topic != "" {
		parts = append(parts, topic)
	}
	parts = append(parts, body, input)
	out := strings.Join(parts, "\n")

	switch m.overlay {
	case OverlayKeymap:
		return components.Keymap(w, h, keymapGroups(), m.keymapScroll)
	case OverlayThemes:
		return components.ThemePicker(w, h, m.themeNames(), m.themeIdx)
	case OverlayNames:
		b := m.findBuffer(m.namesChannel)
		if b == nil {
			return out
		}
		rendered, hits := components.NamesList(w, h, m.namesChannel, b.Names, m.namesIdx)
		m.namesHits = hits
		return rendered
	}
	return out
}

func (m *Model) tabs() []components.Tab {
	out := make([]components.Tab, 0, len(m.order))
	for i, b := range m.order {
		out = append(out, components.Tab{
			Name: b.Name, Active: i == m.active,
			IsServer: b.IsServer, IsPM: b.IsPM(),
			Unread: b.Unread, Highlighted: b.Highlighted,
		})
	}
	return out
}

// renderTopic returns the channel topic line shown above the input, or "" if
// the active buffer has no topic to show.
func (m *Model) renderTopic(width int) string {
	b := m.CurrentBuffer()
	if b == nil || !b.IsChannel() || b.Topic == "" {
		return ""
	}
	prefix := lipgloss.NewStyle().Foreground(ui.ColorAccent).Render("topic ")
	body := lipgloss.NewStyle().Foreground(ui.ColorDim).Render(b.Topic)
	return lipgloss.NewStyle().Width(width).Render(prefix + body)
}

func (m *Model) connStatus() string {
	if m.connected {
		return "✓"
	}
	return "…"
}

func (m *Model) renderInput(width int) string {
	prompt := "*server*"
	if t := m.CurrentTarget(); t != "" {
		prompt = t
	}
	promptStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	cursor := "▏"
	cursorStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)

	left := promptStyle.Render(prompt + "›") + " "
	value := m.input.Value
	c := m.input.Cursor
	if c < 0 {
		c = 0
	}
	if c > len(value) {
		c = len(value)
	}
	body := textStyle.Render(value[:c]) + cursorStyle.Render(cursor) + textStyle.Render(value[c:])
	return lipgloss.NewStyle().Width(width).Render(left + body)
}

var cachedKeymapGroups = func() []components.Group {
	out := make([]components.Group, 0, len(Keymap))
	for _, g := range Keymap {
		bs := make([][2]string, len(g.Bindings))
		for i, b := range g.Bindings {
			bs[i] = [2]string{b.Keys, b.Desc}
		}
		out = append(out, components.Group{Title: g.Title, Bindings: bs})
	}
	return out
}()

func keymapGroups() []components.Group { return cachedKeymapGroups }

