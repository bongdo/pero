package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/widgets"
)

func (m *Model) openFind(replace bool) {
	m.closeOverlays()
	m.ov.kind = overlayFind
	m.ov.replace = replace
	m.ov.focus2 = false
	t := m.tab()
	if t != nil && t.ed.HasSelection() {
		sel := strings.ReplaceAll(t.ed.SelectedText(), "\n", " ")
		m.ov.input.Set(strings.TrimSpace(sanitizeOneLine(sel)))
	} else if m.lastQuery != "" {
		m.ov.input.Set(m.lastQuery)
	}
	m.ov.input.Placeholder = m.tr("что найти", "what to find")
	m.ov.input2.Placeholder = m.tr("на что заменить", "replace with")
	m.recomputeFind()
}

func (m *Model) recomputeFind() {
	t := m.tab()
	q := m.ov.input.Value()
	if t == nil || q == "" {
		m.ov.findMatches = nil
		m.ov.findLines = nil
		m.ov.cur = 0
		if q != "" {
			m.lastQuery = q
		}
		return
	}
	m.ov.findMatches = t.ed.FindAll(q, true)
	m.ov.findLines = make(map[int][]ed.Range, len(m.ov.findMatches))
	for _, r := range m.ov.findMatches {
		if r.Start.Line == r.End.Line {
			m.ov.findLines[r.Start.Line] = append(m.ov.findLines[r.Start.Line], r)
		}
	}
	m.lastQuery = q
	cur := t.ed.Cursor
	idx := 0
	for i, r := range m.ov.findMatches {
		if posLessEq(cur, r.Start) {
			idx = i
			break
		}
	}
	m.ov.cur = idx
}

func (m *Model) closeFind() {
	if q := strings.TrimSpace(m.ov.input.Value()); q != "" {
		m.lastQuery = q
	}
	m.closeOverlays()
}

func (m *Model) findKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.Keystroke()
	switch k {
	case "esc":
		m.closeFind()
		return nil
	case "tab":
		if m.ov.replace {
			m.ov.focus2 = !m.ov.focus2
		}
		return nil
	case "enter":
		if m.ov.replace && m.ov.focus2 {
			m.replaceOne()
			return nil
		}
		m.findStep(1)
		return nil
	case "shift+enter", "shift+f3":
		m.findStep(-1)
		return nil
	case "f3":
		m.findStep(1)
		return nil
	case "ctrl+a":
		if m.ov.replace {
			return m.replaceAll()
		}
		m.focusedFindInput().Home()
		return nil
	case "ctrl+z":
		m.undo()
		m.recomputeFind()
		return nil
	case "ctrl+y":
		m.redo()
		m.recomputeFind()
		return nil
	case "up":
		m.findStep(-1)
		return nil
	case "down":
		m.findStep(1)
		return nil
	case "ctrl+u":
		m.focusedFindInput().Clear()
		m.recomputeFind()
	case "ctrl+w":
		m.focusedFindInput().DeleteWordLeft()
		m.recomputeFind()
		return nil
	case "backspace":
		m.focusedFindInput().Backspace()
		m.recomputeFind()
		return nil
	case "delete":
		m.focusedFindInput().Delete()
		m.recomputeFind()
		return nil
	case "left":
		m.focusedFindInput().MoveLeft()
		return nil
	case "right":
		m.focusedFindInput().MoveRight()
		return nil
	case "ctrl+left":
		m.focusedFindInput().WordLeft()
		return nil
	case "ctrl+right":
		m.focusedFindInput().WordRight()
		return nil
	case "home":
		m.focusedFindInput().Home()
		return nil
	case "end":
		m.focusedFindInput().End()
		return nil
	}
	if printable(msg) {
		m.focusedFindInput().Insert(msg.Text)
		m.recomputeFind()
	}
	return nil
}

func (m *Model) focusedFindInput() *widgets.Input {
	if m.ov.focus2 {
		return &m.ov.input2
	}
	return &m.ov.input
}

func (m *Model) findStep(delta int) {
	n := len(m.ov.findMatches)
	if n == 0 {
		return
	}
	t := m.tab()
	if t == nil {
		return
	}
	if m.ov.cur < 0 || m.ov.cur >= n {
		m.ov.cur = 0
	}
	// Первый Enter выделяет текущее совпадение, следующие — листают.
	if t.ed.HasSelection() && m.ov.findMatches[m.ov.cur] == t.ed.SelectionRange() {
		m.ov.cur = (m.ov.cur + delta%n + n) % n
	}
	m.selectFindMatch(m.ov.cur)
}

func (m *Model) selectFindMatch(i int) {
	t := m.tab()
	if t == nil || i < 0 || i >= len(m.ov.findMatches) {
		return
	}
	r := m.ov.findMatches[i]
	t.ed.SetSelection(r.Start, r.End)
	m.clampScroll()
}

func (m *Model) replaceOne() {
	t := m.tab()
	if t == nil || t.ed.Buf.ReadOnly || len(m.ov.findMatches) == 0 {
		return
	}
	if m.ov.cur < 0 || m.ov.cur >= len(m.ov.findMatches) {
		m.ov.cur = 0
	}
	r := m.ov.findMatches[m.ov.cur]
	repl := m.ov.input2.Value()
	t.ed.ReplaceRange(r, repl)
	m.afterEdit()
	m.recomputeFind()
	if len(m.ov.findMatches) > 0 {
		if m.ov.cur >= len(m.ov.findMatches) {
			m.ov.cur = 0
		}
		m.selectFindMatch(m.ov.cur)
	}
}

func (m *Model) replaceAll() tea.Cmd {
	t := m.tab()
	if t == nil || t.ed.Buf.ReadOnly {
		if t != nil {
			m.notifyError(m.tr("только чтение: замена недоступна", "read-only: replace is disabled"))
		}
		return nil
	}
	q := m.ov.input.Value()
	if q == "" {
		return nil
	}
	found := t.ed.FindAll(q, true)
	if len(found) == 0 {
		m.notifyInfo(m.tr("не найдено: ", "not found: ") + q)
		return nil
	}
	repl := m.ov.input2.Value()
	t.ed.BeginGroup()
	for i := len(found) - 1; i >= 0; i-- {
		t.ed.ReplaceRange(found[i], repl)
	}
	t.ed.EndGroup()
	t.ed.ClearSelection()
	m.afterEdit()
	m.recomputeFind()
	m.notifySuccess(m.trf("заменено: %d", "replaced: %d", len(found)))
	return nil
}

func (m *Model) findRows() []string {
	if m.ov.kind != overlayFind || m.layout().findH == 0 {
		return nil
	}
	rows := make([]string, 0, 2)
	n := len(m.ov.findMatches)
	counter := m.trf("%d из %d", "%d of %d", 0, 0)
	if n > 0 {
		counter = m.trf("%d из %d", "%d of %d", m.ov.cur+1, n)
	}
	promptStyle := m.th.panelOn
	if m.ov.input.Value() != "" && n == 0 {
		promptStyle = m.th.panelErr
	}

	// Поиск.
	prompt := m.tr(" Найти: ", " Find: ")
	if m.w < 40 {
		prompt = " / "
	}
	inputStyle := m.th.panelOn
	cursorStyle := m.th.panelOn + m.th.cursor
	if m.ov.focus2 {
		inputStyle = m.th.panelDim
		cursorStyle = inputStyle
	}
	left := promptStyle + prompt + renderInput(&m.ov.input, max(1, m.w-len([]rune(prompt))-len([]rune(counter))-4), inputStyle, cursorStyle) + sgrReset
	right := counter
	if m.w >= 60 {
		if m.ov.replace {
			right += m.tr("  ↵ заменить  Ctrl-A всё  Esc", "  ↵ replace  Ctrl-A all  Esc")
		} else {
			right += m.tr("  ↵ далее  ⇧↵ назад  Esc", "  ↵ next  ⇧↵ prev  Esc")
		}
	} else if m.w >= 40 {
		right += "  ↵  Esc"
	}
	gap := m.w - ansiWidth(left) - ansiWidth(right) - 1
	if gap < 1 {
		left = truncateTo(left, max(1, m.w-ansiWidth(right)-1))
		gap = max(1, m.w-ansiWidth(left)-ansiWidth(right)-1)
	}
	row := m.th.panel + left + strings.Repeat(" ", gap) + promptStyle + right + sgrReset
	rows = append(rows, padANSI(row, m.w, m.th.panel))

	if m.ov.replace {
		prompt2 := m.tr(" Заменить: ", " Replace: ")
		if m.w < 40 {
			prompt2 = " → "
		}
		st := m.th.panelOn
		cs := st + m.th.cursor
		if !m.ov.focus2 {
			st = m.th.panelDim
			cs = st
		}
		row2 := m.th.panel + prompt2 + renderInput(&m.ov.input2, max(1, m.w-len([]rune(prompt2))-2), st, cs) + sgrReset
		rows = append(rows, padANSI(row2, m.w, m.th.panel))
	}
	return rows
}
