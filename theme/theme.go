// Package theme loads color themes from embedded and user TOML files.
package theme

import (
	"bufio"
	"cmp"
	"embed"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"coo/internal/appdir"
)

//go:embed themes/*.toml
var builtinThemes embed.FS

// DefaultName is the display name for the built-in ANSI fallback theme.
const DefaultName = "Default - Terminal colors"

// Theme is a named color scheme. Empty hex fields signal ANSI fallback.
type Theme struct {
	Name     string
	Accent   string
	BrightFG string
	FG       string
	Green    string
	Yellow   string
	Red      string
}

// IsDefault reports whether this is the sentinel default theme.
func (t Theme) IsDefault() bool {
	return t.Accent == "" && t.Green == "" && t.BrightFG == ""
}

// Default returns the sentinel theme that triggers ANSI fallback colors.
func Default() Theme { return Theme{Name: DefaultName} }

// Parse reads flat TOML key=value lines from r.
func Parse(name string, r io.Reader) (Theme, error) {
	t := Theme{Name: name}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "accent":
			t.Accent = val
		case "bright_fg":
			t.BrightFG = val
		case "fg":
			t.FG = val
		case "red":
			t.Red = val
		case "yellow":
			t.Yellow = val
		case "green":
			t.Green = val
		}
	}
	return t, scanner.Err()
}

// LoadAll returns the default theme followed by all builtin and user themes,
// sorted alphabetically (case-insensitive). User themes override builtins
// with the same name.
func LoadAll() []Theme {
	themes := make(map[string]Theme)
	loadBuiltin(themes)
	if dir, err := appdir.Dir(); err == nil {
		loadUserDir(filepath.Join(dir, "themes"), themes)
	}
	result := make([]Theme, 0, len(themes)+1)
	result = append(result, Default())
	for _, t := range themes {
		result = append(result, t)
	}
	slices.SortFunc(result[1:], func(a, b Theme) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return result
}

// Find returns the theme whose name matches (case-insensitive). Falls back
// to Default() when name is blank or no match exists.
func Find(name string) Theme {
	if name == "" {
		return Default()
	}
	for _, t := range LoadAll() {
		if strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return Default()
}

func loadBuiltin(themes map[string]Theme) {
	entries, err := builtinThemes.ReadDir("themes")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".toml")
		f, err := builtinThemes.Open("themes/" + e.Name())
		if err != nil {
			continue
		}
		t, err := Parse(name, f)
		f.Close()
		if err != nil {
			continue
		}
		themes[strings.ToLower(name)] = t
	}
}

func loadUserDir(dir string, themes map[string]Theme) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".toml")
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		t, err := Parse(name, f)
		f.Close()
		if err != nil {
			continue
		}
		themes[strings.ToLower(name)] = t
	}
}
