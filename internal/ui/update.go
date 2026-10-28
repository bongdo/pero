package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/markdown"
	"github.com/bongdo/pero/internal/widgets"
)

func printable(msg tea.KeyPressMsg) bool {
	return msg.Text != "" &&
		!msg.Mod.Contains(tea.ModCtrl) &&
		!msg.Mod.Contains(tea.ModAlt) &&
		!msg.Mod.Contains(tea.ModMeta) &&
		!msg.Mod.Contains(tea.ModSuper)
}

func hasModKey(msg tea.KeyPressMsg) bool {
	return msg.Mod.Contains(tea.ModCtrl) || msg.Mod.Contains(tea.ModAlt) ||
		msg.Mod.Contains(tea.ModMeta) || msg.Mod.Contains(tea.ModSuper)
}

// key - маршрутизация: help → confirm → popup/input → текст.
func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	if t := m.tab(); t != nil {
		t.free = false
	}
	k := msg.Keystroke()
	if k == "ctrl+e" && m.ov.kind != overlayNone && m.ov.kind != overlayCommands &&
		m.ov.kind != overlayHelp && m.ov.kind != overlayConfirm && m.ov.kind != overlayThemes {
		m.pendingClose = -1
		return m.runCommand("palette.commands")
	}
	switch m.ov.kind {
	case overlayHelp:
		return m.helpKey(msg)
	case overlayConfirm:
		return m.confirmKey(msg)
	case overlayMenu:
		return m.menuKey(msg)
	case overlayCompletion:
		return m.completionKey(msg)
	case overlayHover:
		return m.hoverKey(msg)
	case overlayLocs:
		return m.locationsKey(msg)
	case overlayQuickfix:
		return m.quickfixKey(msg)
	case overlayAI:
		return m.aiKey(msg)
	case overlayNone:
	default:
		return m.overlayKey(msg)
	}

	if k == "esc" {
		return m.escKey()
	}
	if id, ok := m.bindings[k]; ok {
		return m.runCommand(id)
	}
	// Alt-G и Shift-Ctrl-S приходят с заглавной буквой, а биндинги хранятся
	// в нижнем регистре. Печатный текст без модификаторов сюда не попадает.
	if hasModKey(msg) {
		if id, ok := m.bindings[strings.ToLower(k)]; ok {
			return m.runCommand(id)
		}
	}
	if printable(msg) {
		if m.tryAutoPair(msg) {
			return nil
		}
		m.insertText(msg.Text)
		return nil
	}
	switch k {
	case "enter", "shift+enter":
		m.newline()
	case "backspace":
		if t := m.tab(); t != nil {
			if t.ed.Buf.ReadOnly {
				m.canEdit()
				return nil
			}
			if m.cfg.Editor.AutoPairs && !t.ed.HasSelection() {
				for _, p := range [][2]rune{{'(', ')'}, {'[', ']'}, {'{', '}'}, {'"', '"'}, {'\'', '\''}, {'`', '`'}} {
					if t.ed.DeleteEmptyPair(p[0], p[1]) {
						m.afterEdit()
						return nil
					}
				}
			}
			if t.ed.DeleteBackward() {
				m.afterEdit()
			}
		}
	case "delete":
		if t := m.tab(); t != nil {
			if t.ed.Buf.ReadOnly {
				m.canEdit()
				return nil
			}
			if t.ed.DeleteForward() {
				m.afterEdit()
			}
		}
	}
	return nil
}

func (m *Model) overlayKey(msg tea.KeyPressMsg) tea.Cmd {
	switch m.ov.kind {
	case overlayCommands, overlaySaveAs:
		return m.paletteKey(msg)
	case overlayOpen:
		return m.openKey(msg)
	case overlayFind:
		return m.findKey(msg)
	case overlayThemes:
		return m.themeKey(msg)
	}
	return nil
}

// escKey - цепочка: AI в ожидании → оверлей (закрыт выше) → ошибка →
// выделение → ghost.
func (m *Model) escKey() tea.Cmd {
	m.pendingClose = -1
	m.pendingPaste = false
	if m.aiCancel != nil {
		m.aiCancel()
		m.aiCancel = nil
		m.aiGen++
		if m.toast.Kind == "progress" {
			m.toast = widgets.Toast{}
		}
		m.notifyInfo(m.tr("AI отменён", "AI cancelled"))
		return nil
	}
	if m.toast.Kind == "error" && m.toast.Until.IsZero() {
		m.toast = widgets.Toast{}
		return nil
	}
	if t := m.tab(); t != nil {
		if t.ed.HasSelection() {
			t.ed.ClearSelection()
			return nil
		}
	}
	m.cancelGhost()
	return nil
}

func (m *Model) newline() {
	t := m.tab()
	if t == nil {
		return
	}
	if t.ed.Buf.ReadOnly {
		m.canEdit()
		return
	}
	e := t.ed
	cur := e.Cursor
	line := e.Buf.Line(cur.Line)
	step := m.indentStep()

	// Пустая пара на строке разбивается на три строки с отступом.
	if m.cfg.Editor.AutoPairs {
		indent := e.Buf.Indent(cur)
		for _, p := range [][2]rune{{'{', '}'}, {'(', ')'}, {'[', ']'}} {
			if e.BreakPair(p[0], p[1], indent, step) {
				m.afterEdit()
				return
			}
		}
	}

	if m.cfg.Markdown.ContinueList && m.isProsePath(e.Buf.Path) && !markdown.FenceState(e.Buf.Lines)[cur.Line] {
		indent := leadingRunes(line)
		rest := string(line[indent:])
		if prefix, ok := checkboxPrefix(rest); ok {
			if strings.TrimSpace(rest[len(prefix):]) == "" && cur.Col >= indent+len(prefix) {
				e.ReplaceRange(ed.Range{
					Start: ed.Pos{Line: cur.Line, Col: indent},
					End:   ed.Pos{Line: cur.Line, Col: indent + len(prefix)},
				}, "")
			} else {
				e.Insert("\n" + string(line[:indent]) + prefix)
			}
			m.afterEdit()
			return
		}
		insert, clear := markdown.ContinueList(string(line))
		if clear {
			if prefix, ok := markdown.ListPrefix(rest); ok {
				e.ReplaceRange(ed.Range{
					Start: ed.Pos{Line: cur.Line, Col: indent},
					End:   ed.Pos{Line: cur.Line, Col: indent + len([]rune(prefix))},
				}, "")
				m.afterEdit()
				return
			}
		} else if insert != "" {
			e.Insert(insert)
			m.afterEdit()
			return
		}
	}

	indent := e.Buf.Indent(cur)
	if !m.isProsePath(e.Buf.Path) && blockOpened(line, cur.Col) {
		indent += step
	}
	e.Insert("\n" + indent)
	m.afterEdit()
}

func (m *Model) indentStep() string {
	if m.cfg.Editor.Tabstospaces {
		return strings.Repeat(" ", m.cfg.Editor.Tabsize)
	}
	return "\t"
}

// blockOpened - строка до курсора кончается на открывающую скобку.
func blockOpened(line []rune, col int) bool {
	s := strings.TrimSpace(string(line[:min(col, len(line))]))
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '{', '[', '(':
		return true
	}
	return false
}

func leadingRunes(line []rune) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// checkboxPrefix - полный префикс чекбокса, включая маркер списка.
func checkboxPrefix(rest string) (string, bool) {
	for _, p := range []string{"- [ ] ", "- [x] ", "- [X] ", "* [ ] ", "* [x] ", "+ [ ] ", "+ [x] "} {
		if strings.HasPrefix(rest, p) {
			return p, true
		}
	}
	for _, p := range []string{"[ ] ", "[x] ", "[X] "} {
		if strings.HasPrefix(rest, p) {
			return p, true
		}
	}
	return "", false
}

func (m *Model) indent() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if !m.canEdit() {
		return nil
	}
	e := t.ed
	if t.ghost.text != "" {
		e.Insert(t.ghost.text)
		t.ghost = ghostState{}
		m.afterEdit()
		return nil
	}
	cur := e.Cursor
	line := e.Buf.Line(cur.Line)
	indent := leadingRunes(line)
	proseList := m.isProsePath(e.Buf.Path) && (lineHasList(string(line[indent:])))
	if e.HasSelection() || proseList {
		e.IndentSelection()
		m.afterEdit()
		return nil
	}
	if m.cfg.Editor.Tabstospaces {
		cols := displayCols(line, m.cfg.Editor.Tabsize)
		x := cols[max(0, min(cur.Col, len(cols)-1))]
		n := m.cfg.Editor.Tabsize - x%m.cfg.Editor.Tabsize
		if n == 0 {
			n = m.cfg.Editor.Tabsize
		}
		e.Insert(strings.Repeat(" ", n))
	} else {
		e.Insert("\t")
	}
	m.afterEdit()
	return nil
}

func lineHasList(rest string) bool {
	if _, ok := markdown.ListPrefix(rest); ok {
		return true
	}
	_, ok := checkboxPrefix(rest)
	return ok
}

func (m *Model) outdent() {
	if !m.canEdit() {
		return
	}
	if t := m.tab(); t != nil {
		if t.ed.OutdentSelection() {
			m.afterEdit()
		}
	}
}

func (m *Model) toggleCheckbox() {
	t := m.tab()
	if t == nil || !m.cfg.Markdown.Checkbox || !m.isProsePath(t.ed.Buf.Path) {
		return
	}
	if !m.canEdit() {
		return
	}
	line := t.ed.Cursor.Line
	next, ok := markdown.ToggleCheckboxAt(t.ed.Buf.Lines, line)
	if !ok {
		if markdown.IsCheckbox(string(t.ed.Buf.Line(line))) {
			m.notifyInfo(m.tr("внутри кода чекбокс не меняется", "checkbox cannot be toggled inside code"))
		}
		return
	}
	old := string(t.ed.Buf.Line(line))
	if next == old {
		return
	}
	t.ed.ReplaceRange(ed.Range{
		Start: ed.Pos{Line: line, Col: 0},
		End:   ed.Pos{Line: line, Col: len(t.ed.Buf.Line(line))},
	}, next)
	m.afterEdit()
}

func (m *Model) collapseSpaces() {
	if !m.canEdit() {
		return
	}
	if t := m.tab(); t != nil && m.isProsePath(t.ed.Buf.Path) {
		if t.ed.CollapseSpaces() {
			m.afterEdit()
		}
	}
}

func (m *Model) toggleWrap() {
	t := m.tab()
	if t == nil {
		return
	}
	t.wrap = !t.wrap
	t.left = 0
	t.wrapCache.valid = false
	m.clampScroll()
	if t.wrap {
		m.notifyInfo(m.tr("перенос строк включён", "soft wrap on"))
	} else {
		m.notifyInfo(m.tr("перенос строк выключен", "soft wrap off"))
	}
}

func (m *Model) toggleGhost() {
	m.cfg.Editor.Ghost = !m.cfg.Editor.Ghost
	if m.cfg.Editor.Ghost {
		m.scheduleGhost()
		m.notifyInfo(m.tr("подсказки включены", "suggestions on"))
	} else {
		m.cancelGhost()
		m.notifyInfo(m.tr("подсказки выключены", "suggestions off"))
	}
}

func (m *Model) findAgain(forward bool) {
	t := m.tab()
	if t == nil {
		return
	}
	if m.lastQuery == "" {
		m.notifyInfo(m.tr("нет запроса: Ctrl-F", "no query: Ctrl-F"))
		return
	}
	from := t.ed.Cursor
	if t.ed.HasSelection() {
		if forward {
			from = t.ed.SelectionRange().End
		} else {
			from = t.ed.SelectionRange().Start
		}
	}
	var (
		r  ed.Range
		ok bool
	)
	if forward {
		r, ok = t.ed.FindNext(m.lastQuery, from, true)
	} else {
		r, ok = t.ed.FindPrev(m.lastQuery, from, true)
	}
	if !ok {
		m.notifyInfo(m.tr("не найдено: ", "not found: ") + m.lastQuery)
		return
	}
	t.ed.SetSelection(r.Start, r.End)
	m.clampScroll()
}

func (m *Model) jumpDiagnostic(delta int) tea.Cmd {
	t := m.tab()
	if t == nil {
		m.notifyInfo(m.tr("диагностик нет", "no diagnostics"))
		return nil
	}
	diags := t.visibleDiags()
	if len(diags) == 0 {
		m.notifyInfo(m.tr("диагностик нет", "no diagnostics"))
		return nil
	}
	type dp struct {
		pos ed.Pos
		msg string
	}
	points := make([]dp, 0, len(diags))
	for _, d := range diags {
		p := t.ed.Buf.PosFromUTF16(int(d.Range.Start.Line), int(d.Range.Start.Character))
		points = append(points, dp{pos: p, msg: d.Message})
	}
	cur := t.ed.Cursor
	best := -1
	if delta >= 0 {
		for i, p := range points {
			if posLess(cur, p.pos) && (best < 0 || posLess(p.pos, points[best].pos)) {
				best = i
			}
		}
		if best < 0 {
			best = 0
			for i, p := range points {
				if posLess(p.pos, points[best].pos) {
					best = i
				}
			}
		}
	} else {
		for i, p := range points {
			if posLess(p.pos, cur) && (best < 0 || posLess(points[best].pos, p.pos)) {
				best = i
			}
		}
		if best < 0 {
			best = 0
			for i, p := range points {
				if posLess(points[best].pos, p.pos) {
					best = i
				}
			}
		}
	}
	t.ed.SetCursor(points[best].pos)
	t.free = false
	m.ensureVisible(m.layout())
	m.clampScroll()
	m.scheduleGhost()
	if msg := strings.TrimSpace(points[best].msg); msg != "" {
		m.notifyInfo(msg)
	}
	return nil
}
