package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/highlight"
)

func themePalette(t config.Theme) highlight.Palette {
	return highlight.Palette{
		Keyword: t.Keyword, String: t.String, Number: t.Number, Comment: t.Comment,
		Function: t.Function, Type: t.Type, Operator: t.Operator, Constant: t.Constant,
		Builtin: t.Builtin, Punctuation: t.Punctuation,
	}
}

// applyTheme меняет тему везде: UI, подсветку и кэш вкладок.
func (m *Model) applyTheme(name string) {
	th := config.ThemeByName(name)
	m.cfg.Theme = th.Name
	m.theme = th
	m.th = newSGRTheme(th)
	m.hl.SetPalette(themePalette(th))
	for _, t := range m.tabs {
		t.hlVersion = -1
		t.hlLines = nil
	}
	m.clampScroll()
}

func (m *Model) openThemeList() {
	m.closeOverlays()
	m.ov.kind = overlayThemes
	m.ov.input.Placeholder = m.tr("тема", "theme")
	m.ov.themeOrig = m.cfg.Theme
	items := m.themeItems()
	m.ov.sel = 0
	for i, name := range items {
		if name == m.cfg.Theme {
			m.ov.sel = i
			break
		}
	}
}

func (m *Model) themeItems() []string {
	all := config.ThemeNames()
	q := strings.TrimSpace(m.ov.input.Value())
	if q == "" {
		return all
	}
	out := make([]string, 0, len(all))
	for _, name := range all {
		if matchScore(name, q) > 0 {
			out = append(out, name)
		}
	}
	return out
}

func (m *Model) themeMove(delta int) {
	items := m.themeItems()
	if len(items) == 0 {
		return
	}
	n := len(items)
	m.ov.sel = (m.ov.sel + delta%n + n) % n
	m.previewTheme()
}

func (m *Model) previewTheme() {
	items := m.themeItems()
	if m.ov.sel < 0 || m.ov.sel >= len(items) {
		return
	}
	if name := items[m.ov.sel]; name != m.cfg.Theme {
		m.applyTheme(name)
	}
}

func (m *Model) cancelThemeList() {
	orig := m.ov.themeOrig
	if orig != "" {
		m.applyTheme(orig)
	}
	m.closeOverlays()
}

func (m *Model) saveTheme() {
	items := m.themeItems()
	if len(items) == 0 {
		return
	}
	m.ov.sel = max(0, min(m.ov.sel, len(items)-1))
	name := items[m.ov.sel]
	m.applyTheme(name)
	m.closeOverlays()
	if err := config.SaveTheme(name); err != nil {
		m.notifyError(m.trf("тема не сохранена: %v", "theme not saved: %v", err))
		return
	}
	m.notifySuccess(m.tr("тема: ", "theme: ") + name)
}

func (m *Model) themeKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		m.cancelThemeList()
		return nil
	case "ctrl+e":
		m.cancelThemeList()
		return m.runCommand("palette.commands")
	case "enter":
		m.saveTheme()
		return nil
	case "up", "ctrl+p":
		m.themeMove(-1)
		return nil
	case "down", "ctrl+n":
		m.themeMove(1)
		return nil
	case "pgup":
		m.themeMove(-12)
		return nil
	case "pgdown":
		m.themeMove(12)
		return nil
	case "ctrl+u":
		m.ov.input.Clear()
		m.ov.sel = 0
		m.previewTheme()
		return nil
	case "ctrl+w":
		m.ov.input.DeleteWordLeft()
		m.ov.sel = 0
		m.previewTheme()
		return nil
	case "backspace":
		m.ov.input.Backspace()
		m.ov.sel = 0
		m.previewTheme()
		return nil
	case "delete":
		m.ov.input.Delete()
		m.ov.sel = 0
		m.previewTheme()
		return nil
	case "left":
		m.ov.input.MoveLeft()
		return nil
	case "right":
		m.ov.input.MoveRight()
		return nil
	case "ctrl+left":
		m.ov.input.WordLeft()
		return nil
	case "ctrl+right":
		m.ov.input.WordRight()
		return nil
	case "home":
		m.ov.input.Home()
		return nil
	case "end":
		m.ov.input.End()
		return nil
	}
	if printable(msg) {
		m.ov.input.Insert(msg.Text)
		m.ov.sel = 0
		m.previewTheme()
	}
	return nil
}

func (m *Model) themeGeometry() (listY, listH, footerY, inputY int) {
	if m.h < 5 || m.w < 8 {
		return -1, 0, -1, -1
	}
	inputY = m.h - 2
	footerY = m.h - 3
	items := m.themeItems()
	n := len(items)
	if n == 0 {
		n = 1
	}
	listH = min(n, 12, max(0, footerY))
	listY = footerY - listH
	if listY < 0 {
		listY = 0
		listH = footerY
	}
	return listY, listH, footerY, inputY
}

func (m *Model) themeStart() int {
	_, listH, _, _ := m.themeGeometry()
	if listH <= 0 {
		return 0
	}
	if m.ov.sel >= listH {
		return m.ov.sel - listH + 1
	}
	return 0
}

func (m *Model) themeRow(name string, selected bool) string {
	style := m.th.panelOn
	mark := "  "
	if selected {
		style = m.th.panelSel
		mark = "▸ "
	}
	line := mark + name
	if name == m.cfg.Theme {
		line += "  ✓"
	}
	return style + padANSI(truncateTo(line, m.w), m.w, "") + sgrReset
}

func (m *Model) drawThemeOverlay(rows []string) []string {
	listY, listH, footerY, inputY := m.themeGeometry()
	if inputY < 0 {
		return rows
	}
	items := m.themeItems()
	start := m.themeStart()
	line := listY
	if len(items) == 0 {
		row := m.th.panelDim + "  " + m.tr("ничего не найдено", "nothing found") + sgrReset
		if line >= 0 && line < len(rows) {
			rows[line] = padANSI(row, m.w, m.th.panel)
		}
	} else {
		for i := start; i < len(items) && i < start+listH; i++ {
			if line >= 0 && line < len(rows) {
				rows[line] = m.themeRow(items[i], i == m.ov.sel)
			}
			line++
		}
	}
	sel := 0
	if len(items) > 0 {
		sel = max(0, min(m.ov.sel, len(items)-1)) + 1
	}
	footer := m.trf("%d из %d · ↑↓ · Enter · Esc", "%d of %d · ↑↓ · Enter · Esc", sel, len(items))
	if footerY >= 0 && footerY < len(rows) {
		rows[footerY] = m.th.panel + m.th.panelDim + padANSI("  "+footer, m.w, "") + sgrReset
	}
	rows[inputY] = m.inputOverlayRow("> ", &m.ov.input)
	return rows
}

func (m *Model) themeClick(e tea.Mouse) tea.Cmd {
	listY, listH, _, inputY := m.themeGeometry()
	if inputY < 0 {
		m.cancelThemeList()
		return nil
	}
	if e.Y >= listY && e.Y < listY+listH {
		if idx := m.themeStart() + (e.Y - listY); idx >= 0 && idx < len(m.themeItems()) {
			m.ov.sel = idx
			m.previewTheme()
		}
		return nil
	}
	if e.Y >= listY && e.Y <= inputY {
		return nil
	}
	m.cancelThemeList()
	return nil
}

func (m *Model) themeHover(e tea.Mouse) {
	listY, listH, _, _ := m.themeGeometry()
	if listH <= 0 || e.Y < listY || e.Y >= listY+listH {
		return
	}
	if idx := m.themeStart() + (e.Y - listY); idx >= 0 && idx < len(m.themeItems()) && idx != m.ov.sel {
		m.ov.sel = idx
		m.previewTheme()
	}
}

func (m *Model) themeWheel(up bool) {
	if up {
		m.themeMove(-3)
	} else {
		m.themeMove(3)
	}
}
