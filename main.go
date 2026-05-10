// Command coo is a light Bubble Tea IRC client.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"coo/config"
	"coo/internal/applog"
	"coo/irc"
	"coo/theme"
	"coo/ui/model"
)

// version is set at build time via -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	app := buildApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "coo:", err)
		os.Exit(1)
	}
}

func buildApp() *cli.Command {
	return &cli.Command{
		Name:      "coo",
		Usage:     "light Bubble Tea IRC client",
		Version:   version,
		ArgsUsage: "[#channel ...]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "path to TOML config"},
			&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Usage: "IRC server hostname"},
			&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Usage: "IRC server port", Value: 6697},
			&cli.BoolFlag{Name: "tls", Usage: "use TLS", Value: true},
			&cli.BoolFlag{Name: "insecure", Usage: "skip TLS verification"},
			&cli.StringFlag{Name: "nick", Aliases: []string{"n"}, Usage: "nickname"},
			&cli.StringFlag{Name: "user", Usage: "ident (defaults to nick)"},
			&cli.StringFlag{Name: "realname", Usage: "GECOS / real name"},
			&cli.StringSliceFlag{Name: "channels", Usage: "comma-separated channels"},
			&cli.StringFlag{Name: "theme", Usage: "theme name (blank = terminal default)"},
			&cli.BoolFlag{Name: "sasl", Usage: "use SASL PLAIN (prompts for password)"},
			&cli.BoolFlag{Name: "sasl-password-stdin", Usage: "read SASL password from stdin"},
			&cli.BoolFlag{Name: "nickserv", Usage: "identify via NickServ (prompts for password)"},
			&cli.BoolFlag{Name: "nickserv-stdin", Usage: "read NickServ password from stdin"},
			&cli.StringFlag{Name: "log-level", Value: "info"},
			&cli.StringFlag{Name: "log-file"},
			&cli.StringFlag{Name: "quit-msg", Value: "coo"},
			&cli.BoolFlag{Name: "list-themes", Usage: "print available themes and exit"},
		},
		Action: runAction,
	}
}

func runAction(_ context.Context, c *cli.Command) error {
	if c.Bool("list-themes") {
		for _, t := range theme.LoadAll() {
			fmt.Println(t.Name)
		}
		return nil
	}

	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	overrides := buildOverrides(c)
	overrides.Channels = append(overrides.Channels, c.Args().Slice()...)
	overrides.Apply(&cfg)

	normalizeServerURL(&cfg)

	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d (expected 1-65535)", cfg.Port)
	}
	if !irc.IsValidNick(cfg.Nick) {
		return fmt.Errorf("invalid nickname: %q", cfg.Nick)
	}

	logCloser, err := applog.Init(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log file: %v (continuing without)\n", err)
	}
	defer logCloser.Close()
	slog.Info("coo starting", "version", version, "server", cfg.Server, "nick", cfg.Nick)

	// Password acquisition (must happen before tea.NewProgram takes the TTY).
	if c.Bool("sasl") {
		pw, err := readPassword("SASL password: ", c.Bool("sasl-password-stdin"))
		if err != nil {
			return err
		}
		cfg.SASL = true
		cfg.Password = pw
	}
	var nickservPw string
	if c.Bool("nickserv") || cfg.NickServ {
		pw, err := readPassword("NickServ password: ", c.Bool("nickserv-stdin"))
		if err != nil {
			return err
		}
		cfg.NickServ = true
		nickservPw = pw
	}

	client := irc.New(irc.Config{
		Server:       cfg.Server,
		Port:         cfg.Port,
		TLS:          cfg.TLS,
		Insecure:     cfg.Insecure,
		Nick:         cfg.Nick,
		User:         cfg.User,
		Realname:     cfg.Realname,
		SASL:         cfg.SASL,
		SASLPassword: cfg.Password,
		NickServ:     cfg.NickServ,
		NickServPass: nickservPw,
		QuitMsg:      cfg.QuitMsg,
		Version:      "coo/" + version,
	})
	// Register initial channels before Connect so the on-connect callback
	// joins them AFTER NickServ IDENTIFY / SASL completes — otherwise +r
	// channels reject the JOIN before login finishes.
	client.SetAutoJoin(cfg.Channels)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	themes := theme.LoadAll()
	m := model.New(client, themes, themeNameOrDefault(cfg.Theme))

	prog := tea.NewProgram(m)
	if _, err := prog.Run(); err != nil {
		return err
	}
	return nil
}

func buildOverrides(c *cli.Command) config.Overrides {
	o := config.Overrides{}
	if c.IsSet("server") {
		s := c.String("server")
		o.Server = &s
	}
	if c.IsSet("port") {
		p := c.Int("port")
		o.Port = &p
	}
	if c.IsSet("tls") {
		v := c.Bool("tls")
		o.TLS = &v
	}
	if c.IsSet("insecure") {
		v := c.Bool("insecure")
		o.Insecure = &v
	}
	if c.IsSet("nick") {
		s := c.String("nick")
		o.Nick = &s
	}
	if c.IsSet("user") {
		s := c.String("user")
		o.User = &s
	}
	if c.IsSet("realname") {
		s := c.String("realname")
		o.Realname = &s
	}
	if v := c.StringSlice("channels"); len(v) > 0 {
		o.Channels = append(o.Channels, splitCSV(v)...)
	}
	if c.IsSet("theme") {
		s := c.String("theme")
		o.Theme = &s
	}
	if c.IsSet("sasl") {
		v := c.Bool("sasl")
		o.SASL = &v
	}
	if c.IsSet("nickserv") {
		v := c.Bool("nickserv")
		o.NickServ = &v
	}
	if c.IsSet("log-level") {
		s := c.String("log-level")
		o.LogLevel = &s
	}
	if c.IsSet("log-file") {
		s := c.String("log-file")
		o.LogFile = &s
	}
	if c.IsSet("quit-msg") {
		s := c.String("quit-msg")
		o.QuitMsg = &s
	}
	return o
}

// splitCSV expands "#a,#b,#c" entries into separate elements so the user can
// pass either form: --channels "#a,#b" or --channels "#a" --channels "#b".
func splitCSV(in []string) []string {
	var out []string
	for _, s := range in {
		for _, p := range strings.Split(s, ",") {
			if v := strings.TrimSpace(p); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// readPassword reads a password from stdin (no echo) or from a piped stdin.
// Falls back to an error if neither is available.
func readPassword(prompt string, fromStdin bool) (string, error) {
	if fromStdin {
		var buf [4096]byte
		n, err := os.Stdin.Read(buf[:])
		if err != nil && n == 0 {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		return strings.TrimRight(string(buf[:n]), "\r\n"), nil
	}
	if !term.IsTerminal(int(syscall.Stdin)) {
		return "", fmt.Errorf("password required but stdin is not a terminal (use --*-stdin)")
	}
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

func themeNameOrDefault(name string) string {
	if name == "" {
		return theme.DefaultName
	}
	return name
}

// normalizeServerURL accepts either a bare host or a URL like irc://host:port
// or ircs://host:port and folds it into cfg.Server / cfg.Port / cfg.TLS.
// Surplus path components are ignored.
func normalizeServerURL(cfg *config.Config) {
	s := strings.TrimSpace(cfg.Server)
	switch {
	case strings.HasPrefix(s, "ircs://"):
		s = strings.TrimPrefix(s, "ircs://")
		cfg.TLS = true
	case strings.HasPrefix(s, "irc://"):
		s = strings.TrimPrefix(s, "irc://")
		cfg.TLS = false
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		s = host
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	cfg.Server = s
}
