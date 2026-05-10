package main

import (
	"testing"

	"coo/config"
)

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct {
		in       string
		port     int
		wantHost string
		wantPort int
		wantTLS  bool
	}{
		{"irc.libera.chat", 6697, "irc.libera.chat", 6697, true},
		{"ircs://irc.libera.chat:6697", 6697, "irc.libera.chat", 6697, true},
		{"irc://localhost:6667", 6697, "localhost", 6667, false},
		{"ircs://example.com", 6697, "example.com", 6697, true},
		{"host.example:7000", 6697, "host.example", 7000, true},
		{"ircs://host:6697/channel/path", 6697, "host", 6697, true},
		{"[2001:db8::1]:6697", 6697, "2001:db8::1", 6697, true},
		{"  irc://server.x  ", 6697, "server.x", 6697, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			cfg := config.Config{Server: tc.in, Port: tc.port, TLS: true}
			normalizeServerURL(&cfg)
			if cfg.Server != tc.wantHost {
				t.Errorf("server = %q, want %q", cfg.Server, tc.wantHost)
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("port = %d, want %d", cfg.Port, tc.wantPort)
			}
			if cfg.TLS != tc.wantTLS {
				t.Errorf("tls = %v, want %v", cfg.TLS, tc.wantTLS)
			}
		})
	}
}
