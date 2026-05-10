package irc

import "time"

// Event types are sent to the UI as tea.Msg values via the Client's Events
// channel. They are plain Go structs so the UI never has to import ergochat.

type ConnectedMsg struct {
	Network string
	Nick    string
}

type DisconnectedMsg struct {
	Reason string
}

type ConnErrorMsg struct {
	Err error
}

type PrivmsgMsg struct {
	Time    time.Time
	From    string
	Target  string // channel or our nick (for PMs)
	Text    string
	IsAction bool
}

type NoticeMsg struct {
	Time   time.Time
	From   string
	Target string
	Text   string
}

type JoinMsg struct {
	Time    time.Time
	Nick    string
	Channel string
}

type PartMsg struct {
	Time    time.Time
	Nick    string
	Channel string
	Reason  string
}

type QuitMsg struct {
	Time   time.Time
	Nick   string
	Reason string
}

type NickChangeMsg struct {
	Time time.Time
	From string
	To   string
}

type TopicMsg struct {
	Time    time.Time
	Channel string
	Topic   string
	SetBy   string
}

// TopicWhoTimeMsg carries RPL_TOPICWHOTIME (333): who set the topic and when.
type TopicWhoTimeMsg struct {
	Time    time.Time
	Channel string
	SetBy   string
	SetAt   time.Time
}

// NamesMsg is one chunk of an RPL_NAMREPLY (353). Multiple may arrive per
// /names or join; NamesEndMsg signals completion.
type NamesMsg struct {
	Channel string
	Nicks   []string
}

// NamesEndMsg signals RPL_ENDOFNAMES (366) for a channel.
type NamesEndMsg struct {
	Channel string
}

// ServerLineMsg captures unhandled server output (numerics, MOTD, etc.) for
// display in the *server* buffer.
type ServerLineMsg struct {
	Time time.Time
	Text string
}

// InfoLineMsg is a sanitized, formatted response to a user-initiated query
// (WHOIS, WHO, LIST, AWAY, INVITE …). The model routes these to whichever
// buffer is active when the response arrives — same place the user typed
// the command, in the common case. Hostmasks are pre-redacted.
type InfoLineMsg struct {
	Time time.Time
	Text string
}

// KickedMsg is emitted on every KICK we observe.
type KickedMsg struct {
	Time    time.Time
	Channel string
	Kicker  string
	Victim  string
	Reason  string
	Self    bool // true if Victim is us
}

// ModeMsg captures channel- or user-mode changes.
type ModeMsg struct {
	Time   time.Time
	Setter string
	Target string
	Modes  string
}

// InvitedMsg fires when someone invites us to a channel.
type InvitedMsg struct {
	Time    time.Time
	From    string
	Channel string
}
