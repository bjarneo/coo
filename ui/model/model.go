package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"coo/irc"
	"coo/theme"
	"coo/ui"
	"coo/ui/components"
)

// ServerBufferName is the synthetic buffer for server numerics and notices.
const ServerBufferName = "*server*"

// Overlay represents which modal is currently shown (if any).
type Overlay int

const (
	OverlayNone Overlay = iota
	OverlayKeymap
	OverlayThemes
	OverlayNames
)

// Model is the Bubble Tea state for coo.
type Model struct {
	client    *irc.Client
	buffers   map[string]*Buffer
	order     []*Buffer // display order in the channel strip
	active    int
	input     Input
	width     int
	height    int
	overlay   Overlay
	themes    []theme.Theme
	themeIdx  int
	connected bool
	statusMsg string

	// pendingNamesShow tracks channels for which the user invoked /names
	// and we should auto-render the result on NamesEnd.
	pendingNamesShow map[string]struct{}

	keymapScroll int

	// tabHits / namesHits are the per-render click target lists emitted by
	// the strip and names components; consulted by MouseClickMsg.
	tabHits   []components.TabHit
	namesHits []components.NameHit

	// namesChannel / namesIdx drive the /names overlay.
	namesChannel string
	namesIdx     int

	// quitting becomes true once Close has been issued and we're waiting
	// for the disconnect-then-Quit flow to complete.
	quitting bool
}

// New constructs the model with the server buffer. Channels are created
// lazily as JOINs come in.
func New(client *irc.Client, themes []theme.Theme, themeName string) *Model {
	m := &Model{
		client:           client,
		buffers:          make(map[string]*Buffer),
		themes:           themes,
		pendingNamesShow: make(map[string]struct{}),
	}
	m.addBuffer(&Buffer{Name: ServerBufferName, IsServer: true})
	for i, t := range themes {
		if strings.EqualFold(t.Name, themeName) {
			m.themeIdx = i
			break
		}
	}
	return m
}

// CurrentBuffer returns the active buffer, or nil if none have been added.
func (m *Model) CurrentBuffer() *Buffer {
	if len(m.order) == 0 {
		return nil
	}
	if m.active >= len(m.order) {
		m.active = len(m.order) - 1
	}
	return m.order[m.active]
}

// CurrentTarget returns the IRC target for sent messages, or "" for the
// server buffer.
func (m *Model) CurrentTarget() string {
	b := m.CurrentBuffer()
	if b == nil || b.IsServer {
		return ""
	}
	return b.Name
}

// ThemeName returns the active theme's display name.
func (m *Model) ThemeName() string {
	if m.themeIdx < 0 || m.themeIdx >= len(m.themes) {
		return theme.DefaultName
	}
	return m.themes[m.themeIdx].Name
}

func (m *Model) addBuffer(b *Buffer) (*Buffer, int) {
	key := strings.ToLower(b.Name)
	if existing, ok := m.buffers[key]; ok {
		for i, ob := range m.order {
			if ob == existing {
				return existing, i
			}
		}
	}
	m.buffers[key] = b
	m.order = append(m.order, b)
	return b, len(m.order) - 1
}

func (m *Model) removeBuffer(name string) {
	key := strings.ToLower(name)
	b, ok := m.buffers[key]
	if !ok || b.IsServer {
		return
	}
	delete(m.buffers, key)
	for i, ob := range m.order {
		if ob == b {
			m.order = append(m.order[:i], m.order[i+1:]...)
			if m.active >= i && m.active > 0 {
				m.active--
			}
			break
		}
	}
}

// findBuffer returns the buffer for a given target, creating it on demand
// for channels and PMs.
func (m *Model) findBuffer(name string) *Buffer {
	if name == "" {
		return m.buffers[strings.ToLower(ServerBufferName)]
	}
	key := strings.ToLower(name)
	if b, ok := m.buffers[key]; ok {
		return b
	}
	b := &Buffer{Name: name}
	m.addBuffer(b)
	return b
}

// activeBuffer is shorthand for the buffer at m.active, or nil.
func (m *Model) activeBuffer() *Buffer {
	if m.active < 0 || m.active >= len(m.order) {
		return nil
	}
	return m.order[m.active]
}

// openNamesOverlay shows the participant list for channel as a scrollable
// modal. Caller must ensure the buffer has Names populated.
func (m *Model) openNamesOverlay(channel string) {
	m.overlay = OverlayNames
	m.namesChannel = channel
	m.namesIdx = 0
}

// openQuery focuses (creating if necessary) a private-message buffer for
// nick and dismisses any open overlay.
func (m *Model) openQuery(nick string) {
	nick = strings.TrimSpace(nick)
	if nick == "" {
		return
	}
	b := m.findBuffer(nick)
	for i, ob := range m.order {
		if ob == b {
			m.active = i
			break
		}
	}
	m.overlay = OverlayNone
	m.namesIdx = 0
}

// applyThemeIdx switches to themes[i] and updates the palette.
func (m *Model) applyThemeIdx(i int) {
	if i < 0 || i >= len(m.themes) {
		return
	}
	m.themeIdx = i
	ui.ApplyTheme(m.themes[i])
}

func (m *Model) Init() tea.Cmd {
	ui.ApplyTheme(m.themes[m.themeIdx])
	return waitForEvent(m.client.Events())
}

// waitForEvent reads one message off the IRC events channel as a tea.Cmd
// and recursively re-subscribes after each delivery.
func waitForEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}
