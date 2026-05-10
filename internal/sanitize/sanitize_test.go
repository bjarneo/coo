package sanitize

import "testing"

func TestText(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "hello world", "hello world"},
		{"strip ANSI CSI", "before\x1b[31mred\x1b[0mafter", "beforeredafter"},
		{"strip OSC bel", "title\x1b]0;evil\x07tail", "titletail"},
		{"strip OSC st", "x\x1b]52;c;evil\x1b\\y", "xy"},
		{"strip mIRC color short", "\x033red", "red"},
		{"strip mIRC color long", "\x0312,4hello", "hello"},
		{"strip mIRC bold", "be \x02bold\x02 here", "be bold here"},
		{"strip BEL", "ding\x07dong", "ding dong"},
		{"replace tab with space", "a\tb", "a b"},
		{"strip nul", "x\x00y", "x y"},
		{"strip CR/LF", "line1\r\nline2", "line1  line2"},
		{"preserve UTF-8", "café 日本語", "café 日本語"},
		{"empty", "", ""},
		{"trailing spaces preserved", "hi   ", "hi   "},
		{"single space", " ", " "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Text(tc.in); got != tc.want {
				t.Errorf("Text(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNick(t *testing.T) {
	cases := map[string]string{
		"alice":                   "alice",
		"alice!user@host":         "alice",
		"alice@host":              "alice",
		"bob!~user@1.2.3.4":       "bob",
		"":                        "",
		"alice\x1b[31m!user@x":    "alice",
		"alice\x02bold!user@host": "alicebold",
	}
	for in, want := range cases {
		if got := Nick(in); got != want {
			t.Errorf("Nick(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNickNeverLeaksHost(t *testing.T) {
	for _, in := range []string{
		"nick!user@evil.host.com",
		"nick!~ident@10.0.0.1",
		"nick!user@2001:db8::1",
		"foo!bar@baz",
	} {
		got := Nick(in)
		for _, leak := range []string{"@", "!", "host", "10.0.0", "2001"} {
			if contains(got, leak) {
				t.Errorf("Nick(%q) leaked %q in result %q", in, leak, got)
			}
		}
	}
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
