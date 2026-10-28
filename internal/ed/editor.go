package ed

import (
	"strings"
	"time"
)

// Editor — курсор, выделение и транзакции поверх Buffer.
type Editor struct {
	Buf      *Buffer
	Cursor   Pos
	Sel      Selection
	TabWidth int

	goalCol  int
	goalLine int
	hasGoal  bool

	undo        []undoEntry
	redo        []undoEntry
	groupDepth  int
	pending     *undoEntry
	now         func() time.Time
	groupWindow time.Duration
}

// NewEditor создаёт редактор с курсором в начале буфера.
func NewEditor(b *Buffer) *Editor {
	if b == nil {
		b = New("")
	}
	return &Editor{Buf: b, Cursor: b.Clamp(Pos{}), TabWidth: 4}
}

func normalizeNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func (e *Editor) setCursorRaw(p Pos) {
	e.Cursor = e.Buf.Clamp(p)
}

func (e *Editor) dropGoal() {
	e.hasGoal = false
}

func (e *Editor) cursor() Pos {
	return e.Buf.Clamp(e.Cursor)
}

// editRange применяет замену, ведёт undo и пересчитывает курсор/выделение.
func (e *Editor) editRange(r Range, text string, coalesce, cursorToEnd, keepSel bool) bool {
	if e.Buf.ReadOnly {
		return false
	}
	text = normalizeNewlines(text)
	r = e.Buf.orderedRange(r)
	old := e.Buf.textRange(r)
	newEnd := e.Buf.posAfter(r.Start, text)
	if old == text {
		if cursorToEnd {
			e.setCursorRaw(newEnd)
		}
		if !keepSel {
			e.Sel = Selection{}
		}
		e.dropGoal()
		return false
	}

	before := e.state()
	beforeRev := e.Buf.revision
	// Позиции считаем до правки: иначе clamp прижмёт старые колонки к уже
	// укороченной строке.
	newCursor := newEnd
	if !cursorToEnd {
		newCursor = e.Buf.mapPos(before.Cursor, r, text)
	}
	keep := keepSel && before.Sel.Active
	newAnchor := Pos{}
	if keep {
		newAnchor = e.Buf.mapPos(before.Sel.Anchor, r, text)
	}

	e.Buf.replace(r, text)
	afterRev := e.Buf.revision
	e.Cursor = e.Buf.Clamp(newCursor)
	if keep {
		newAnchor = e.Buf.Clamp(newAnchor)
	}
	if keep && newAnchor != e.Cursor {
		e.Sel = Selection{Anchor: newAnchor, Cursor: e.Cursor, Active: true}
	} else {
		e.Sel = Selection{}
	}
	e.commit(before, beforeRev, afterRev, change{start: r.Start, old: old, new: text}, coalesce)
	e.dropGoal()
	return true
}

func (e *Editor) selectionOrCursor() Range {
	if e.HasSelection() {
		return e.SelectionRange()
	}
	p := e.cursor()
	return Range{Start: p, End: p}
}

// Insert вставляет текст, заменяя выделение, одной транзакцией.
func (e *Editor) Insert(text string) {
	text = normalizeNewlines(text)
	// Группируем посимвольный набор. Замена выделения первым символом — тоже
	// часть этого набора: Ctrl-A → «XYZ» → Ctrl-Z должен вернуть всё.
	coalesce := len([]rune(text)) == 1
	e.editRange(e.selectionOrCursor(), text, coalesce, true, false)
}

// Paste вставляет текст из буфера обмена одной транзакцией.
func (e *Editor) Paste(text string) {
	e.editRange(e.selectionOrCursor(), normalizeNewlines(text), false, true, false)
}

// ReplaceRange заменяет диапазон текстом (LSP, replace-all).
func (e *Editor) ReplaceRange(r Range, text string) {
	e.editRange(r, text, false, false, false)
}

// DeleteBackward удаляет выделение или кластер перед курсором.
func (e *Editor) DeleteBackward() bool {
	if e.Buf.ReadOnly {
		return false
	}
	if e.HasSelection() {
		return e.editRange(e.SelectionRange(), "", false, true, false)
	}
	cur := e.cursor()
	prev, ok := e.Buf.prevPos(cur)
	if !ok {
		return false
	}
	return e.editRange(Range{Start: prev, End: cur}, "", true, true, false)
}

// DeleteForward удаляет выделение или кластер под курсором.
func (e *Editor) DeleteForward() bool {
	if e.Buf.ReadOnly {
		return false
	}
	if e.HasSelection() {
		return e.editRange(e.SelectionRange(), "", false, true, false)
	}
	cur := e.cursor()
	next, ok := e.Buf.nextPos(cur)
	if !ok {
		return false
	}
	return e.editRange(Range{Start: cur, End: next}, "", true, true, false)
}

// SetCursor ставит курсор и снимает выделение.
func (e *Editor) SetCursor(p Pos) {
	e.Cursor = e.Buf.Clamp(p)
	e.Sel = Selection{}
	e.dropGoal()
}

// SetSelection выделяет от anchor до cursor.
func (e *Editor) SetSelection(anchor, cursor Pos) {
	anchor = e.Buf.Clamp(anchor)
	cursor = e.Buf.Clamp(cursor)
	e.Cursor = cursor
	if anchor == cursor {
		e.Sel = Selection{}
	} else {
		e.Sel = Selection{Anchor: anchor, Cursor: cursor, Active: true}
	}
	e.dropGoal()
}

// ClearSelection снимает выделение.
func (e *Editor) ClearSelection() {
	e.Sel = Selection{}
}

// HasSelection — есть ли непустое выделение.
func (e *Editor) HasSelection() bool {
	return e.Sel.Active && e.Sel.Anchor != e.Cursor
}

// SelectionRange — выделение в порядке возрастания.
func (e *Editor) SelectionRange() Range {
	if !e.HasSelection() {
		p := e.cursor()
		return Range{Start: p, End: p}
	}
	a, c := e.Sel.Anchor, e.cursor()
	if posLess(c, a) {
		a, c = c, a
	}
	return Range{Start: a, End: c}
}

// SelectedText — текст выделения с "\n" между строками.
func (e *Editor) SelectedText() string {
	if !e.HasSelection() {
		return ""
	}
	return e.Buf.textRange(e.SelectionRange())
}

// SelectAll выделяет весь буфер.
func (e *Editor) SelectAll() {
	end := Pos{e.Buf.LastLine(), len(e.Buf.Lines[e.Buf.LastLine()])}
	e.SetSelection(Pos{}, end)
}

// SelectWord выделяет слово под курсором (двойной клик).
func (e *Editor) SelectWord() {
	cur := e.cursor()
	line := e.Buf.Lines[cur.Line]
	if len(line) == 0 {
		e.SetSelection(cur, cur)
		return
	}
	col := cur.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	start, end := col, col+1
	if isSelectWordRune(line[col]) {
		for start > 0 && isSelectWordRune(line[start-1]) {
			start--
		}
		for end < len(line) && isSelectWordRune(line[end]) {
			end++
		}
	}
	start = snapClusterStart(line, start)
	end = snapClusterEnd(line, end)
	e.SetSelection(Pos{cur.Line, start}, Pos{cur.Line, end})
}

// SelectLine выделяет строку вместе с переводом строки (тройной клик).
func (e *Editor) SelectLine() {
	cur := e.cursor()
	end := Pos{cur.Line, len(e.Buf.Lines[cur.Line])}
	if cur.Line < e.Buf.LastLine() {
		end = Pos{cur.Line + 1, 0}
	}
	e.SetSelection(Pos{cur.Line, 0}, end)
}

func isSelectWordRune(r rune) bool {
	return isWord(r) || r == '-'
}

// moveTo перемещает курсор; extend сохраняет якорь выделения.
func (e *Editor) moveTo(p Pos, extend bool) {
	p = e.Buf.Clamp(p)
	anchor := e.Cursor
	if extend && e.HasSelection() {
		anchor = e.Sel.Anchor
	}
	e.Cursor = p
	if extend && anchor != p {
		e.Sel = Selection{Anchor: anchor, Cursor: p, Active: true}
	} else {
		e.Sel = Selection{}
	}
	e.dropGoal()
}

// collapse снимает выделение, ставя курсор в p.
func (e *Editor) collapse(p Pos) {
	e.moveTo(p, false)
}

// MoveLeft — на кластер влево; без Shift сворачивает выделение к началу.
func (e *Editor) MoveLeft(extend bool) {
	if !extend && e.HasSelection() {
		e.collapse(e.SelectionRange().Start)
		return
	}
	prev, ok := e.Buf.prevPos(e.Cursor)
	if !ok {
		if !extend {
			e.collapse(e.cursor())
		}
		return
	}
	e.moveTo(prev, extend)
}

// MoveRight — на кластер вправо; без Shift сворачивает выделение к концу.
func (e *Editor) MoveRight(extend bool) {
	if !extend && e.HasSelection() {
		e.collapse(e.SelectionRange().End)
		return
	}
	next, ok := e.Buf.nextPos(e.Cursor)
	if !ok {
		if !extend {
			e.collapse(e.cursor())
		}
		return
	}
	e.moveTo(next, extend)
}

// MoveWordLeft — на слово влево.
func (e *Editor) MoveWordLeft(extend bool) {
	if !extend && e.HasSelection() {
		e.collapse(e.SelectionRange().Start)
		return
	}
	p := e.Buf.WordLeft(e.Cursor)
	p = e.Buf.Clamp(p)
	p.Col = snapClusterStart(e.Buf.Lines[p.Line], p.Col)
	e.moveTo(p, extend)
}

// MoveWordRight — на слово вправо.
func (e *Editor) MoveWordRight(extend bool) {
	if !extend && e.HasSelection() {
		e.collapse(e.SelectionRange().End)
		return
	}
	p := e.Buf.WordRight(e.Cursor)
	p = e.Buf.Clamp(p)
	p.Col = snapClusterEnd(e.Buf.Lines[p.Line], p.Col)
	e.moveTo(p, extend)
}

// vertical двигает курсор по строкам, сохраняя желаемую колонку.
func (e *Editor) vertical(delta int, extend bool) {
	if delta == 0 {
		return
	}
	cur := e.cursor()
	if !e.hasGoal || e.goalLine != cur.Line {
		e.goalCol = cur.Col
	}
	line := cur.Line + delta
	if line < 0 {
		line = 0
	}
	if line > e.Buf.LastLine() {
		line = e.Buf.LastLine()
	}
	col := e.goalCol
	if col > len(e.Buf.Lines[line]) {
		col = len(e.Buf.Lines[line])
	}
	col = snapClusterStart(e.Buf.Lines[line], col)
	p := Pos{line, col}

	anchor := e.Cursor
	if extend && e.HasSelection() {
		anchor = e.Sel.Anchor
	}
	e.Cursor = p
	if extend && anchor != p {
		e.Sel = Selection{Anchor: anchor, Cursor: p, Active: true}
	} else {
		e.Sel = Selection{}
	}
	e.goalLine = line
	e.hasGoal = true
}

// MoveUp — на строку вверх.
func (e *Editor) MoveUp(extend bool) { e.vertical(-1, extend) }

// MoveDown — на строку вниз.
func (e *Editor) MoveDown(extend bool) { e.vertical(1, extend) }

// MoveHome — в начало строки.
func (e *Editor) MoveHome(extend bool) { e.moveTo(e.Buf.LineStart(e.Cursor), extend) }

// MoveEnd — в конец строки.
func (e *Editor) MoveEnd(extend bool) { e.moveTo(e.Buf.LineEnd(e.Cursor), extend) }

// MoveDocStart — в начало файла.
func (e *Editor) MoveDocStart(extend bool) { e.moveTo(Pos{}, extend) }

// MoveDocEnd — в конец файла.
func (e *Editor) MoveDocEnd(extend bool) {
	p := Pos{e.Buf.LastLine(), len(e.Buf.Lines[e.Buf.LastLine()])}
	e.moveTo(p, extend)
}

// MovePage — на delta строк (высота окна передаётся вызывающим).
func (e *Editor) MovePage(delta int, extend bool) { e.vertical(delta, extend) }

// Cut вырезает выделение и возвращает текст.
func (e *Editor) Cut() string {
	if e.Buf.ReadOnly || !e.HasSelection() {
		return ""
	}
	r := e.SelectionRange()
	text := e.Buf.textRange(r)
	e.editRange(r, "", false, true, false)
	return text
}

// Copy возвращает текст выделения, не меняя буфер.
func (e *Editor) Copy() string {
	if !e.HasSelection() {
		return ""
	}
	return e.Buf.textRange(e.SelectionRange())
}
