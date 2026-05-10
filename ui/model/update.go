package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"coo/config"
	"coo/irc"
	"coo/ui/components"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	// IRC events
	case irc.ConnectedMsg:
		m.connected = true
		m.serverLine("connected to " + msg.Network + " as " + msg.Nick)
		return m, waitForEvent(m.client.Events())

	case irc.DisconnectedMsg:
		m.connected = false
		m.serverLine("disconnected: " + msg.Reason)
		if m.quitting {
			return m, tea.Quit
		}
		return m, waitForEvent(m.client.Events())

	case irc.ConnErrorMsg:
		m.serverLine("error: " + msg.Err.Error())
		return m, waitForEvent(m.client.Events())

	case irc.ServerLineMsg:
		m.serverLineAt(msg.Time, msg.Text)
		return m, waitForEvent(m.client.Events())

	case irc.PrivmsgMsg:
		m.handlePrivmsg(msg)
		return m, waitForEvent(m.client.Events())

	case irc.NoticeMsg:
		m.handleNotice(msg)
		return m, waitForEvent(m.client.Events())

	case irc.JoinMsg:
		m.handleJoin(msg)
		return m, waitForEvent(m.client.Events())

	case irc.PartMsg:
		m.handlePart(msg)
		return m, waitForEvent(m.client.Events())

	case irc.QuitMsg:
		m.handleQuit(msg)
		return m, waitForEvent(m.client.Events())

	case irc.NickChangeMsg:
		m.handleNickChange(msg)
		return m, waitForEvent(m.client.Events())

	case irc.TopicMsg:
		if b := m.findBuffer(msg.Channel); b != nil {
			b.Topic = msg.Topic
			text := "topic: " + msg.Topic
			if msg.Topic == "" {
				text = "no topic set"
			} else if msg.SetBy != "" {
				text = msg.SetBy + " set topic: " + msg.Topic
			}
			b.Append(Line{Time: msg.Time, Kind: LineSystem, Text: text})
		}
		return m, waitForEvent(m.client.Events())

	case irc.NamesMsg:
		if b := m.findBuffer(msg.Channel); b != nil {
			b.pendingNames = append(b.pendingNames, msg.Nicks...)
		}
		return m, waitForEvent(m.client.Events())

	case irc.InfoLineMsg:
		// Route query responses to the active buffer at delivery time —
		// where the user typed the command, in the common case.
		b := m.CurrentBuffer()
		if b == nil {
			b = m.findBuffer("")
		}
		b.Append(Line{Time: msg.Time, Kind: LineSystem, Text: msg.Text})
		return m, waitForEvent(m.client.Events())

	case irc.KickedMsg:
		b := m.findBuffer(msg.Channel)
		text := msg.Kicker + " kicked " + msg.Victim
		if msg.Reason != "" {
			text += " (" + msg.Reason + ")"
		}
		b.Append(Line{Time: msg.Time, Kind: LinePart, Nick: msg.Kicker, Text: text})
		if msg.Self {
			m.removeBuffer(msg.Channel)
		}
		return m, waitForEvent(m.client.Events())

	case irc.ModeMsg:
		// Route to the buffer named by Target if it's a channel; otherwise
		// the server buffer.
		var b *Buffer
		if irc.IsValidChannelName(msg.Target) {
			b = m.findBuffer(msg.Target)
		} else {
			b = m.findBuffer("")
		}
		text := "mode " + msg.Modes + " on " + msg.Target
		if msg.Setter != "" {
			text = msg.Setter + " set " + text
		}
		b.Append(Line{Time: msg.Time, Kind: LineSystem, Text: text})
		return m, waitForEvent(m.client.Events())

	case irc.InvitedMsg:
		b := m.findBuffer("")
		b.Append(Line{
			Time: msg.Time, Kind: LineSystem,
			Text: msg.From + " invited you to " + msg.Channel + " (use /join " + msg.Channel + ")",
		})
		return m, waitForEvent(m.client.Events())

	case irc.TopicWhoTimeMsg:
		if b := m.findBuffer(msg.Channel); b != nil {
			b.TopicSetBy = msg.SetBy
			text := "topic set by " + msg.SetBy
			if !msg.SetAt.IsZero() {
				text += " on " + msg.SetAt.Format("2006-01-02")
			}
			b.Append(Line{Time: msg.Time, Kind: LineSystem, Text: text})
		}
		return m, waitForEvent(m.client.Events())

	case tea.MouseClickMsg:
		if m.overlay == OverlayNames {
			for _, h := range m.namesHits {
				if msg.Y == h.Y && msg.X >= h.StartX && msg.X < h.EndX {
					m.openQuery(h.Nick)
					return m, nil
				}
			}
			return m, nil
		}
		if m.overlay != OverlayNone {
			return m, nil
		}
		// Strip click: y == 0 in altscreen layout.
		if msg.Y == 0 {
			for _, h := range m.tabHits {
				if msg.X >= h.StartX && msg.X < h.EndX {
					switch h.Index {
					case components.HitPrev:
						m.nextChannel(-1)
					case components.HitNext:
						m.nextChannel(1)
					default:
						if h.Index >= 0 && h.Index < len(m.order) {
							m.active = h.Index
							m.markRead()
						}
					}
					return m, nil
				}
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		if m.overlay == OverlayNames {
			b := m.findBuffer(m.namesChannel)
			if b != nil && len(b.Names) > 0 {
				switch msg.Button {
				case tea.MouseWheelUp:
					m.namesIdx -= 3
				case tea.MouseWheelDown:
					m.namesIdx += 3
				}
				m.namesIdx = clamp(m.namesIdx, 0, len(b.Names)-1)
			}
			return m, nil
		}
		if m.overlay != OverlayNone {
			return m, nil
		}
		if b := m.CurrentBuffer(); b != nil {
			switch msg.Button {
			case tea.MouseWheelUp:
				b.ScrollOff += 3
			case tea.MouseWheelDown:
				b.ScrollOff -= 3
			}
			b.ClampScroll()
		}
		return m, nil

	case irc.NamesEndMsg:
		if b := m.findBuffer(msg.Channel); b != nil {
			b.Names = b.pendingNames
			b.pendingNames = nil
			key := strings.ToLower(msg.Channel)
			if _, ok := m.pendingNamesShow[key]; ok {
				delete(m.pendingNamesShow, key)
				m.openNamesOverlay(msg.Channel)
			}
		}
		return m, waitForEvent(m.client.Events())
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.overlay != OverlayNone {
		switch key {
		case "esc", "q":
			m.overlay = OverlayNone
			m.keymapScroll = 0
			m.namesIdx = 0
			return m, nil
		}
		switch m.overlay {
		case OverlayThemes:
			return m.handleThemeOverlayKey(key)
		case OverlayNames:
			return m.handleNamesOverlayKey(key)
		default:
			return m.handleKeymapOverlayKey(key)
		}
	}

	switch key {
	case "ctrl+c":
		m.quitting = true
		_ = m.client.Send("QUIT", "coo")
		m.client.Close()
		return m, tea.Quit
	case "ctrl+n":
		m.nextChannel(1)
		return m, nil
	case "ctrl+p":
		m.nextChannel(-1)
		return m, nil
	case "ctrl+w":
		if t := m.CurrentTarget(); t != "" {
			_ = m.client.Part(t, "")
			m.removeBuffer(t)
		}
		return m, nil
	case "ctrl+l":
		if b := m.CurrentBuffer(); b != nil {
			b.Clear()
		}
		return m, nil
	case "ctrl+t":
		m.overlay = OverlayThemes
		return m, nil
	case "pgup":
		if b := m.CurrentBuffer(); b != nil {
			b.ScrollOff += 10
			b.ClampScroll()
		}
		return m, nil
	case "pgdown":
		if b := m.CurrentBuffer(); b != nil {
			b.ScrollOff -= 10
			b.ClampScroll()
		}
		return m, nil
	case "home":
		if b := m.CurrentBuffer(); b != nil {
			b.ScrollOff = len(b.Lines)
		}
		return m, nil
	case "end":
		if b := m.CurrentBuffer(); b != nil {
			b.ScrollOff = 0
		}
		return m, nil
	}

	if strings.HasPrefix(key, "alt+") && len(key) == 5 {
		d := key[4]
		if d >= '1' && d <= '9' {
			idx := int(d - '1')
			if idx < len(m.order) {
				m.active = idx
				m.markRead()
			}
			return m, nil
		}
	}

	// `?` toggles help when input is empty.
	if key == "?" && m.input.IsEmpty() {
		m.overlay = OverlayKeymap
		m.keymapScroll = 0
		return m, nil
	}

	switch key {
	case "enter":
		text := strings.TrimSpace(m.input.Value)
		m.input.Reset()
		if text == "" {
			return m, nil
		}
		if !m.runSlash(text) {
			m.sendChat(text)
		}
		return m, nil
	case "backspace":
		m.input.Backspace()
		return m, nil
	case "delete":
		m.input.Delete()
		return m, nil
	case "left":
		m.input.Left()
		return m, nil
	case "right":
		m.input.Right()
		return m, nil
	case "ctrl+a":
		m.input.Home()
		return m, nil
	case "ctrl+e":
		m.input.End()
		return m, nil
	case "ctrl+u":
		m.input.Reset()
		return m, nil
	case "tab":
		m.completeNick()
		return m, nil
	case "space":
		m.input.Insert(" ")
		return m, nil
	}

	// Printable runes — bubbletea v2 reports them as the literal string.
	if r := msg.Text; r != "" {
		m.input.Insert(r)
		return m, nil
	}
	if len(key) == 1 {
		m.input.Insert(key)
	}
	return m, nil
}

func (m *Model) handleKeymapOverlayKey(key string) (tea.Model, tea.Cmd) {
	maxScroll := components.MaxKeymapScroll(m.width, m.height, keymapGroups())
	step := 1
	bigStep := 5
	switch key {
	case "?":
		m.overlay = OverlayNone
		m.keymapScroll = 0
	case "up", "k":
		m.keymapScroll -= step
	case "down", "j":
		m.keymapScroll += step
	case "pgup":
		m.keymapScroll -= bigStep
	case "pgdown":
		m.keymapScroll += bigStep
	case "home", "g":
		m.keymapScroll = 0
	case "end", "G":
		m.keymapScroll = maxScroll
	}
	if m.keymapScroll < 0 {
		m.keymapScroll = 0
	}
	if m.keymapScroll > maxScroll {
		m.keymapScroll = maxScroll
	}
	return m, nil
}

func (m *Model) handleNamesOverlayKey(key string) (tea.Model, tea.Cmd) {
	b := m.findBuffer(m.namesChannel)
	if b == nil || len(b.Names) == 0 {
		m.overlay = OverlayNone
		return m, nil
	}
	last := len(b.Names) - 1
	switch key {
	case "up", "k":
		m.namesIdx--
	case "down", "j":
		m.namesIdx++
	case "pgup":
		m.namesIdx -= 10
	case "pgdown":
		m.namesIdx += 10
	case "home", "g":
		m.namesIdx = 0
	case "end", "G":
		m.namesIdx = last
	case "enter":
		_, bare := components.SplitModePrefix(b.Names[clamp(m.namesIdx, 0, last)])
		if bare != "" {
			m.openQuery(bare)
		}
		return m, nil
	}
	m.namesIdx = clamp(m.namesIdx, 0, last)
	return m, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Model) handleThemeOverlayKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.themeIdx > 0 {
			m.themeIdx--
		}
	case "down", "j":
		if m.themeIdx < len(m.themes)-1 {
			m.themeIdx++
		}
	case "enter":
		m.applyThemeIdx(m.themeIdx)
		_ = config.SaveTheme(m.ThemeName())
		m.overlay = OverlayNone
	}
	return m, nil
}

func (m *Model) nextChannel(delta int) {
	if len(m.order) == 0 {
		return
	}
	m.active = (m.active + delta + len(m.order)) % len(m.order)
	m.markRead()
}

func (m *Model) markRead() {
	if b := m.CurrentBuffer(); b != nil {
		b.Unread = 0
		b.Highlighted = false
	}
}

func (m *Model) handlePrivmsg(msg irc.PrivmsgMsg) {
	target := msg.Target
	// Direct messages addressed to us land in a buffer named after the sender.
	if !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "&") {
		target = msg.From
	}
	b := m.findBuffer(target)
	kind := LineChat
	if msg.IsAction {
		kind = LineAction
	}
	b.Append(Line{Time: msg.Time, Kind: kind, Nick: msg.From, Text: msg.Text})
	m.markActivity(b, msg.Text)
}

func (m *Model) handleNotice(msg irc.NoticeMsg) {
	target := msg.Target
	if !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "&") {
		target = ServerBufferName
	}
	b := m.findBuffer(target)
	b.Append(Line{Time: msg.Time, Kind: LineNotice, Nick: msg.From, Text: msg.Text})
	m.markActivity(b, msg.Text)
}

func (m *Model) handleJoin(msg irc.JoinMsg) {
	b := m.findBuffer(msg.Channel)
	if msg.Nick == m.client.Nick() {
		for i, ob := range m.order {
			if ob == b {
				m.active = i
				break
			}
		}
	}
	b.Append(Line{Time: msg.Time, Kind: LineJoin, Nick: msg.Nick, Text: "joined " + msg.Channel})
}

func (m *Model) handlePart(msg irc.PartMsg) {
	b := m.findBuffer(msg.Channel)
	text := "left " + msg.Channel
	if msg.Reason != "" {
		text += " (" + msg.Reason + ")"
	}
	b.Append(Line{Time: msg.Time, Kind: LinePart, Nick: msg.Nick, Text: text})
}

func (m *Model) handleQuit(msg irc.QuitMsg) {
	text := "quit"
	if msg.Reason != "" {
		text += " (" + msg.Reason + ")"
	}
	for _, b := range m.order {
		if b.IsServer {
			continue
		}
		b.Append(Line{Time: msg.Time, Kind: LinePart, Nick: msg.Nick, Text: text})
	}
}

func (m *Model) handleNickChange(msg irc.NickChangeMsg) {
	for _, b := range m.order {
		if b.IsServer {
			continue
		}
		b.Append(Line{Time: msg.Time, Kind: LineNick, Nick: msg.From, Text: "is now " + msg.To})
	}
}

func (m *Model) markActivity(b *Buffer, text string) {
	if b == m.activeBuffer() || b.IsServer {
		return
	}
	b.Unread++
	mention := strings.Contains(strings.ToLower(text), strings.ToLower(m.client.Nick()))
	if mention || !strings.HasPrefix(b.Name, "#") {
		b.Highlighted = true
	}
}

func (m *Model) serverLine(text string) {
	m.serverLineAt(time.Now(), text)
}

func (m *Model) serverLineAt(t time.Time, text string) {
	b := m.findBuffer("")
	b.Append(Line{Time: t, Kind: LineServer, Text: text})
}

// completeNick prefix-completes the word at the cursor against the channel's
// participant list (NAMES), falling back to nicks seen in recent buffer
// lines. When the completion is at the start of input, append ": " — the
// IRCv2 convention for addressing a user.
func (m *Model) completeNick() {
	b := m.CurrentBuffer()
	if b == nil {
		return
	}
	prefix := lastWord(m.input.Value[:m.input.Cursor])
	if prefix == "" {
		return
	}
	prefixLow := strings.ToLower(prefix)

	candidate := matchNick(b.Names, prefixLow)
	if candidate == "" {
		candidate = matchHistoryNick(b.Lines, prefixLow)
	}
	if candidate == "" {
		return
	}
	suffix := candidate[len(prefix):]
	atStart := strings.TrimSpace(m.input.Value[:m.input.Cursor-len(prefix)]) == ""
	if atStart {
		suffix += ": "
	}
	m.input.Insert(suffix)
}

// matchNick returns the first participant whose lowercase form (after
// stripping leading IRC mode chars like @ + % & ~) starts with prefixLow.
func matchNick(names []string, prefixLow string) string {
	for _, n := range names {
		bare := strings.TrimLeft(n, "@+%&~")
		if strings.HasPrefix(strings.ToLower(bare), prefixLow) {
			return bare
		}
	}
	return ""
}

// matchHistoryNick scans the last 50 buffer lines for a recently-active nick.
func matchHistoryNick(lines []Line, prefixLow string) string {
	seen := make(map[string]struct{})
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}
	for i := len(lines) - 1; i >= start; i-- {
		nick := lines[i].Nick
		if nick == "" {
			continue
		}
		key := strings.ToLower(nick)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if strings.HasPrefix(key, prefixLow) {
			return nick
		}
	}
	return ""
}

func lastWord(s string) string {
	i := strings.LastIndexByte(s, ' ')
	if i < 0 {
		return s
	}
	return s[i+1:]
}
