package model

import (
	"strings"
	"time"

	"coo/config"
	"coo/internal/sanitize"
	"coo/irc"
)

// runSlash interprets the input as a slash command. Returns true if the input
// was consumed (whether or not it succeeded).
func (m *Model) runSlash(input string) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}
	input = strings.TrimPrefix(input, "/")
	cmd, rest, _ := strings.Cut(input, " ")
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	rest = strings.TrimSpace(rest)

	switch cmd {
	case "join", "j":
		return m.cmdJoin(rest)
	case "part", "leave":
		return m.cmdPart(rest)
	case "msg", "query":
		return m.cmdMsg(rest)
	case "me":
		return m.cmdAction(rest)
	case "nick":
		return m.cmdNick(rest)
	case "quit", "exit":
		m.quitting = true
		if rest == "" {
			rest = "coo"
		}
		_ = m.client.Send("QUIT", rest)
		m.client.Close()
		return true
	case "theme":
		return m.cmdTheme(rest)
	case "themes":
		m.overlay = OverlayThemes
		return true
	case "names":
		return m.cmdNames(rest)
	case "topic":
		return m.cmdTopic(rest)
	case "whois":
		return m.cmdWhois(rest)
	case "whowas":
		return m.cmdWhowas(rest)
	case "who":
		return m.cmdWho(rest)
	case "kick":
		return m.cmdKick(rest)
	case "mode":
		return m.cmdMode(rest)
	case "op":
		return m.cmdOpVoice(rest, "+o")
	case "deop":
		return m.cmdOpVoice(rest, "-o")
	case "voice":
		return m.cmdOpVoice(rest, "+v")
	case "devoice":
		return m.cmdOpVoice(rest, "-v")
	case "ban":
		return m.cmdBan(rest, "+b")
	case "unban":
		return m.cmdBan(rest, "-b")
	case "invite":
		return m.cmdInvite(rest)
	case "away":
		return m.cmdAway(rest)
	case "back":
		_ = m.client.Away("")
		m.systemLine("away cleared")
		return true
	case "list":
		_ = m.client.List(rest)
		m.systemLine("requesting channel list…")
		return true
	case "notice":
		return m.cmdNotice(rest)
	case "ping":
		return m.cmdPing(rest)
	case "clear":
		if b := m.CurrentBuffer(); b != nil {
			b.Clear()
		}
		return true
	case "raw":
		return m.cmdRaw(rest)
	case "help", "?":
		m.overlay = OverlayKeymap
		m.keymapScroll = 0
		return true
	}
	m.systemLine("unknown command: /" + cmd)
	return true
}

func (m *Model) cmdJoin(rest string) bool {
	if rest == "" {
		m.systemLine("usage: /join #channel")
		return true
	}
	raw := strings.Fields(rest)
	channels := make([]string, 0, len(raw))
	for _, c := range raw {
		if !strings.HasPrefix(c, "#") && !strings.HasPrefix(c, "&") {
			c = "#" + c
		}
		if !irc.IsValidChannelName(c) {
			m.systemLine("invalid channel name: " + c)
			continue
		}
		channels = append(channels, c)
	}
	if len(channels) == 0 {
		return true
	}
	if err := m.client.Join(channels...); err != nil {
		m.systemLine("join failed: " + err.Error())
	}
	if _, idx := m.addBuffer(&Buffer{Name: channels[0]}); idx >= 0 {
		m.active = idx
	}
	return true
}

func (m *Model) cmdPart(rest string) bool {
	target := m.CurrentTarget()
	if target == "" {
		m.systemLine("nothing to part")
		return true
	}
	if err := m.client.Part(target, rest); err != nil {
		m.systemLine("part failed: " + err.Error())
	}
	m.removeBuffer(target)
	return true
}

func (m *Model) cmdMsg(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		m.systemLine("usage: /msg <nick|#chan> [text]")
		return true
	}
	target, text, _ := strings.Cut(rest, " ")
	target = strings.TrimSpace(target)
	text = strings.TrimSpace(text)
	if !irc.IsValidChannelName(target) && !irc.IsValidNick(target) {
		m.systemLine("invalid target: " + target)
		return true
	}
	// Always open/focus a buffer for the target — `/msg alice` works as
	// "open a query window with alice" even without text.
	b := m.findBuffer(target)
	for i, ob := range m.order {
		if ob == b {
			m.active = i
			break
		}
	}
	if text == "" {
		return true
	}
	if err := m.client.Privmsg(target, text); err != nil {
		m.systemLine("msg failed: " + err.Error())
		return true
	}
	b.Append(Line{Time: time.Now(), Kind: LineSelf, Nick: m.client.Nick(), Text: text})
	return true
}

func (m *Model) cmdAction(rest string) bool {
	target := m.CurrentTarget()
	if target == "" || rest == "" {
		m.systemLine("usage: /me <action> (in a channel)")
		return true
	}
	if err := m.client.Action(target, rest); err != nil {
		m.systemLine("action failed: " + err.Error())
		return true
	}
	if b := m.CurrentBuffer(); b != nil {
		b.Append(Line{Time: time.Now(), Kind: LineAction, Nick: m.client.Nick(), Text: rest})
	}
	return true
}

func (m *Model) cmdNames(rest string) bool {
	target := strings.TrimSpace(rest)
	if target == "" {
		target = m.CurrentTarget()
	}
	if !irc.IsValidChannelName(target) {
		m.systemLine("usage: /names [#channel]")
		return true
	}
	b := m.findBuffer(target)
	if len(b.Names) == 0 {
		if err := m.client.Names(target); err != nil {
			m.systemLine("names failed: " + err.Error())
			return true
		}
		m.pendingNamesShow[strings.ToLower(target)] = struct{}{}
		m.systemLine("names: requesting list for " + target)
		return true
	}
	m.openNamesOverlay(target)
	return true
}

func (m *Model) cmdTopic(rest string) bool {
	target := m.CurrentTarget()
	if target == "" {
		m.systemLine("not in a channel")
		return true
	}
	if rest == "" {
		b := m.findBuffer(target)
		if b.Topic == "" {
			m.systemLine("no topic set for " + target)
		} else {
			m.systemLine("topic: " + b.Topic)
		}
		return true
	}
	if err := m.client.Send("TOPIC", target, rest); err != nil {
		m.systemLine("topic failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdWhois(rest string) bool {
	nick := strings.TrimSpace(rest)
	if !irc.IsValidNick(nick) {
		m.systemLine("usage: /whois <nick>")
		return true
	}
	if err := m.client.Whois(nick); err != nil {
		m.systemLine("whois failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdWhowas(rest string) bool {
	nick := strings.TrimSpace(rest)
	if !irc.IsValidNick(nick) {
		m.systemLine("usage: /whowas <nick>")
		return true
	}
	if err := m.client.Whowas(nick); err != nil {
		m.systemLine("whowas failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdWho(rest string) bool {
	target := strings.TrimSpace(rest)
	if target == "" {
		target = m.CurrentTarget()
	}
	if target == "" {
		m.systemLine("usage: /who <#chan|nick>")
		return true
	}
	if err := m.client.Who(target); err != nil {
		m.systemLine("who failed: " + err.Error())
	}
	return true
}

// cmdKick accepts either "/kick <nick> [reason]" (in a channel) or
// "/kick <#chan> <nick> [reason]" (anywhere).
func (m *Model) cmdKick(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		m.systemLine("usage: /kick [#chan] <nick> [reason]")
		return true
	}
	parts := strings.SplitN(rest, " ", 3)
	var channel, nick, reason string
	if irc.IsValidChannelName(parts[0]) {
		channel = parts[0]
		if len(parts) < 2 {
			m.systemLine("usage: /kick <#chan> <nick> [reason]")
			return true
		}
		nick = parts[1]
		if len(parts) > 2 {
			reason = parts[2]
		}
	} else {
		channel = m.CurrentTarget()
		if !irc.IsValidChannelName(channel) {
			m.systemLine("/kick must be run in a channel or include one")
			return true
		}
		nick = parts[0]
		if len(parts) > 1 {
			reason = strings.Join(parts[1:], " ")
		}
	}
	if err := m.client.Kick(channel, nick, reason); err != nil {
		m.systemLine("kick failed: " + err.Error())
	}
	return true
}

// cmdMode passes args through; with no args, queries current mode of the
// active channel. With a leading +/- argument and no target, applies to
// the active channel.
func (m *Model) cmdMode(rest string) bool {
	rest = strings.TrimSpace(rest)
	target := ""
	args := []string(nil)
	if rest == "" {
		target = m.CurrentTarget()
		if target == "" {
			m.systemLine("usage: /mode [#chan] [+modes]")
			return true
		}
	} else {
		fields := strings.Fields(rest)
		if irc.IsValidChannelName(fields[0]) || irc.IsValidNick(fields[0]) {
			target = fields[0]
			args = fields[1:]
		} else {
			target = m.CurrentTarget()
			args = fields
		}
	}
	if target == "" {
		m.systemLine("/mode needs a target")
		return true
	}
	if err := m.client.Mode(target, args...); err != nil {
		m.systemLine("mode failed: " + err.Error())
	}
	return true
}

// cmdOpVoice toggles +o/-o/+v/-v on a nick in the current channel.
func (m *Model) cmdOpVoice(rest, mode string) bool {
	nick := strings.TrimSpace(rest)
	if !irc.IsValidNick(nick) {
		m.systemLine("usage: /" + modeHumanName(mode) + " <nick>")
		return true
	}
	channel := m.CurrentTarget()
	if !irc.IsValidChannelName(channel) {
		m.systemLine("must be run in a channel")
		return true
	}
	if err := m.client.Mode(channel, mode, nick); err != nil {
		m.systemLine("mode failed: " + err.Error())
	}
	return true
}

func modeHumanName(m string) string {
	switch m {
	case "+o":
		return "op"
	case "-o":
		return "deop"
	case "+v":
		return "voice"
	case "-v":
		return "devoice"
	default:
		return "mode"
	}
}

// cmdBan applies +b or -b on a mask in the current channel. Bare nicks are
// expanded to "nick!*@*" so the user doesn't have to remember the syntax.
func (m *Model) cmdBan(rest, op string) bool {
	mask := strings.TrimSpace(rest)
	if mask == "" {
		m.systemLine("usage: /" + map[string]string{"+b": "ban", "-b": "unban"}[op] + " <nick|mask>")
		return true
	}
	if !strings.ContainsAny(mask, "!@*?") && irc.IsValidNick(mask) {
		mask = mask + "!*@*"
	}
	channel := m.CurrentTarget()
	if !irc.IsValidChannelName(channel) {
		m.systemLine("must be run in a channel")
		return true
	}
	if err := m.client.Mode(channel, op, mask); err != nil {
		m.systemLine("mode failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdInvite(rest string) bool {
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		m.systemLine("usage: /invite <nick> [#chan]")
		return true
	}
	nick := parts[0]
	channel := m.CurrentTarget()
	if len(parts) > 1 {
		channel = parts[1]
	}
	if !irc.IsValidNick(nick) || !irc.IsValidChannelName(channel) {
		m.systemLine("invalid invite arguments")
		return true
	}
	if err := m.client.Invite(nick, channel); err != nil {
		m.systemLine("invite failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdAway(rest string) bool {
	reason := strings.TrimSpace(rest)
	if err := m.client.Away(reason); err != nil {
		m.systemLine("away failed: " + err.Error())
		return true
	}
	if reason == "" {
		m.systemLine("away cleared")
	} else {
		m.systemLine("away: " + reason)
	}
	return true
}

func (m *Model) cmdNotice(rest string) bool {
	target, text, ok := strings.Cut(rest, " ")
	if !ok || strings.TrimSpace(text) == "" {
		m.systemLine("usage: /notice <nick|#chan> <text>")
		return true
	}
	target = strings.TrimSpace(target)
	if !irc.IsValidChannelName(target) && !irc.IsValidNick(target) {
		m.systemLine("invalid target: " + target)
		return true
	}
	text = sanitize.Text(text)
	if text == "" {
		return true
	}
	if err := m.client.Notice(target, text); err != nil {
		m.systemLine("notice failed: " + err.Error())
		return true
	}
	if b := m.CurrentBuffer(); b != nil {
		b.Append(Line{Time: time.Now(), Kind: LineNotice, Nick: m.client.Nick(), Text: "→ " + target + ": " + text})
	}
	return true
}

func (m *Model) cmdPing(rest string) bool {
	nick := strings.TrimSpace(rest)
	if !irc.IsValidNick(nick) {
		m.systemLine("usage: /ping <nick>")
		return true
	}
	if err := m.client.CTCPPing(nick); err != nil {
		m.systemLine("ping failed: " + err.Error())
		return true
	}
	m.systemLine("CTCP PING → " + nick)
	return true
}

func (m *Model) cmdRaw(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		m.systemLine("usage: /raw <COMMAND> [args...]")
		return true
	}
	parts := strings.Fields(rest)
	cmd := strings.ToUpper(parts[0])
	if !isRawCommandName(cmd) {
		m.systemLine("invalid raw command: " + parts[0])
		return true
	}
	if err := m.client.Send(cmd, parts[1:]...); err != nil {
		m.systemLine("raw failed: " + err.Error())
	}
	return true
}

// isRawCommandName accepts only IRC-shaped command tokens: 1-32 chars,
// uppercase letters or digits. This keeps /raw from smuggling control bytes
// or newlines onto the wire.
func isRawCommandName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func (m *Model) cmdNick(rest string) bool {
	if rest == "" {
		m.systemLine("usage: /nick <newnick>")
		return true
	}
	nick := strings.Fields(rest)[0]
	if !irc.IsValidNick(nick) {
		m.systemLine("invalid nickname: " + nick)
		return true
	}
	if err := m.client.SetNick(nick); err != nil {
		m.systemLine("nick failed: " + err.Error())
	}
	return true
}

func (m *Model) cmdTheme(rest string) bool {
	if rest == "" {
		m.overlay = OverlayThemes
		return true
	}
	for i, t := range m.themes {
		if strings.EqualFold(t.Name, rest) {
			m.applyThemeIdx(i)
			_ = config.SaveTheme(m.ThemeName())
			m.systemLine("theme: " + t.Name)
			return true
		}
	}
	m.systemLine("theme not found: " + rest + " (try /themes)")
	return true
}

// systemLine appends a system message to the active (or server) buffer.
func (m *Model) systemLine(text string) {
	b := m.CurrentBuffer()
	if b == nil {
		b = m.findBuffer("")
	}
	b.Append(Line{Time: time.Now(), Kind: LineSystem, Text: text})
}

// sendChat sends the input as a regular PRIVMSG to the active channel.
// Outgoing text is sanitized to defend against control-byte smuggling via
// pasted content that the input layer somehow let through.
func (m *Model) sendChat(text string) {
	target := m.CurrentTarget()
	if target == "" {
		m.systemLine("not in a channel")
		return
	}
	text = sanitize.Text(text)
	if text == "" {
		return
	}
	if err := m.client.Privmsg(target, text); err != nil {
		m.systemLine("send failed: " + err.Error())
		return
	}
	if b := m.CurrentBuffer(); b != nil {
		b.Append(Line{Time: time.Now(), Kind: LineSelf, Nick: m.client.Nick(), Text: text})
	}
}

func (m *Model) themeNames() []string {
	out := make([]string, len(m.themes))
	for i, t := range m.themes {
		out[i] = t.Name
	}
	return out
}
