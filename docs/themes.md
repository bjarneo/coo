# Themes

`coo` ships with 20 builtin themes and supports user themes loaded from `~/.config/coo/themes/*.toml`.

## Switching themes

Three ways:

```bash
# At startup
coo --theme tokyo-night ...

# In the running app
/theme tokyo-night
/themes               # opens the picker overlay

# Via the keymap
Ctrl+T                # opens the picker; arrow keys + Enter to apply
```

Whatever you choose is written to `~/.config/coo/config.toml` atomically and restored on next launch. `theme = ""` (empty) means follow the terminal's own palette.

## Builtin themes

| Theme | Notes |
| --- | --- |
| `Default - Terminal colors` | ANSI 0-15, follows your terminal palette |
| `ayu-mirage-dark` | |
| `catppuccin` | Mocha |
| `catppuccin-latte` | Light |
| `dracula` | |
| `ember` | |
| `ethereal` | |
| `everforest` | |
| `flexoki-light` | Light |
| `gruvbox` | |
| `hackerman` | Green-on-black |
| `kanagawa` | |
| `matte-black` | |
| `miasma` | |
| `neon-blade-runner` | |
| `nord` | |
| `osaka-jade` | |
| `ristretto` | |
| `rose-pine` | |
| `tokyo-night` | |
| `vantablack` | High-contrast dark |

Run `coo --list-themes` to print the current list.

## Custom themes

Each theme is a TOML file with six color fields. Drop one at `~/.config/coo/themes/myname.toml`:

```toml
accent    = "#73d0ff"
bright_fg = "#f3f4f5"
fg        = "#cccac2"
green     = "#d5ff80"
yellow    = "#ffad66"
red       = "#f28779"
```

| Field | Used for |
| --- | --- |
| `accent` | Active tab background, prompt symbol, highlights, mode prefixes |
| `bright_fg` | Primary text |
| `fg` | Muted text, separators |
| `green` | Joins, success, your own messages |
| `yellow` | Parts, warnings, mentions |
| `red` | Errors, kicks, mentions of your nick |

User themes override builtin themes with the same name. The filename (without `.toml`) becomes the theme's display name.

## Default vs named themes

The default theme uses ANSI colors 0-15 directly. This means the colors track whatever palette your terminal emulator is configured with: alacritty themes, kitty themes, GNOME Terminal profiles all "just work" without configuration.

Named themes use absolute hex values, so they look the same regardless of terminal palette. Useful when you want a specific look (e.g. catppuccin) regardless of where you're running.

Switching between them is free; `coo` doesn't restart on theme change.
