package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/widgets"
)

func (m *Model) openCommandPalette() {
	m.closeOverlays()
	m.ov.kind = overlayCommands
	m.ov.input.Placeholder = m.tr("команда", "command")
	m.ov.sel = 0
}

func (m *Model) closeOverlays() {
	if m.ov.cancel != nil {
		m.ov.cancel()
	}
	m.ov = overlay{}
	if t := m.tab(); t != nil && t.comp.open {
		t.comp = completionState{}
		m.compGen++
	}
}

func (m *Model) paletteMaxRows() int {
	if m.w < 40 {
		return 3
	}
	return 8
}

func (m *Model) paletteItems() []command {
	all := m.commandList()
	q := strings.TrimSpace(m.ov.input.Value())
	if q != "" {
		return filterCommands(all, q)
	}
	// Пустой запрос: последние команды сверху, затем каталог.
	byID := make(map[string]command, len(all))
	for _, c := range all {
		byID[c.id] = c
	}
	var out []command
	seen := map[string]bool{}
	for _, id := range m.recentCmds {
		if c, ok := byID[id]; ok && !seen[id] {
			out = append(out, c)
			seen[id] = true
		}
	}
	for _, c := range all {
		if !seen[c.id] {
			out = append(out, c)
		}
	}
	return out
}

func (m *Model) paletteClamp() {
	items := m.paletteItems()
	if len(items) == 0 {
		m.ov.sel = 0
		return
	}
	m.ov.sel = max(0, min(m.ov.sel, len(items)-1))
}

func (m *Model) paletteMove(delta int) {
	items := m.paletteItems()
	if len(items) == 0 {
		return
	}
	n := len(items)
	m.ov.sel = (m.ov.sel + delta%n + n) % n
}

func (m *Model) paletteKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.Keystroke()
	if m.ov.kind == overlaySaveAs {
		switch k {
		case "esc":
			m.pendingClose = -1
			m.closeOverlays()
		case "enter":
			path := m.ov.input.Value()
			m.closeOverlays()
			return m.saveAs(path)
		case "ctrl+u":
			m.ov.input.Clear()
		case "ctrl+w":
			m.ov.input.DeleteWordLeft()
		case "backspace":
			m.ov.input.Backspace()
		case "delete":
			m.ov.input.Delete()
		case "left":
			m.ov.input.MoveLeft()
		case "right":
			m.ov.input.MoveRight()
		case "ctrl+left":
			m.ov.input.WordLeft()
		case "ctrl+right":
			m.ov.input.WordRight()
		case "home", "ctrl+a":
			m.ov.input.Home()
		case "end":
			m.ov.input.End()
		default:
			if printable(msg) {
				m.ov.input.Insert(msg.Text)
			}
		}
		return nil
	}

	switch k {
	case "esc", "ctrl+e":
		m.closeOverlays()
		return nil
	case "enter":
		items := m.paletteItems()
		if len(items) == 0 {
			return nil
		}
		m.ov.sel = max(0, min(m.ov.sel, len(items)-1))
		id := items[m.ov.sel].id
		m.closeOverlays()
		return m.runCommand(id)
	case "up", "ctrl+p":
		m.paletteMove(-1)
		return nil
	case "down", "ctrl+n":
		m.paletteMove(1)
		return nil
	case "pgup":
		m.paletteMove(-m.paletteMaxRows())
		return nil
	case "pgdown":
		m.paletteMove(m.paletteMaxRows())
		return nil
	case "tab":
		items := m.paletteItems()
		if len(items) > 0 {
			m.ov.sel = max(0, min(m.ov.sel, len(items)-1))
			m.ov.input.Set(items[m.ov.sel].titleFor(m.cfg.Language))
			m.ov.input.End()
		}
		return nil
	case "ctrl+u":
		m.ov.input.Clear()
		m.ov.sel = 0
		return nil
	case "ctrl+w":
		m.ov.input.DeleteWordLeft()
		m.ov.sel = 0
		return nil
	case "backspace":
		m.ov.input.Backspace()
		m.ov.sel = 0
		return nil
	case "delete":
		m.ov.input.Delete()
		m.ov.sel = 0
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
	}
	return nil
}

// renderInput — строка ввода с блочным курсором и горизонтальным скроллом.
func renderInput(in *widgets.Input, width int, textStyle, cursorStyle string) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(sanitizeOneLine(in.Value()))
	cur := max(0, min(in.Cursor, len(runes)))
	start := 0
	for start < cur {
		used := 1 // курсор занимает ячейку
		for i := start; i < cur; i++ {
			used += ansiWidth(string(runes[i]))
		}
		if used <= width {
			break
		}
		start++
	}
	var b strings.Builder
	used := 0
	for i := start; i < len(runes) && used < width; i++ {
		w := ansiWidth(string(runes[i]))
		if used+w > width {
			break
		}
		style := textStyle
		if i == cur {
			style = cursorStyle
		}
		b.WriteString(style)
		b.WriteString(string(runes[i]))
		b.WriteString(sgrReset)
		used += w
	}
	if cur == len(runes) && used < width {
		b.WriteString(cursorStyle)
		b.WriteString(" ")
		b.WriteString(sgrReset)
		used++
	}
	if len(runes) == 0 && in.Placeholder != "" && used < width {
		ph := truncateTo(in.Placeholder, width-used)
		b.WriteString(textStyle)
		b.WriteString(ph)
		b.WriteString(sgrReset)
		used += ansiWidth(ph)
	}
	if used < width {
		b.WriteString(textStyle)
		b.WriteString(strings.Repeat(" ", width-used))
		b.WriteString(sgrReset)
	}
	return b.String()
}

func (m *Model) commandPaletteGeometry() (inputY, listY, listH int) {
	if m.h < 3 || m.w < 8 {
		return -1, -1, 0
	}
	inputY = m.h - 2
	items := m.paletteItems()
	n := len(items)
	if n == 0 {
		n = 1
	}
	listH = min(n, m.paletteMaxRows(), inputY)
	listY = inputY - listH
	if listY < 0 {
		listY = 0
		listH = inputY
	}
	return
}

func (m *Model) commandPaletteStart() int {
	_, _, listH := m.commandPaletteGeometry()
	if listH <= 0 {
		return 0
	}
	if m.ov.sel >= listH {
		return m.ov.sel - listH + 1
	}
	return 0
}

func (m *Model) commandPaletteRows() []string {
	inputY, _, listH := m.commandPaletteGeometry()
	if inputY < 0 {
		return nil
	}
	rows := make([]string, 0, 2)
	items := m.paletteItems()
	if len(items) == 0 {
		row := m.th.panelDim + "  " + m.tr("ничего не найдено", "nothing found") + sgrReset
		rows = append(rows, row)
	} else {
		start := m.commandPaletteStart()
		for i := start; i < len(items) && i < start+listH; i++ {
			rows = append(rows, m.commandPaletteRow(items[i], i == m.ov.sel))
		}
	}
	return rows
}

func (m *Model) commandPaletteRow(c command, selected bool) string {
	title := c.titleFor(m.cfg.Language)
	key := c.key
	if m.w < 40 {
		key = ""
	}
	style := m.th.base
	if selected {
		style = m.th.selected
		title = "▸ " + title
	} else {
		title = "  " + title
	}
	line := title
	if key != "" {
		gap := m.w - ansiWidth(title) - ansiWidth(key) - 2
		if gap < 1 {
			gap = 1
		}
		line += strings.Repeat(" ", gap) + key
	}
	line = truncateTo(line, m.w)
	pad := m.w - ansiWidth(line)
	return style + line + strings.Repeat(" ", max(0, pad)) + sgrReset
}

func (m *Model) inputOverlayRow(prompt string, in *widgets.Input) string {
	pw := ansiWidth(prompt)
	style := m.th.panel
	row := style + prompt + renderInput(in, max(1, m.w-pw), m.th.panelOn, m.th.panelOn+m.th.cursor) + sgrReset
	return padANSI(row, m.w, style)
}

func (m *Model) drawInputOverlays(rows []string) []string {
	if len(rows) == 0 {
		return rows
	}
	switch m.ov.kind {
	case overlayCommands:
		inputY, listY, _ := m.commandPaletteGeometry()
		if inputY < 0 {
			return rows
		}
		listRows := m.commandPaletteRows()
		for i, r := range listRows {
			y := listY + i
			if y >= 0 && y < len(rows) {
				rows[y] = r
			}
		}
		rows[inputY] = m.inputOverlayRow("> ", &m.ov.input)
	case overlaySaveAs:
		y := m.h - 2
		if y >= 0 && y < len(rows) {
			rows[y] = m.inputOverlayRow(m.tr("Сохранить как: ", "Save as: "), &m.ov.input)
		}
	}
	return rows
}
