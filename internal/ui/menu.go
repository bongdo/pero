package ui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/widgets"
)

type menuItem struct {
	title    string
	key      string
	action   string
	disabled bool
	sep      bool
}

type menuState struct {
	open    bool
	anchorX int
	anchorY int
	items   []menuItem
	sel     int
	target  int
	title   string
}

func (m *Model) openTextMenu(x, y int) {
	if m.w < 40 || m.h < 10 {
		return
	}
	t := m.tab()
	if t == nil {
		return
	}
	hasSel := t.ed.HasSelection()
	word := m.wordAtCursor()
	items := []menuItem{
		{title: m.tr("Вырезать", "Cut"), key: "Ctrl-X", action: "edit.cut", disabled: !hasSel || t.ed.Buf.ReadOnly},
		{title: m.tr("Копировать", "Copy"), key: "Ctrl-C", action: "edit.copy", disabled: !hasSel},
		{title: m.tr("Вставить", "Paste"), key: "Ctrl-V", action: "edit.paste", disabled: t.ed.Buf.ReadOnly},
		{sep: true},
		{title: m.tr("Выделить всё", "Select all"), key: "Ctrl-A", action: "edit.selectAll"},
		{title: m.tr("Найти слово", "Find word"), action: "menu.findWord", disabled: word == ""},
		{title: m.tr("Исправить…", "Quick fix…"), key: "Alt-.", action: "lsp.codeaction", disabled: len(t.visibleDiags()) == 0},
	}
	m.openMenu(x, y, m.tr("Текст", "Text"), items, m.active)
}

func (m *Model) openTabMenu(x, y, idx int) {
	if m.w < 40 || m.h < 10 {
		return
	}
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	m.switchTo(idx)
	if m.ov.kind == overlayConfirm {
		return
	}
	t := m.tabs[idx]
	items := []menuItem{
		{title: m.tr("Сохранить", "Save"), key: "Ctrl-S", action: "file.save", disabled: t.ed.Buf.ReadOnly},
		{title: m.tr("Закрыть", "Close"), key: "Alt-Q", action: "tab.close"},
		{title: m.tr("Закрыть остальные", "Close others"), action: "tab.closeOthers", disabled: len(m.tabs) < 2},
		{sep: true},
		{title: m.tr("Показать путь", "Show path"), action: "menu.showPath", disabled: t.ed.Buf.Path == ""},
	}
	m.openMenu(x, y, m.tr("Вкладка", "Tab"), items, idx)
}

func (m *Model) openMenu(x, y int, title string, items []menuItem, target int) {
	if m.ov.cancel != nil {
		m.ov.cancel()
	}
	m.ov = overlay{kind: overlayMenu}
	m.ov.menu = menuState{open: true, anchorX: x, anchorY: y, items: items, target: target, title: title}
	m.menuSelectFirst()
}

func (m *Model) menuEnabled(i int) bool {
	if i < 0 || i >= len(m.ov.menu.items) {
		return false
	}
	it := m.ov.menu.items[i]
	return !it.sep && !it.disabled
}

func (m *Model) menuSelectFirst() {
	m.ov.menu.sel = -1
	m.menuMove(1)
}

func (m *Model) menuMove(delta int) {
	n := len(m.ov.menu.items)
	if n == 0 {
		return
	}
	i := m.ov.menu.sel
	for step := 0; step < n; step++ {
		i = (i + delta%n + n) % n
		if m.menuEnabled(i) {
			m.ov.menu.sel = i
			return
		}
	}
}

func (m *Model) closeMenu() {
	m.ov = overlay{}
}

func (m *Model) menuKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		m.closeMenu()
	case "up", "k":
		m.menuMove(-1)
	case "down", "j":
		m.menuMove(1)
	case "enter":
		return m.activateMenu(m.ov.menu.sel)
	case "ctrl+x":
		m.closeMenu()
		return m.runCommand("edit.cut")
	case "ctrl+c":
		m.closeMenu()
		return m.runCommand("edit.copy")
	case "ctrl+v":
		m.closeMenu()
		return m.runCommand("edit.paste")
	}
	return nil
}

func (m *Model) activateMenu(i int) tea.Cmd {
	if !m.menuEnabled(i) {
		return nil
	}
	it := m.ov.menu.items[i]
	switch it.action {
	case "menu.showPath":
		path := ""
		if m.ov.menu.target >= 0 && m.ov.menu.target < len(m.tabs) {
			path = m.tabs[m.ov.menu.target].ed.Buf.Path
		}
		m.closeMenu()
		if path != "" {
			m.notifyInfo(path)
		}
		return nil
	case "menu.findWord":
		word := m.wordAtCursor()
		m.closeMenu()
		if word == "" {
			return nil
		}
		m.openFind(false)
		m.ov.input.Set(word)
		m.recomputeFind()
		return nil
	}
	m.closeMenu()
	return m.runCommand(it.action)
}

func (m *Model) wordAtCursor() string {
	t := m.tab()
	if t == nil {
		return ""
	}
	if t.ed.HasSelection() {
		text := strings.TrimSpace(t.ed.SelectedText())
		text = strings.ReplaceAll(text, "\n", " ")
		if text != "" {
			return text
		}
	}
	line := t.ed.Buf.Line(t.ed.Cursor.Line)
	col := t.ed.Cursor.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	if col < 0 {
		return ""
	}
	if !wordRune(line[col]) {
		return ""
	}
	start, end := col, col+1
	for start > 0 && wordRune(line[start-1]) {
		start--
	}
	for end < len(line) && wordRune(line[end]) {
		end++
	}
	return string(line[start:end])
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func (m *Model) menuShorten(s string) string {
	if m.w >= 40 {
		return s
	}
	r := []rune(s)
	if len(r) <= 7 {
		return s
	}
	return string(r[:6]) + "."
}

type menuView struct {
	shown bool
	box   []string
	x, y  int
	rows  []rect // по индексу items; y абсолютный, только для непустых
}

func (m *Model) menuView() menuView {
	if m.ov.kind != overlayMenu || !m.ov.menu.open {
		return menuView{}
	}
	items := m.ov.menu.items
	showKeys := m.w >= 40
	maxItem := 0
	for _, it := range items {
		if it.sep {
			continue
		}
		w := ansiWidth(it.title)
		if showKeys && it.key != "" {
			w += ansiWidth(it.key) + 2
		}
		maxItem = max(maxItem, w)
	}
	inner := min(max(24, maxItem+4), max(10, m.w-6))
	rows := make([]string, 0, len(items))
	for i, it := range items {
		if it.sep {
			rows = append(rows, m.th.panelBorder+strings.Repeat("─", inner)+sgrReset)
			continue
		}
		title := m.menuShorten(it.title)
		line := "  " + title
		if showKeys && it.key != "" {
			gap := inner - ansiWidth(line) - ansiWidth(it.key) - 2
			if gap < 1 {
				gap = 1
			}
			line += strings.Repeat(" ", gap) + it.key
		}
		line = truncateTo(line, inner)
		style := m.th.panelOn
		switch {
		case it.disabled:
			style = m.th.panelDim
		case i == m.ov.menu.sel:
			style = m.th.panelSel
		}
		rows = append(rows, style+padANSI(line, inner, "")+sgrReset)
	}
	box := widgets.RenderBox(widgets.Box{Rows: rows, Width: 0}, m.th.widgetStyle())
	bw := 0
	for _, l := range box {
		bw = max(bw, ansiWidth(l))
	}
	x, y := widgets.PlaceBelow(m.ov.menu.anchorX, m.ov.menu.anchorY+1, bw, len(box), m.w, m.h)
	if x == 0 && y == 0 && (m.w < 40 || m.h < 10) {
		return menuView{}
	}
	rects := make([]rect, len(items))
	for i := range items {
		if items[i].sep {
			continue
		}
		rects[i] = rect{x: x + 1, y: y + 1 + i, w: bw - 2, h: 1}
	}
	return menuView{shown: true, box: box, x: x, y: y, rows: rects}
}
