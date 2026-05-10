# Configuration

`coo` reads `~/.config/coo/config.toml` by default; CLI flags override TOML values. A starter config is included at the repo root: `config.toml.example`.

## CLI flags

```
coo [flags] [#channel ...]

  --config, -c PATH        Path to TOML config (default: ~/.config/coo/config.toml)
  --server, -s HOST        IRC server hostname or `irc://` / `ircs://` URL
  --port, -p PORT          Default 6697
  --tls / --no-tls         Default true
  --insecure               Skip TLS verification (for self-signed certs)
  --nick, -n NAME          Nickname (required)
  --user USER              Ident; defaults to nick
  --realname STR           GECOS string; defaults to nick
  --channels LIST          Comma-separated channels (positional args also work)
  --theme NAME             Theme name; blank means follow terminal palette
  --list-themes            Print available themes and exit
  --sasl                   Use SASL PLAIN; prompts for password
  --sasl-password-stdin    Read SASL password from stdin instead of TTY
  --nickserv               Identify via NickServ; prompts for password
  --nickserv-stdin         Read NickServ password from stdin
  --log-level LEVEL        debug | info | warn | error
  --log-file PATH          Write logs to a file (no console output by default)
  --quit-msg STR           Message sent on QUIT (default "coo")
  --version, -v            Print version
  --help, -h               Show help
```

Positional args after the flags are appended to `--channels`, with a leading `#` added if missing. Both forms work:

```bash
coo --channels '#archlinux,#go-nuts'
coo '#archlinux' '#go-nuts'
coo -s irc.libera.chat -n yourname '#archlinux'
```

## TOML config

```toml
server   = "irc.libera.chat"
port     = 6697
tls      = true
nick     = "yourname"
realname = "Your Name"
channels = ["#archlinux", "#go-nuts"]

# Blank or omitted = follow the terminal's ANSI palette.
theme = "tokyo-night"

sasl = false

[nickserv]
enabled = false

log_level = "info"
# log_file = "/tmp/coo.log"
```

The config parser is a flat key=value reader with `[section]` headers. Quotes around strings are optional and stripped. There are no struct tags or schema validators; unknown keys are silently ignored.

## Server URLs

`--server` accepts plain hosts and IRC URLs:

```
irc.libera.chat                 # bare host (uses --port)
irc.libera.chat:6697            # host:port
ircs://irc.libera.chat:6697     # forces TLS
irc://localhost:6667            # forces no TLS
[2001:db8::1]:6697              # IPv6 with port
```

URL scheme overrides `--tls`. Path segments after the host are ignored.

## Passwords

Passwords are never written to disk by `coo`. They're prompted at startup with no echo, kept only in memory, and zeroed implicitly when the process exits.

For scripted use, both flags accept a stdin alternative:

```bash
echo "$NICKSERV_PASSWORD" | coo --nickserv --nickserv-stdin --server irc.libera.chat --nick yourname
```

The stdin variants read until EOF and trim trailing newlines.

## Channels and reconnect

Channels passed at startup are registered into an autojoin list before the connection opens. The autojoin list is replayed on every successful (re)connect, after NickServ identification or SASL completes.

If NickServ is in use, JOINs are deferred until one of three signals fires:

1. `900 RPL_LOGGEDIN` (IRCv3 services confirmation)
2. A NickServ NOTICE matching "now identified", "now recognized", "now logged in", or "password accepted"
3. A 5-second timeout fallback (logged at warn level)

This avoids the race where `+r` channels reject early JOINs before NickServ has propagated the identification.

When you `/join` or `/part` at runtime, the autojoin list updates accordingly.

## Logging

By default, `coo` writes nothing to disk and nothing to the console (the TUI takes over the terminal). Pass `--log-file /tmp/coo.log` to capture diagnostics: connect/disconnect events, callback panics that were recovered, and dropped events under burst.

`--log-level debug` includes ergochat protocol traces.

## Theme persistence

When you change the theme at runtime via `/theme NAME` or the picker (`Ctrl+T`), the chosen name is written back to `~/.config/coo/config.toml` atomically (temp file + rename, no risk of a half-written config).

Setting `theme = ""` reverts to the terminal's ANSI palette.
