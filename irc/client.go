// Package irc wraps ergochat/irc-go and emits typed events for the UI.
package irc

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"

	"coo/internal/sanitize"
)

// Config carries the connection parameters.
type Config struct {
	Server       string
	Port         int
	TLS          bool
	Insecure     bool
	Nick         string
	User         string
	Realname     string
	SASL         bool
	SASLPassword string
	NickServ     bool
	NickServPass string
	QuitMsg      string
	Version      string // CTCP VERSION reply
}

// Client is a thin wrapper around ircevent.Connection that fans events into
// a single channel readable by the Bubble Tea program.
type Client struct {
	conn      *ircevent.Connection
	events    chan tea.Msg
	cfg       Config
	closeOnce sync.Once

	mu       sync.Mutex
	joined   map[string]struct{} // current optimistic membership
	autoJoin []string            // channels to (re)join on every connect
	quitting bool

	// authPending is true between sending NickServ IDENTIFY and receiving
	// confirmation (RPL_LOGGEDIN 900 or a "you are now identified"-style
	// notice). While true, channel JOINs are deferred to avoid +r refusal.
	authPending bool
}

// New constructs a client. Connect must be called separately.
func New(cfg Config) *Client {
	c := &Client{
		events: make(chan tea.Msg, 256),
		cfg:    cfg,
		joined: make(map[string]struct{}),
	}

	conn := &ircevent.Connection{
		Server:        net.JoinHostPort(cfg.Server, strconv.Itoa(cfg.Port)),
		Nick:          cfg.Nick,
		User:          cfg.User,
		RealName:      cfg.Realname,
		UseTLS:        cfg.TLS,
		QuitMessage:   cfg.QuitMsg,
		ReconnectFreq: 30 * time.Second,
		Timeout:       30 * time.Second,
		KeepAlive:     2 * time.Minute,
	}
	if cfg.TLS {
		tlsCfg := &tls.Config{ServerName: cfg.Server}
		if cfg.Insecure {
			tlsCfg.InsecureSkipVerify = true
		}
		conn.TLSConfig = tlsCfg
	}
	if cfg.SASL && cfg.SASLPassword != "" {
		conn.UseSASL = true
		conn.SASLLogin = cfg.Nick
		conn.SASLPassword = cfg.SASLPassword
	}
	c.conn = conn
	c.installCallbacks()
	return c
}

// Events returns the read end of the event channel.
func (c *Client) Events() <-chan tea.Msg { return c.events }

// Nick returns the client's current nickname.
func (c *Client) Nick() string {
	if c.conn == nil {
		return c.cfg.Nick
	}
	return c.conn.CurrentNick()
}

// Network returns the configured server hostname (without port).
func (c *Client) Network() string { return c.cfg.Server }

// Connect dials the server and starts the reader loop.
func (c *Client) Connect() error {
	if err := c.conn.Connect(); err != nil {
		return err
	}
	go c.conn.Loop()
	return nil
}

// Close cleanly disconnects from the server. Safe to call more than once;
// subsequent calls are no-ops, so deferred cleanup paths plus interactive
// /quit handlers don't fight each other.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.quitting = true
		c.mu.Unlock()
		if c.conn != nil {
			c.conn.Quit()
		}
	})
}

// safeCallback wraps an ircevent callback in a panic recovery so a malformed
// server line can't take down the network goroutine.
func safeCallback(name string, fn func(ircmsg.Message)) func(ircmsg.Message) {
	return func(m ircmsg.Message) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("irc callback panic",
					"callback", name,
					"recover", fmt.Sprint(r),
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn(m)
	}
}

// IsValidChannelName reports whether name is a valid IRC channel name. Used
// for input sanitization before issuing JOIN/PART/etc.
func IsValidChannelName(name string) bool {
	if len(name) < 2 || len(name) > 50 {
		return false
	}
	if c := name[0]; c != '#' && c != '&' && c != '+' && c != '!' {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == ' ' || c == ',' || c == 0x07 || c == '\n' || c == '\r' || c == 0 {
			return false
		}
	}
	return true
}

// IsValidNick reports whether nick is plausibly a usable IRC nickname. Used
// for /msg target sanity checks.
func IsValidNick(nick string) bool {
	if nick == "" || len(nick) > 50 {
		return false
	}
	for i := 0; i < len(nick); i++ {
		c := nick[i]
		if c <= 0x20 || c == ',' || c == '*' || c == '?' || c == '!' || c == '@' || c == '#' || c == ':' {
			return false
		}
	}
	return true
}

// Send dispatches a raw IRC command. Empty commands are silently ignored
// rather than returning an error; ergochat doesn't currently do this, so we
// guard here to keep the client robust against bad slash-command input.
func (c *Client) Send(command string, params ...string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	return c.conn.Send(command, params...)
}

// maxPrivmsgPayload is a conservative cap for the body of a PRIVMSG so the
// full line stays under IRC's 512-byte limit after the server prefix is
// added. 400 leaves headroom for ":nick!user@long.host PRIVMSG #channel :".
const maxPrivmsgPayload = 400

// Privmsg sends a chat message, splitting long bodies across several lines
// at word boundaries so the server never truncates them. Empty target or
// text is a silent no-op.
func (c *Client) Privmsg(target, text string) error {
	target = strings.TrimSpace(target)
	if target == "" || text == "" {
		return nil
	}
	for _, chunk := range splitMessage(text, maxPrivmsgPayload) {
		if chunk == "" {
			continue
		}
		if err := c.conn.Privmsg(target, chunk); err != nil {
			return err
		}
	}
	return nil
}

// Action sends a CTCP ACTION ("/me ..."). Long actions are split too; each
// chunk is wrapped in its own CTCP frame.
func (c *Client) Action(target, text string) error {
	target = strings.TrimSpace(target)
	if target == "" || text == "" {
		return nil
	}
	budget := maxPrivmsgPayload - len("\x01ACTION \x01")
	if budget < 1 {
		budget = 1
	}
	for _, chunk := range splitMessage(text, budget) {
		if chunk == "" {
			continue
		}
		if err := c.conn.Privmsg(target, "\x01ACTION "+chunk+"\x01"); err != nil {
			return err
		}
	}
	return nil
}

// splitMessage breaks text into chunks no longer than maxBytes, preferring
// space boundaries. A single word longer than maxBytes is hard-cut on a
// rune boundary so chunks are always valid UTF-8.
func splitMessage(text string, maxBytes int) []string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return []string{text}
	}
	var out []string
	for len(text) > maxBytes {
		cut := strings.LastIndexByte(text[:maxBytes], ' ')
		if cut <= 0 {
			cut = runeBoundary(text, maxBytes)
		}
		out = append(out, text[:cut])
		text = strings.TrimLeft(text[cut:], " ")
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

// runeBoundary returns the largest index ≤ limit that lies on a UTF-8 rune
// boundary, so callers slicing at that index never split a multi-byte rune.
func runeBoundary(s string, limit int) int {
	if limit >= len(s) {
		return len(s)
	}
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	if limit == 0 {
		return 1
	}
	return limit
}

// Join joins one or more channels and adds them to the auto-join list, so
// they're rejoined on reconnect. Invalid names are skipped silently.
func (c *Client) Join(channels ...string) error {
	for _, ch := range channels {
		if !IsValidChannelName(ch) {
			continue
		}
		c.addAutoJoin(ch)
		if err := c.conn.Join(ch); err != nil {
			return err
		}
	}
	return nil
}

// SetAutoJoin records channels to be joined on every successful connect,
// without sending any JOIN command now. Use this for channels that should
// be entered after authentication (NickServ IDENTIFY, SASL) so registered-
// only (+r) channels don't reject the JOIN before login completes.
func (c *Client) SetAutoJoin(channels []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoJoin = c.autoJoin[:0]
	for _, ch := range channels {
		if !IsValidChannelName(ch) {
			continue
		}
		dup := false
		for _, x := range c.autoJoin {
			if strings.EqualFold(x, ch) {
				dup = true
				break
			}
		}
		if !dup {
			c.autoJoin = append(c.autoJoin, ch)
		}
	}
}

// Part leaves a channel and removes it from the auto-join list.
func (c *Client) Part(channel, reason string) error {
	if !IsValidChannelName(channel) {
		return nil
	}
	c.removeAutoJoin(channel)
	if reason == "" {
		return c.conn.Part(channel)
	}
	return c.conn.Send("PART", channel, reason)
}

func (c *Client) addAutoJoin(ch string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, x := range c.autoJoin {
		if strings.EqualFold(x, ch) {
			return
		}
	}
	c.autoJoin = append(c.autoJoin, ch)
}

func (c *Client) removeAutoJoin(ch string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, x := range c.autoJoin {
		if strings.EqualFold(x, ch) {
			c.autoJoin = append(c.autoJoin[:i], c.autoJoin[i+1:]...)
			return
		}
	}
}

// SetNick changes nickname.
func (c *Client) SetNick(nick string) error {
	if !IsValidNick(nick) {
		return fmt.Errorf("invalid nickname: %q", nick)
	}
	return c.conn.Send("NICK", nick)
}

// Names asks the server to refresh the participant list for a channel.
func (c *Client) Names(channel string) error {
	if !IsValidChannelName(channel) {
		return nil
	}
	return c.conn.Send("NAMES", channel)
}

// Whois queries the server for nick metadata.
func (c *Client) Whois(nick string) error {
	if !IsValidNick(nick) {
		return fmt.Errorf("invalid nick: %q", nick)
	}
	return c.conn.Send("WHOIS", nick)
}

// Whowas queries historical nick info.
func (c *Client) Whowas(nick string) error {
	if !IsValidNick(nick) {
		return fmt.Errorf("invalid nick: %q", nick)
	}
	return c.conn.Send("WHOWAS", nick)
}

// Who queries channel membership / nick details.
func (c *Client) Who(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	return c.conn.Send("WHO", target)
}

// Kick removes nick from channel with an optional reason.
func (c *Client) Kick(channel, nick, reason string) error {
	if !IsValidChannelName(channel) || !IsValidNick(nick) {
		return fmt.Errorf("invalid channel/nick")
	}
	if reason == "" {
		return c.conn.Send("KICK", channel, nick)
	}
	return c.conn.Send("KICK", channel, nick, reason)
}

// Mode sends a MODE change for target. modes/params are passed through.
func (c *Client) Mode(target string, args ...string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	return c.conn.Send("MODE", append([]string{target}, args...)...)
}

// Invite invites nick to channel.
func (c *Client) Invite(nick, channel string) error {
	if !IsValidNick(nick) || !IsValidChannelName(channel) {
		return fmt.Errorf("invalid invite arguments")
	}
	return c.conn.Send("INVITE", nick, channel)
}

// Away sets your away status. Empty reason clears it.
func (c *Client) Away(reason string) error {
	if reason == "" {
		return c.conn.Send("AWAY")
	}
	return c.conn.Send("AWAY", reason)
}

// List requests the channel directory (optionally filtered).
func (c *Client) List(filter string) error {
	if filter == "" {
		return c.conn.Send("LIST")
	}
	return c.conn.Send("LIST", filter)
}

// Notice sends a NOTICE (split for line length, like Privmsg).
func (c *Client) Notice(target, text string) error {
	target = strings.TrimSpace(target)
	if target == "" || text == "" {
		return nil
	}
	for _, chunk := range splitMessage(text, maxPrivmsgPayload) {
		if chunk == "" {
			continue
		}
		if err := c.conn.Notice(target, chunk); err != nil {
			return err
		}
	}
	return nil
}

// CTCPPing sends a CTCP PING request to nick. Replies arrive as NOTICEs and
// are surfaced via the existing CTCP NOTICE handler path.
func (c *Client) CTCPPing(nick string) error {
	if !IsValidNick(nick) {
		return fmt.Errorf("invalid nick: %q", nick)
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	return c.conn.Privmsg(nick, "\x01PING "+stamp+"\x01")
}

// IdentifyNickServ sends the standard NickServ IDENTIFY command and starts
// a watchdog that flushes the autoJoin list if no auth confirmation arrives
// within authConfirmTimeout (so a network without 900 / silent NickServ
// doesn't strand initial JOINs forever).
func (c *Client) IdentifyNickServ(password string) error {
	if password == "" {
		return nil
	}
	if err := c.conn.Privmsg("NickServ", "IDENTIFY "+password); err != nil {
		return err
	}
	go c.authWatchdog()
	return nil
}

const authConfirmTimeout = 5 * time.Second

func (c *Client) authWatchdog() {
	time.Sleep(authConfirmTimeout)
	c.mu.Lock()
	pending := c.authPending
	c.authPending = false
	c.mu.Unlock()
	if pending {
		slog.Warn("nickserv: no auth confirmation in time; joining channels anyway",
			"timeout", authConfirmTimeout)
		c.flushAutoJoin()
	}
}

// shouldDeferJoins reports whether we should hold off on the initial JOIN
// burst because NickServ auth hasn't been confirmed yet.
func (c *Client) shouldDeferJoins() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authPending
}

// flushAutoJoin sends JOIN for every channel in the autoJoin list. Safe to
// call multiple times — duplicate JOINs are harmless (server replies with
// ERR_USERONCHANNEL or just no-ops).
func (c *Client) flushAutoJoin() {
	c.mu.Lock()
	list := append([]string(nil), c.autoJoin...)
	c.mu.Unlock()
	for _, ch := range list {
		_ = c.conn.Join(ch)
	}
}

// markAuthenticated transitions out of the deferred-join window and flushes
// the autoJoin list. Called by the 900 numeric handler and the NickServ
// notice matcher; idempotent.
func (c *Client) markAuthenticated() {
	c.mu.Lock()
	pending := c.authPending
	c.authPending = false
	c.mu.Unlock()
	if pending {
		c.flushAutoJoin()
	}
}

func (c *Client) installCallbacks() {
	conn := c.conn

	conn.AddConnectCallback(safeCallback("connect", func(_ ircmsg.Message) {
		c.mu.Lock()
		c.joined = make(map[string]struct{})
		c.authPending = c.cfg.NickServ && c.cfg.NickServPass != ""
		c.mu.Unlock()

		if c.cfg.NickServ && c.cfg.NickServPass != "" {
			_ = c.IdentifyNickServ(c.cfg.NickServPass)
		}
		// SASL is handled pre-001 by ergochat; if it's in use, auth is
		// already done. Joining is safe immediately. For NickServ-only,
		// joins are deferred until the auth confirmation handler fires.
		if !c.shouldDeferJoins() {
			c.flushAutoJoin()
		}
		c.send(ConnectedMsg{Network: c.cfg.Server, Nick: conn.CurrentNick()})
	}))

	conn.AddDisconnectCallback(safeCallback("disconnect", func(_ ircmsg.Message) {
		c.mu.Lock()
		quitting := c.quitting
		c.mu.Unlock()
		reason := "connection closed"
		if quitting {
			reason = "quit"
		}
		c.send(DisconnectedMsg{Reason: reason})
	}))

	add := func(code string, fn func(ircmsg.Message)) {
		conn.AddCallback(code, safeCallback(code, fn))
	}

	add("PRIVMSG", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		from := m.Nick()
		target := m.Params[0]
		text := m.Params[1]
		isAction := false
		switch {
		case strings.HasPrefix(text, "\x01ACTION ") && strings.HasSuffix(text, "\x01"):
			text = strings.TrimSuffix(strings.TrimPrefix(text, "\x01ACTION "), "\x01")
			isAction = true
		case strings.HasPrefix(text, "\x01") && strings.HasSuffix(text, "\x01"):
			c.handleCTCP(from, strings.Trim(text, "\x01"))
			return
		case strings.HasPrefix(text, "\x01"):
			return
		}
		c.send(PrivmsgMsg{
			Time: time.Now(), From: sanitize.Nick(from), Target: target,
			Text: sanitize.Text(text), IsAction: isAction,
		})
	})

	add("NOTICE", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		from := sanitize.Nick(m.Nick())
		text := sanitize.Text(m.Params[1])
		if strings.EqualFold(from, "NickServ") && isAuthSuccessNotice(text) {
			c.markAuthenticated()
		}
		c.send(NoticeMsg{
			Time: time.Now(), From: from,
			Target: m.Params[0], Text: text,
		})
	})

	// 900 RPL_LOGGEDIN: services confirmation that we're authenticated.
	add("900", func(m ircmsg.Message) { c.markAuthenticated() })

	add("JOIN", func(m ircmsg.Message) {
		if len(m.Params) < 1 {
			return
		}
		ch := m.Params[0]
		nick := sanitize.Nick(m.Nick())
		if nick == conn.CurrentNick() {
			c.mu.Lock()
			c.joined[strings.ToLower(ch)] = struct{}{}
			c.mu.Unlock()
		}
		c.send(JoinMsg{Time: time.Now(), Nick: nick, Channel: ch})
	})

	add("PART", func(m ircmsg.Message) {
		if len(m.Params) < 1 {
			return
		}
		ch := m.Params[0]
		reason := ""
		if len(m.Params) > 1 {
			reason = sanitize.Text(m.Params[1])
		}
		nick := sanitize.Nick(m.Nick())
		if nick == conn.CurrentNick() {
			c.mu.Lock()
			delete(c.joined, strings.ToLower(ch))
			c.mu.Unlock()
		}
		c.send(PartMsg{Time: time.Now(), Nick: nick, Channel: ch, Reason: reason})
	})

	add("QUIT", func(m ircmsg.Message) {
		reason := ""
		if len(m.Params) > 0 {
			reason = sanitize.Text(m.Params[0])
		}
		// Some servers leak host/IP in the QUIT reason ("Connection reset
		// by peer (10.x.x.x)"). Drop anything in parentheses to be safe.
		reason = stripParenthetical(reason)
		c.send(QuitMsg{Time: time.Now(), Nick: sanitize.Nick(m.Nick()), Reason: reason})
	})

	add("NICK", func(m ircmsg.Message) {
		if len(m.Params) < 1 {
			return
		}
		c.send(NickChangeMsg{
			Time: time.Now(),
			From: sanitize.Nick(m.Nick()),
			To:   sanitize.Nick(m.Params[0]),
		})
	})

	add("TOPIC", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.send(TopicMsg{
			Time: time.Now(), Channel: m.Params[0],
			Topic: sanitize.Text(m.Params[1]), SetBy: sanitize.Nick(m.Nick()),
		})
	})

	// 332 RPL_TOPIC: <our_nick> <channel> :<topic>
	add("332", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.send(TopicMsg{
			Time: time.Now(), Channel: m.Params[1], Topic: sanitize.Text(m.Params[2]),
		})
	})

	// 331 RPL_NOTOPIC: <our_nick> <channel> :No topic is set
	add("331", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.send(TopicMsg{Time: time.Now(), Channel: m.Params[1], Topic: ""})
	})

	// 333 RPL_TOPICWHOTIME: <our_nick> <channel> <setby> <unixtime>
	add("333", func(m ircmsg.Message) {
		if len(m.Params) < 4 {
			return
		}
		setBy := sanitize.Nick(m.Params[2])
		ts, _ := strconv.ParseInt(m.Params[3], 10, 64)
		var setAt time.Time
		if ts > 0 {
			setAt = time.Unix(ts, 0)
		}
		c.send(TopicWhoTimeMsg{
			Time: time.Now(), Channel: m.Params[1],
			SetBy: setBy, SetAt: setAt,
		})
	})

	// 353 RPL_NAMREPLY: <nick> = <channel> :<nick> <nick> ...
	add("353", func(m ircmsg.Message) {
		if len(m.Params) < 4 {
			return
		}
		ch := m.Params[2]
		raw := strings.Fields(m.Params[3])
		const maxNamesPerChunk = 1000
		if len(raw) > maxNamesPerChunk {
			raw = raw[:maxNamesPerChunk]
		}
		nicks := make([]string, 0, len(raw))
		for _, n := range raw {
			// Preserve mode prefixes (@, +, etc) but strip any !user@host
			// after parse, just in case a server gets cute.
			prefix := ""
			for len(n) > 0 && strings.ContainsRune("@+%&~!", rune(n[0])) {
				if n[0] == '!' {
					break
				}
				prefix += string(n[0])
				n = n[1:]
			}
			nicks = append(nicks, prefix+sanitize.Nick(n))
		}
		c.send(NamesMsg{Channel: ch, Nicks: nicks})
	})

	// KICK: <channel> <victim> [:reason]
	add("KICK", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		reason := ""
		if len(m.Params) > 2 {
			reason = sanitize.Text(m.Params[2])
		}
		victim := sanitize.Nick(m.Params[1])
		c.send(KickedMsg{
			Time:    time.Now(),
			Channel: m.Params[0],
			Kicker:  sanitize.Nick(m.Nick()),
			Victim:  victim,
			Reason:  reason,
			Self:    victim == conn.CurrentNick(),
		})
	})

	// MODE: <target> <modes> [params...]
	add("MODE", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.send(ModeMsg{
			Time:   time.Now(),
			Setter: sanitize.Nick(m.Nick()),
			Target: m.Params[0],
			Modes:  strings.Join(m.Params[1:], " "),
		})
	})

	// INVITE <target> <channel>
	add("INVITE", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		// Only surface invites addressed to us.
		if !strings.EqualFold(m.Params[0], conn.CurrentNick()) {
			return
		}
		c.send(InvitedMsg{
			Time:    time.Now(),
			From:    sanitize.Nick(m.Nick()),
			Channel: m.Params[1],
		})
	})

	// 301 RPL_AWAY: <ournick> <nick> :<away message>
	add("301", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.info("%s is away: %s", sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[2]))
	})

	// 305/306 you are no longer/now away
	add("305", func(m ircmsg.Message) { c.info("you are back") })
	add("306", func(m ircmsg.Message) { c.info("you are now away") })

	// 311 RPL_WHOISUSER: <ournick> <nick> <user> <host> * :<realname>
	// We deliberately drop user@host fields and emit only nick + realname.
	add("311", func(m ircmsg.Message) {
		if len(m.Params) < 6 {
			return
		}
		c.info("whois: %s — %s", sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[5]))
	})

	// 312 RPL_WHOISSERVER: <ournick> <nick> <server> :<info>
	add("312", func(m ircmsg.Message) {
		if len(m.Params) < 4 {
			return
		}
		c.info("whois: %s connected via %s (%s)",
			sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[2]), sanitize.Text(m.Params[3]))
	})

	// 313 RPL_WHOISOPERATOR
	add("313", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.info("whois: %s is an IRC operator", sanitize.Nick(m.Params[1]))
	})

	// 317 RPL_WHOISIDLE: <ournick> <nick> <secs> [<signon>] :seconds idle …
	add("317", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		secs, _ := strconv.Atoi(m.Params[2])
		c.info("whois: %s idle %s", sanitize.Nick(m.Params[1]), formatDuration(secs))
	})

	// 318 RPL_ENDOFWHOIS
	add("318", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.info("whois: end of /whois for %s", sanitize.Nick(m.Params[1]))
	})

	// 319 RPL_WHOISCHANNELS: <ournick> <nick> :<channels>
	add("319", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.info("whois: %s on %s", sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[2]))
	})

	// 330 RPL_WHOISACCOUNT: <ournick> <nick> <account> :is logged in as
	add("330", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.info("whois: %s account %s", sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[2]))
	})

	// 671 RPL_WHOISSECURE
	add("671", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.info("whois: %s is using a secure connection", sanitize.Nick(m.Params[1]))
	})

	// 314 RPL_WHOWASUSER: same shape as 311 — drop user/host, keep nick + realname
	add("314", func(m ircmsg.Message) {
		if len(m.Params) < 6 {
			return
		}
		c.info("whowas: %s — %s", sanitize.Nick(m.Params[1]), sanitize.Text(m.Params[5]))
	})

	// 369 RPL_ENDOFWHOWAS
	add("369", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.info("whowas: end of /whowas for %s", sanitize.Nick(m.Params[1]))
	})

	// 352 RPL_WHOREPLY: <ournick> <chan> <user> <host> <server> <nick> <flags> :<hops> <realname>
	// Drop user, host, server. Show channel, nick, flags, realname.
	add("352", func(m ircmsg.Message) {
		if len(m.Params) < 8 {
			return
		}
		realname := sanitize.Text(m.Params[7])
		// strip leading "<hops> " from realname
		if i := strings.IndexByte(realname, ' '); i >= 0 {
			realname = realname[i+1:]
		}
		c.info("who: %s in %s [%s] — %s",
			sanitize.Nick(m.Params[5]), m.Params[1], sanitize.Text(m.Params[6]), realname)
	})

	add("315", func(m ircmsg.Message) { c.info("who: end of /who") })

	// 322 RPL_LIST: <ournick> <channel> <users> :<topic>
	add("322", func(m ircmsg.Message) {
		if len(m.Params) < 4 {
			return
		}
		c.info("list: %s (%s users) — %s",
			m.Params[1], m.Params[2], sanitize.Text(m.Params[3]))
	})

	add("323", func(m ircmsg.Message) { c.info("list: end of channel list") })

	// 341 RPL_INVITING: <ournick> <invitee> <channel>
	add("341", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.info("invited %s to %s", sanitize.Nick(m.Params[1]), m.Params[2])
	})

	// 401 ERR_NOSUCHNICK: <ournick> <nick> :No such nick/channel
	add("401", func(m ircmsg.Message) {
		if len(m.Params) < 3 {
			return
		}
		c.info("no such nick/channel: %s", sanitize.Text(m.Params[1]))
	})

	// 366 RPL_ENDOFNAMES: <our_nick> <channel> :End of /NAMES list.
	add("366", func(m ircmsg.Message) {
		if len(m.Params) < 2 {
			return
		}
		c.send(NamesEndMsg{Channel: m.Params[1]})
	})

	// MOTD and other server numerics → server buffer
	for _, code := range []string{"001", "002", "003", "004", "005", "251", "252", "253", "254", "255", "265", "266", "372", "375", "376", "NOTICE"} {
		_ = code
	}
	add("372", func(m ircmsg.Message) { c.serverLine(m) })
	add("375", func(m ircmsg.Message) { c.serverLine(m) })
	add("376", func(m ircmsg.Message) { c.serverLine(m) })
	add("001", func(m ircmsg.Message) { c.serverLine(m) })
	add("002", func(m ircmsg.Message) { c.serverLine(m) })
	add("003", func(m ircmsg.Message) { c.serverLine(m) })
	add("004", func(m ircmsg.Message) { c.serverLine(m) })
}

// handleCTCP responds to non-ACTION CTCP queries from from with the
// standard NOTICE-wrapped CTCP reply pattern.
func (c *Client) handleCTCP(from, body string) {
	cmd, arg, _ := strings.Cut(body, " ")
	cmd = strings.ToUpper(cmd)
	var reply string
	switch cmd {
	case "VERSION":
		v := c.cfg.Version
		if v == "" {
			v = "coo"
		}
		reply = "VERSION " + v
	case "PING":
		reply = "PING " + arg
	case "TIME":
		reply = "TIME " + time.Now().Format(time.RFC1123)
	case "CLIENTINFO":
		reply = "CLIENTINFO ACTION VERSION PING TIME CLIENTINFO"
	default:
		return
	}
	_ = c.conn.Send("NOTICE", from, "\x01"+reply+"\x01")
}

// info posts a formatted, sanitized response line to the UI as an
// InfoLineMsg. Used for query-response numerics so the UI routes them to
// the active buffer at delivery time.
func (c *Client) info(format string, args ...any) {
	c.send(InfoLineMsg{Time: time.Now(), Text: fmt.Sprintf(format, args...)})
}

// formatDuration converts a seconds count into a compact human string:
// "45s", "12m 03s", "2h 14m", "3d 7h".
func formatDuration(secs int) string {
	if secs < 0 {
		secs = 0
	}
	d := secs / 86400
	h := (secs % 86400) / 3600
	mi := (secs % 3600) / 60
	s := secs % 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, mi)
	case mi > 0:
		return fmt.Sprintf("%dm %02ds", mi, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func (c *Client) serverLine(m ircmsg.Message) {
	parts := m.Params
	if len(parts) > 1 {
		parts = parts[1:] // drop the target (our nick)
	}
	text := sanitize.Text(strings.Join(parts, " "))
	c.send(ServerLineMsg{Time: time.Now(), Text: text})
}

// isAuthSuccessNotice matches phrases that NickServ implementations emit on
// successful identification. Atheme: "You are now identified for ...";
// Anope: "Password accepted - you are now recognized"; ircu/srvx: "You are
// now logged in as ...". Loose substring match keeps this resilient to
// punctuation/casing tweaks across networks.
func isAuthSuccessNotice(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "now identified") ||
		strings.Contains(low, "now recognized") ||
		strings.Contains(low, "now logged in") ||
		strings.Contains(low, "password accepted")
}

// stripParenthetical removes parenthetical clauses, used to elide host/IP
// info that some servers append to QUIT reasons. "Closing link (1.2.3.4)"
// becomes "Closing link".
func stripParenthetical(s string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteByte(s[i])
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func (c *Client) send(msg tea.Msg) {
	select {
	case c.events <- msg:
	default:
		// drop on overflow rather than block the IRC reader
	}
}
