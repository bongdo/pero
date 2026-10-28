package ui

import (
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/bongdo/pero/internal/ed"
)

type pasteWaitMsg struct {
	seq int
}

type pasteFallbackMsg struct {
	text string
}

type clipboardWrittenMsg struct{}

// copyTextCmd сначала просит OSC52, параллельно пробует системный буфер как
// fallback (в ssh OSC52 - единственный рабочий путь).
func copyTextCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return tea.Batch(tea.SetClipboard(s), func() tea.Msg {
		_ = clipboard.WriteAll(s)
		return clipboardWrittenMsg{}
	})
}

func (m *Model) copySelection(cut bool) tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if cut && !m.canEdit() {
		return nil
	}
	text := t.ed.Copy()
	if cut {
		t.ed.Cut()
		m.afterEdit()
	}
	return copyTextCmd(text)
}

func (m *Model) pasteClipboard() tea.Cmd {
	if !m.canEdit() {
		return nil
	}
	m.pasteSeq++
	seq := m.pasteSeq
	m.pendingPaste = true
	m.pasteTab = m.tab()
	return tea.Batch(
		func() tea.Msg { return tea.ReadClipboard() },
		tea.Tick(180*time.Millisecond, func(time.Time) tea.Msg {
			return pasteWaitMsg{seq: seq}
		}),
	)
}

// pasteText - bracketed paste терминала. Текст идёт в тот виджет, который
// сейчас в фокусе, и никогда не меняет буфер чужой вкладки.
func (m *Model) pasteText(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	switch m.ov.kind {
	case overlayCommands, overlaySaveAs:
		m.ov.input.Insert(s)
		return nil
	case overlayThemes:
		m.ov.input.Insert(s)
		m.ov.sel = 0
		m.previewTheme()
		return nil
	case overlayOpen:
		m.ov.input.Insert(s)
		m.recomputeOpenMatches()
		m.ov.sel = 0
		return nil
	case overlayFind:
		if m.ov.focus2 {
			m.ov.input2.Insert(s)
			m.recomputeFind()
		} else {
			m.ov.input.Insert(s)
			m.recomputeFind()
		}
		return nil
	case overlayCompletion:
		if t := m.tab(); t != nil {
			t.comp.query += s
			m.refreshCompletion(t)
		}
		return nil
	case overlayNone:
		m.insertInto(m.tab(), s)
		return nil
	}
	return nil
}

// insertInto вставляет в конкретную вкладку. Если вкладку успели закрыть,
// текст не теряется молча в другой вкладке.
func (m *Model) insertInto(t *Tab, s string) bool {
	if t == nil || !m.tabAlive(t) {
		return false
	}
	if t.ed.Buf.ReadOnly {
		if t == m.tab() {
			m.canEdit()
		}
		return false
	}
	t.ed.Insert(s)
	m.afterEditTab(t)
	return true
}

func (m *Model) onClipboard(content string) tea.Cmd {
	if !m.pendingPaste || content == "" {
		return nil
	}
	m.pendingPaste = false
	t := m.pasteTab
	m.pasteTab = nil
	if !m.insertInto(t, content) {
		m.notifyWarn(m.tr("вставка отменена: вкладка закрыта", "paste cancelled: tab was closed"))
	}
	return nil
}

func (m *Model) onPasteWait(msg pasteWaitMsg) tea.Cmd {
	if !m.pendingPaste || msg.seq != m.pasteSeq {
		return nil
	}
	return func() tea.Msg {
		text, _ := clipboard.ReadAll()
		return pasteFallbackMsg{text: text}
	}
}

func (m *Model) onPasteFallback(msg pasteFallbackMsg) tea.Cmd {
	if !m.pendingPaste {
		return nil
	}
	m.pendingPaste = false
	t := m.pasteTab
	m.pasteTab = nil
	if msg.text == "" {
		m.notifyInfo(m.tr("буфер обмена пуст или недоступен", "clipboard is empty or unavailable"))
		return nil
	}
	if !m.insertInto(t, msg.text) {
		m.notifyWarn(m.tr("вставка отменена: вкладка закрыта", "paste cancelled: tab was closed"))
	}
	return nil
}

func (m *Model) cutLine() tea.Cmd {
	t := m.tab()
	if t == nil || !m.canEdit() {
		return nil
	}
	e := t.ed
	line := e.Cursor.Line
	text := string(e.Buf.Line(line))
	last := line == e.Buf.LastLine()
	if !last {
		e.ReplaceRange(ed.Range{Start: ed.Pos{Line: line}, End: ed.Pos{Line: line + 1}}, "")
		text += "\n"
	} else {
		e.ReplaceRange(ed.Range{Start: ed.Pos{Line: line}, End: ed.Pos{Line: line, Col: len(e.Buf.Line(line))}}, "")
	}
	e.SetCursor(ed.Pos{Line: line})
	m.afterEdit()
	return copyTextCmd(text)
}

// deleteWordLeft удаляет слово слева (Alt-Backspace) одной транзакцией.
func (m *Model) deleteWordLeft() {
	t := m.tab()
	if t == nil || !m.canEdit() {
		return
	}
	e := t.ed
	start := e.Buf.WordLeft(e.Cursor)
	if start == e.Cursor {
		return
	}
	e.SetSelection(start, e.Cursor)
	e.Cut()
	e.SetCursor(start)
	m.afterEdit()
}

func (m *Model) duplicateLine() {
	t := m.tab()
	if t == nil || !m.canEdit() {
		return
	}
	e := t.ed
	line := e.Cursor.Line
	col := e.Cursor.Col
	text := string(e.Buf.Line(line))
	e.SetCursor(ed.Pos{Line: line, Col: len(e.Buf.Line(line))})
	e.Insert("\n" + text)
	e.SetCursor(ed.Pos{Line: line + 1, Col: col})
	m.afterEdit()
}

// tryAutoPair вставляет пару вместо печатного символа. true - символ уже
// обработан (включая read-only тост), false - обычная вставка.
func (m *Model) tryAutoPair(msg tea.KeyPressMsg) bool {
	if !m.cfg.Editor.AutoPairs || !printable(msg) {
		return false
	}
	rs := []rune(msg.Text)
	if len(rs) != 1 {
		return false
	}
	r := rs[0]
	open, close, ok := autoPairFor(r)
	closer := r == ')' || r == ']' || r == '}'
	if !ok && !closer {
		return false
	}
	t := m.tab()
	if t == nil {
		return false
	}
	if t.ed.Buf.ReadOnly {
		m.canEdit()
		return true
	}
	// Закрывающая скобка перед такой же - перешагиваем, а не дублируем.
	if closer && !t.ed.HasSelection() {
		line := t.ed.Buf.Line(t.ed.Cursor.Line)
		if t.ed.Cursor.Col < len(line) && line[t.ed.Cursor.Col] == r {
			t.ed.SetCursor(ed.Pos{Line: t.ed.Cursor.Line, Col: t.ed.Cursor.Col + 1})
			m.afterMove()
			return true
		}
		return false
	}
	// Апостроф в don't - не начало пары. С выделением оборачиваем всегда.
	if open == "'" && !t.ed.HasSelection() && apostropheJoinsText(t.ed) {
		return false
	}
	if !t.ed.InsertPair(open, close) {
		return false
	}
	m.afterEdit()
	return true
}

func autoPairFor(r rune) (open, close string, ok bool) {
	switch r {
	case '(':
		return "(", ")", true
	case '[':
		return "[", "]", true
	case '{':
		return "{", "}", true
	case '"', '\'', '`':
		return string(r), string(r), true
	}
	return "", "", false
}

func apostropheJoinsText(e *ed.Editor) bool {
	line := e.Buf.Line(e.Cursor.Line)
	if e.Cursor.Col > 0 && isWordRune(line[e.Cursor.Col-1]) {
		return true
	}
	return e.Cursor.Col < len(line) && isWordRune(line[e.Cursor.Col])
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (m *Model) undo() {
	if t := m.tab(); t != nil {
		if t.ed.Undo() {
			m.afterEdit()
			m.clampScroll()
		}
	}
}

func (m *Model) redo() {
	if t := m.tab(); t != nil {
		if t.ed.Redo() {
			m.afterEdit()
			m.clampScroll()
		}
	}
}

func (m *Model) selectAll() {
	m.cancelGhost()
	if t := m.tab(); t != nil {
		t.ed.SelectAll()
	}
}
