# Usage

## Keyboard

| Key | Action |
| --- | --- |
| `?` (input empty) | Toggle keymap overlay |
| `Ctrl+N` / `Ctrl+P` | Next / previous channel |
| `Alt+1..9` | Jump to channel by index |
| `Ctrl+W` | Leave (PART) the current channel |
| `Ctrl+L` | Clear current buffer |
| `Ctrl+T` | Theme picker overlay |
| `Tab` | Nick completion (NAMES list, then recent history) |
| `PgUp` / `PgDn` | Scroll buffer ten lines |
| `Home` / `End` | Top / bottom of buffer |
| `Esc` | Close any open overlay |
| `Ctrl+C` | Quit |

In overlays, `j`/`k`, `g`/`G`, and `PgUp`/`PgDn` also work for scrolling and selection. `Enter` activates the selection in the theme picker and the names list.

## Mouse

- Click any tab in the top strip to switch channels.
- Click `<N` or `N>` overflow markers to scroll the strip.
- Click a name in the `/names` overlay to open a query buffer with that user.
- Mouse wheel scrolls the buffer (or the names overlay when open).

## Slash commands

### Messaging

| Command | Description |
| --- | --- |
| `/msg <nick\|#chan> [text]` | Send a private message; bare form opens a query buffer |
| `/me <action>` | CTCP ACTION in the active channel |
| `/notice <target> <text>` | Send a NOTICE |
| `/ping <nick>` | CTCP PING a user |

`/query` is an alias for `/msg`.

### Channels

| Command | Description |
| --- | --- |
| `/join #chan [#chan2 ...]` | Join one or more channels |
| `/part [reason]` | Leave the current channel |
| `/topic [text]` | Show or set the channel topic |
| `/names [#chan]` | Open the participants overlay |
| `/list [filter]` | Request the server channel directory |
| `/invite <nick> [#chan]` | Invite a user |

`/j` is an alias for `/join`.

### Lookup

Responses to these are routed to the buffer where you typed the command. Hostmasks and IPs are redacted; you get nick, realname, channels, idle time, account, and secure status.

| Command | Description |
| --- | --- |
| `/whois <nick>` | User metadata |
| `/whowas <nick>` | Historical metadata |
| `/who <target>` | Channel or nick listing |

### Operator actions

These run against the current channel.

| Command | Description |
| --- | --- |
| `/kick <nick> [reason]` | Kick a user |
| `/ban <nick\|mask>` | `+b` (bare nicks expand to `nick!*@*`) |
| `/unban <mask>` | `-b` |
| `/op <nick>` / `/deop <nick>` | Grant or revoke `+o` |
| `/voice <nick>` / `/devoice <nick>` | Grant or revoke `+v` |
| `/mode [target] [+modes]` | Raw MODE change |

`/kick #chan <nick> [reason]` works from any buffer.

### Self

| Command | Description |
| --- | --- |
| `/nick <newnick>` | Change nickname |
| `/away [reason]` | Set away (blank reason clears) |
| `/back` | Clear away |
| `/theme <name>` | Switch theme (also `/themes` for the picker) |
| `/clear` | Clear the active buffer |
| `/raw <COMMAND> [args]` | Send a raw IRC command |
| `/quit [reason]` | Disconnect and exit |

`/?` and `/help` open the keymap overlay.

## Buffers

The `*server*` buffer collects connection-wide messages: numerics, the MOTD, notices that aren't channel-targeted. You can't part it, only switch away.

Channel buffers (`#chan`) and private message buffers (`@nick`) are created on demand, ordered by JOIN time. Closing a channel buffer with `Ctrl+W` parts the channel. Closing a query buffer is currently not exposed; just switch away.

Unread activity puts a dot on the tab and an accent color on the label. A mention (your nick appearing in a message) or any private message turns the tab red until you read it.

## Tips

- Press `?` to discover everything visually.
- The mouse wheel scrolls the active buffer even if your terminal also scrolls; the altscreen captures wheel events.
- Long pasted messages are split at word boundaries before they hit the wire (IRC has a 512-byte line cap; we use 400 for the payload to leave room for the server prefix).
- For `+r` (registered-only) channels, prefer `--sasl` over `--nickserv` if your network supports it. SASL authenticates before the connection is fully registered, so JOINs succeed on the first try.
