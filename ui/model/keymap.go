package model

// Binding is one row in the keymap help.
type Binding struct {
	Keys string
	Desc string
}

// Group bundles related bindings under a heading.
type Group struct {
	Title    string
	Bindings []Binding
}

// Keymap is the single source of truth for both the help overlay and the
// bottom hint strip.
var Keymap = []Group{
	{"Navigation", []Binding{
		{"Ctrl+N / Ctrl+P", "next / previous channel"},
		{"Alt+1..9", "jump to channel by index"},
		{"PgUp / PgDn", "scroll buffer"},
		{"Home / End", "jump to top / bottom of buffer"},
	}},
	{"Channels", []Binding{
		{"Ctrl+W", "close current channel (PART)"},
		{"Ctrl+L", "clear current buffer"},
		{"Tab", "complete nickname"},
	}},
	{"App", []Binding{
		{"?", "toggle this help"},
		{"Ctrl+T", "theme picker"},
		{"Esc", "close overlay"},
		{"Ctrl+C / /quit", "quit"},
	}},
	{"Messaging", []Binding{
		{"/msg nick [text]", "private message (no text = open buffer)"},
		{"/me action", "CTCP ACTION in current channel"},
		{"/notice <t> <text>", "send a NOTICE"},
		{"/ping <nick>", "CTCP PING a user"},
	}},
	{"Channels", []Binding{
		{"/join #chan", "join a channel"},
		{"/part [reason]", "leave the current channel"},
		{"/topic [text]", "show or set channel topic"},
		{"/names [#chan]", "list channel participants"},
		{"/list [filter]", "list server channels"},
		{"/invite <nick>", "invite to current channel"},
	}},
	{"Lookup", []Binding{
		{"/whois <nick>", "user info (host/IP redacted)"},
		{"/whowas <nick>", "historical user info"},
		{"/who <target>", "channel/user listing"},
	}},
	{"Operators", []Binding{
		{"/kick <nick> [reason]", "kick from current channel"},
		{"/ban <nick|mask>", "+b in current channel"},
		{"/unban <mask>", "-b in current channel"},
		{"/op <nick> · /deop", "grant/revoke +o"},
		{"/voice · /devoice", "grant/revoke +v"},
		{"/mode [target] [+x]", "set channel/user mode"},
	}},
	{"Self", []Binding{
		{"/nick name", "change nickname"},
		{"/away [reason]", "set away (blank = back)"},
		{"/back", "clear away"},
		{"/theme name", "switch theme (or /themes)"},
		{"/quit [reason]", "disconnect and exit"},
	}},
}

