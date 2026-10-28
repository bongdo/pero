package ui

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/widgets"
)

const peroVersion = "0.1"

var helpSectionOrder = [][2]string{
	{"Файлы", "Files"},
	{"Вкладки", "Tabs"},
	{"Правка", "Edit"},
	{"Поиск", "Search"},
	{"Markdown", "Markdown"},
	{"Навигация", "Navigation"},
	{"Вид", "View"},
	{"Интерфейс", "Interface"},
	{"LSP", "LSP"},
}

func (m *Model) openHelp() {
	m.closeOverlays()
	m.ov.kind = overlayHelp
	m.ov.helpScroll = 0
}

func (m *Model) helpKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc", "f1", "q":
		m.closeOverlays()
	case "up", "k":
		m.ov.helpScroll--
	case "down", "j":
		m.ov.helpScroll++
	case "pgup":
		m.ov.helpScroll -= max(1, m.h-4)
	case "pgdown":
		m.ov.helpScroll += max(1, m.h-4)
	case "home", "g":
		m.ov.helpScroll = 0
	case "end", "G":
		m.ov.helpScroll = 1 << 20
	}
	m.clampHelp()
	return nil
}

func (m *Model) clampHelp() {
	lines := m.helpLines(max(10, m.w-4))
	visible := max(1, m.h-2)
	maxScroll := max(0, len(lines)-visible)
	m.ov.helpScroll = max(0, min(m.ov.helpScroll, maxScroll))
}

func (m *Model) helpLines(width int) []string {
	lang := m.cfg.Language
	groups := map[string][]command{}
	var order []string
	for _, c := range m.commands() {
		sec := c.sectionFor(lang)
		if sec == "" {
			sec = m.tr("Прочее", "Other")
		}
		if _, ok := groups[sec]; !ok {
			order = append(order, sec)
		}
		groups[sec] = append(groups[sec], c)
	}
	var out []string
	seen := map[string]bool{}
	for _, pair := range helpSectionOrder {
		sec := m.tr(pair[0], pair[1])
		if cs, ok := groups[sec]; ok {
			out = append(out, "", sec)
			for _, c := range cs {
				out = append(out, m.helpCommandLine(c))
			}
			seen[sec] = true
		}
	}
	for _, sec := range order {
		if seen[sec] {
			continue
		}
		out = append(out, "", sec)
		for _, c := range groups[sec] {
			out = append(out, m.helpCommandLine(c))
		}
	}
	var wrapped []string
	for _, line := range out {
		trimmed := strings.TrimSpace(line)
		switch {
		case line == "":
			wrapped = append(wrapped, "")
		case trimmed == line && m.isSectionTitle(line):
			wrapped = append(wrapped, line)
		default:
			wrapped = append(wrapped, wrapWords(line, max(8, width))...)
		}
	}
	return wrapped
}

func (m *Model) isSectionTitle(s string) bool {
	for _, pair := range helpSectionOrder {
		if s == m.tr(pair[0], pair[1]) {
			return true
		}
	}
	return s == m.tr("Прочее", "Other")
}

func (m *Model) helpCommandLine(c command) string {
	key := c.key
	if key == "" {
		key = "-"
	}
	gap := 16 - ansiWidth(key)
	if gap < 2 {
		gap = 2
	}
	return "  " + key + strings.Repeat(" ", gap) + c.titleFor(m.cfg.Language)
}

type helpView struct {
	box  []string
	x, y int
}

func (m *Model) helpView() helpView {
	inner := max(10, m.w-4)
	lines := m.helpLines(inner)
	visible := max(1, m.h-2)
	start := max(0, min(m.ov.helpScroll, max(0, len(lines)-visible)))
	end := min(len(lines), start+visible)
	rows := make([]string, 0, visible)
	for _, line := range lines[start:end] {
		style := m.th.panelOn
		if m.isSectionTitle(strings.TrimSpace(line)) && strings.TrimSpace(line) != "" {
			style = m.th.panel + m.th.accent + "\x1b[1m"
		}
		rows = append(rows, style+padANSI(line, inner, "")+sgrReset)
	}
	for len(rows) < visible {
		rows = append(rows, m.th.panel+strings.Repeat(" ", inner)+sgrReset)
	}
	cfgPath := "~/.config/pero/config.toml"
	if p, err := config.ConfigPath(); err == nil {
		cfgPath = p
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cfgPath, home) {
			cfgPath = "~" + strings.TrimPrefix(cfgPath, home)
		}
	}
	footer := m.tr("Версия ", "Version ") + peroVersion + " · " + sanitizeOneLine(cfgPath) + " · " + m.tr("Esc закрыть", "Esc close")
	box := widgets.RenderBox(widgets.Box{
		Title: m.tr("Помощь", "Help"), Rows: rows, Footer: footer, Width: m.w,
	}, m.th.widgetStyle())
	x := max(0, (m.w-ansiWidth(box[0]))/2)
	y := max(0, (m.h-len(box))/2)
	return helpView{box: box, x: x, y: y}
}
