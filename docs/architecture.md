# Architecture

A short tour for contributors and curious readers.

## Layout

```
coo/
  main.go                     CLI parsing, password prompt, program wiring
  config/                     TOML loader + flag overrides
  internal/
    appdir/                   ~/.config/coo path resolver
    applog/                   slog wiring (file output, no console clutter)
    sanitize/                 ANSI/control-byte stripping for server input
  irc/
    client.go                 ergochat wrapper; emits typed events as tea.Msg
    events.go                 Event struct definitions (PrivmsgMsg, JoinMsg, ...)
  theme/
    theme.go                  Theme loading from embed + user dir
    themes/*.toml             Builtin themes
  ui/
    styles.go                 Color slots; ApplyTheme rebinds them
    components/
      strip.go                Channel tab strip with click ranges
      buffer.go               Buffer rendering + wrap
      overlay.go              Keymap, ThemePicker, NamesList overlays
    model/                    Bubble Tea Model
      model.go                State, helpers
      update.go               Update(): keys, mouse, IRC events
      view.go                 View(): composes the screen
      commands.go             Slash command dispatch
      input.go                Single-line text input with sanitize.Insert
      buffer.go               Per-channel ring of lines
      keymap.go               Help text source of truth
```

The dependency direction is one-way: `main` -> `ui/model` -> `ui/components` -> `ui/styles`, with `irc`, `config`, `theme`, and `internal/*` all leaf packages. No cycles.

## Event flow

ergochat runs the network reader on its own goroutine. Every message it parses is passed to a callback registered via `AddCallback`. The callbacks live in `irc/client.go` and are wrapped with `safeCallback`, which adds a deferred `recover` and an `slog.Error` so a malformed server line can't kill the network goroutine.

Each callback translates the IRC line into a typed event struct (`PrivmsgMsg`, `JoinMsg`, ...) and pushes it onto a buffered channel (cap 256, drops on overflow). The Bubble Tea program subscribes to that channel via a `tea.Cmd` that recursively re-subscribes after each delivery, so the IRC reader and the UI never block each other.

```
ergochat goroutine        UI goroutine
       |                       |
   server line                 |
       v                       |
  AddCallback fn               |
       |                       |
   typed event ----------> events channel
                                 |
                            tea.Cmd reads
                                 |
                            Update(msg)
                                 |
                              View()
                                 |
                              terminal
```

## State model

`ui/model.Model` holds the entire UI state. The fields most worth knowing:

- `client` is the IRC client.
- `buffers` is keyed by lowercase name; `order` is `[]*Buffer` for the strip and switching order.
- `active` is the index into `order`.
- `input` is a minimal text editor with sanitization on `Insert`.
- `overlay` is one of `OverlayNone | OverlayKeymap | OverlayThemes | OverlayNames`.
- `tabHits` and `namesHits` are click target lists, refreshed every render. Mouse handlers do an x/y range check against them, no layout math.
- `themes` and `themeIdx` drive the theme picker. The active theme name is derived from the index, never duplicated as state.

Buffers cap at 5000 lines. When a new line is appended while the user has scrolled away from the bottom, `ScrollOff` is bumped so the visible window stays anchored to the same content (approximate for wrapped rows; good enough for chat).

## Security model

`coo` treats every byte from the server as untrusted.

### Sanitization

`internal/sanitize/sanitize.go` strips, before display:

- ANSI CSI escape sequences (`ESC [ ... final-byte`)
- ANSI OSC sequences (`ESC ] ... BEL` or `ESC ] ... ESC \`)
- mIRC color codes (`\x03 [<fg>][,<bg>]`) and formatting (`\x02 \x0F \x11 \x16 \x1D \x1E \x1F`)
- BEL (`\x07`) and other C0 controls (incl. NUL/CR/LF/tab) replaced with space
- Invalid UTF-8 bytes replaced with `?`

Spaces (including trailing) and printable Unicode are preserved.

### Hostmask redaction

ergochat's `Message.Nick()` strips `!user@host` from the source prefix, but server-originated parameters can also carry hostmasks (e.g. RPL_TOPICWHOTIME). `sanitize.Nick` runs on every nick we render, regardless of source, ensuring hosts and IPs never reach the screen.

Specifically:

- WHOIS replies: 311's user@host fields are dropped; we surface only nick + realname. 338 (RPL_WHOISACTUALLY) and 378 (RPL_WHOISHOST) are intentionally not handled, since they exist only to expose the user's real address.
- WHO (352): user, host, and server fields are dropped.
- WHOWAS (314): same as 311.
- QUIT reasons: parenthetical clauses are stripped, since some servers append `(192.0.2.1)`.
- 367 RPL_BANLIST is intentionally not surfaced (banmasks contain hostmasks).

### Outgoing safety

- PRIVMSG and ACTION are split on word boundaries and rune-aligned, capped at 400 payload bytes (server line limit is 512 including prefix).
- `Input.Insert` sanitizes pasted content and caps the input field at 8 KB.
- `/raw` accepts only `[A-Z0-9]{1,32}` command names.
- All channel and nick targets are validated before send.

### Authentication

- Passwords are prompted with `golang.org/x/term.ReadPassword` (no echo) before Bubble Tea takes the TTY, or read from stdin with `--*-stdin` flags.
- Passwords are held in memory only; never written to disk, never logged.
- TLS sets `ServerName` for SNI explicitly; `InsecureSkipVerify` is opt-in via `--insecure`.
- For NickServ logins, JOINs are deferred until services confirm identification (RPL_LOGGEDIN 900, NickServ notice match, or 5-second fallback).

### Atomic config writes

`config.SaveTheme` uses temp-file + fsync + same-directory rename, so a crash mid-write never produces a truncated config.

## Bubble Tea v2 quirks worth knowing

- `View()` returns `tea.View` (a struct with `Content`, `AltScreen`, `MouseMode`, ...), not a string.
- `tea.KeyMsg` is now an interface; match `tea.KeyPressMsg` to ignore key-release events.
- Mouse mode is a per-frame `View.MouseMode` field, not a program option. We set `MouseModeCellMotion` in `View()` so it stays on across renders.
- Alt-screen is also a `View.AltScreen` field, not `tea.WithAltScreen()`.

## Tests

Run them all with the race detector:

```bash
go test -race ./...
```

Coverage:

- `internal/sanitize` covers ANSI/OSC/mIRC stripping, nick-host redaction, leak regressions.
- `irc` covers `splitMessage` UTF-8 safety, validation predicates, parenthetical stripping.
- `ui/model` covers buffer scroll anchoring, ring cap, input length cap, Insert sanitization.
- `coo` (root package) covers `--server` URL parsing.

## Adding things

- A new slash command: add a case in `commands.go`'s `runSlash` switch and a handler. Add it to `keymap.go` so the `?` overlay reflects it. Add it to `docs/usage.md`.
- A new IRC numeric: add a callback in `irc/client.go` via the `add(code, fn)` helper. If it's a query response, emit `InfoLineMsg` with a sanitized, formatted string. The model routes those to the active buffer automatically.
- A new theme: drop a TOML in `theme/themes/`, rebuild. Or place one in `~/.config/coo/themes/` with no rebuild.
- A new overlay: add an `Overlay` enum value, a renderer in `ui/components`, key/mouse handlers in `update.go`, and a case in `view.go`.
