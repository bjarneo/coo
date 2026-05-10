// Package config loads ~/.config/coo/config.toml and applies CLI overrides.
package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"coo/internal/appdir"
)

// Config is the resolved runtime configuration.
type Config struct {
	Server     string
	Port       int
	TLS        bool
	Insecure   bool
	Nick       string
	User       string
	Realname   string
	Channels   []string
	Theme      string
	SASL       bool
	NickServ   bool
	LogLevel   string
	LogFile    string
	QuitMsg    string
	Password   string // never persisted; set at runtime from prompt or stdin
}

// Default returns sane defaults.
func Default() Config {
	return Config{
		Port:     6697,
		TLS:      true,
		LogLevel: "info",
		QuitMsg:  "coo",
	}
}

// DefaultPath returns ~/.config/coo/config.toml.
func DefaultPath() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads a TOML config from path. If path is "" the default location is
// used. A missing file returns Default() with a nil error.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return cfg, err
		}
		path = p
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		applyKV(&cfg, section, key, val)
	}
	return cfg, scanner.Err()
}

func applyKV(cfg *Config, section, key, raw string) {
	full := key
	if section != "" {
		full = section + "." + key
	}
	switch full {
	case "server":
		cfg.Server = parseString(raw)
	case "port":
		if n, err := strconv.Atoi(strings.Trim(raw, `"'`)); err == nil {
			cfg.Port = n
		}
	case "tls":
		cfg.TLS = parseBool(raw, cfg.TLS)
	case "insecure":
		cfg.Insecure = parseBool(raw, cfg.Insecure)
	case "nick":
		cfg.Nick = parseString(raw)
	case "user":
		cfg.User = parseString(raw)
	case "realname":
		cfg.Realname = parseString(raw)
	case "channels":
		cfg.Channels = parseList(raw)
	case "theme":
		cfg.Theme = parseString(raw)
	case "sasl":
		cfg.SASL = parseBool(raw, cfg.SASL)
	case "log_level":
		cfg.LogLevel = parseString(raw)
	case "log_file":
		cfg.LogFile = parseString(raw)
	case "quit_msg":
		cfg.QuitMsg = parseString(raw)
	case "nickserv.enabled":
		cfg.NickServ = parseBool(raw, cfg.NickServ)
	}
}

func parseString(s string) string {
	return strings.Trim(s, `"'`)
}

func parseBool(s string, fallback bool) bool {
	switch strings.ToLower(strings.Trim(s, `"'`)) {
	case "true", "yes", "1", "on":
		return true
	case "false", "no", "0", "off":
		return false
	}
	return fallback
}

func parseList(s string) []string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.Trim(strings.TrimSpace(p), `"'`)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Overrides represents CLI flag values. A pointer value of nil means the flag
// was not provided. Apply mutates cfg in place.
type Overrides struct {
	Server   *string
	Port     *int
	TLS      *bool
	Insecure *bool
	Nick     *string
	User     *string
	Realname *string
	Channels []string
	Theme    *string
	SASL     *bool
	NickServ *bool
	LogLevel *string
	LogFile  *string
	QuitMsg  *string
}

// Apply merges flag values into cfg. Non-nil pointers and non-empty slices win.
func (o Overrides) Apply(cfg *Config) {
	if o.Server != nil {
		cfg.Server = *o.Server
	}
	if o.Port != nil {
		cfg.Port = *o.Port
	}
	if o.TLS != nil {
		cfg.TLS = *o.TLS
	}
	if o.Insecure != nil {
		cfg.Insecure = *o.Insecure
	}
	if o.Nick != nil {
		cfg.Nick = *o.Nick
	}
	if o.User != nil {
		cfg.User = *o.User
	}
	if o.Realname != nil {
		cfg.Realname = *o.Realname
	}
	if len(o.Channels) > 0 {
		cfg.Channels = mergeUnique(cfg.Channels, o.Channels)
	}
	if o.Theme != nil {
		cfg.Theme = *o.Theme
	}
	if o.SASL != nil {
		cfg.SASL = *o.SASL
	}
	if o.NickServ != nil {
		cfg.NickServ = *o.NickServ
	}
	if o.LogLevel != nil {
		cfg.LogLevel = *o.LogLevel
	}
	if o.LogFile != nil {
		cfg.LogFile = *o.LogFile
	}
	if o.QuitMsg != nil {
		cfg.QuitMsg = *o.QuitMsg
	}
	cfg.applyDerived()
}

func (cfg *Config) applyDerived() {
	if cfg.User == "" {
		cfg.User = cfg.Nick
	}
	if cfg.Realname == "" {
		cfg.Realname = cfg.Nick
	}
	cfg.Channels = normalizeChannels(cfg.Channels)
}

func normalizeChannels(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !strings.HasPrefix(c, "#") && !strings.HasPrefix(c, "&") {
			c = "#" + c
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

func mergeUnique(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Validate reports the first missing required field.
func (cfg Config) Validate() error {
	if cfg.Server == "" {
		return errors.New("server is required (--server or [server] in config)")
	}
	if cfg.Nick == "" {
		return errors.New("nick is required (--nick or nick in config)")
	}
	return nil
}

// SaveTheme writes only the theme key back to the config file, preserving
// other entries. The write is atomic: contents go to a temp file in the same
// directory, fsync'd, then renamed over the target so a crash mid-write
// can't leave a truncated config.
func SaveTheme(name string) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "theme") && strings.Contains(trimmed, "=") {
				continue
			}
			lines = append(lines, l)
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, "theme = "+strconv.Quote(name))

	return atomicWrite(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// atomicWrite writes data to a temp file in the same directory as path, fsyncs
// it, and renames it over path. Same-directory rename is atomic on POSIX.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".coo-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if rename succeeded
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
