package ed

// Autopairs, парные скобки, «умный» Enter/Home и косметика пробелов.

// InsertPair вставляет пару: выделение оборачивается, а если под курсором уже
// стоит close — курсор просто перешагивает его. Пустые аргументы и read-only
// возвращают false. Вставка пары — одна транзакция undo.
func (e *Editor) InsertPair(open, close string) bool {
	if e.Buf.ReadOnly || open == "" || close == "" {
		return false
	}
	if e.HasSelection() {
		r := e.SelectionRange()
		sel := e.Buf.textRange(r)
		return e.editRange(r, open+sel+close, false, true, false)
	}

	cur := e.cursor()
	next := e.Buf.posAfter(cur, close)
	if e.Buf.textRange(Range{Start: cur, End: next}) == close {
		e.moveTo(next, false)
		return true
	}

	// Сначала close, потом open: cursorToEnd у второй вставки оставляет
	// курсор между ними. Обе правки — одна группа undo.
	e.BeginGroup()
	e.editRange(Range{Start: cur, End: cur}, close, false, false, false)
	e.editRange(Range{Start: cur, End: cur}, open, false, true, false)
	e.EndGroup()
	return true
}

// DeleteEmptyPair удаляет open и close, если курсор стоит ровно между ними.
func (e *Editor) DeleteEmptyPair(open, close rune) bool {
	if e.Buf.ReadOnly || e.HasSelection() {
		return false
	}
	cur := e.cursor()
	prev, at, ok := e.Buf.charBefore(cur)
	if !ok || prev != open {
		return false
	}
	next, _, ok := e.Buf.charAt(cur)
	if !ok || next != close {
		return false
	}
	return e.editRange(Range{Start: at, End: Pos{cur.Line, cur.Col + 1}}, "", true, true, false)
}

// MatchingBracket находит парную скобку для позиции p. Если p стоит на скобке,
// берётся она; иначе проверяется руна перед p (подсветка пары под курсором).
// Вложенность считается только для скобок того же типа. Экранированные
// обратным слешем скобки пропускаются. Строки и комментарии не разбираются:
// для `func f(")")` это эвристика, а не парсер. Весь буфер, O(n).
func (b *Buffer) MatchingBracket(p Pos) (Pos, bool) {
	p = b.clamp(p)
	if r, ok := b.bracketAt(p); ok {
		return b.matchBracket(p, r)
	}
	if p.Col > 0 {
		at := Pos{p.Line, p.Col - 1}
		if r, ok := b.bracketAt(at); ok {
			return b.matchBracket(at, r)
		}
	}
	return Pos{}, false
}

func (b *Buffer) bracketAt(p Pos) (rune, bool) {
	if p.Line < 0 || p.Line >= len(b.Lines) || p.Col < 0 || p.Col >= len(b.Lines[p.Line]) {
		return 0, false
	}
	r := b.Lines[p.Line][p.Col]
	if _, _, ok := bracketKind(r); !ok {
		return 0, false
	}
	return r, true
}

func (b *Buffer) matchBracket(at Pos, r rune) (Pos, bool) {
	if escapedAt(b.Lines[at.Line], at.Col) {
		return Pos{}, false
	}
	open, close, _ := bracketKind(r)
	if r == open {
		return b.matchForward(at, open, close)
	}
	return b.matchBackward(at, open, close)
}

func (b *Buffer) matchForward(at Pos, open, close rune) (Pos, bool) {
	depth := 0
	for line := at.Line; line <= b.LastLine(); line++ {
		text := b.Lines[line]
		start := 0
		if line == at.Line {
			start = at.Col
		}
		for col := start; col < len(text); col++ {
			r := text[col]
			if (r != open && r != close) || escapedAt(text, col) {
				continue
			}
			if r == open {
				depth++
				continue
			}
			depth--
			if depth == 0 {
				return Pos{line, col}, true
			}
		}
	}
	return Pos{}, false
}

func (b *Buffer) matchBackward(at Pos, open, close rune) (Pos, bool) {
	depth := 0
	for line := at.Line; line >= 0; line-- {
		text := b.Lines[line]
		start := len(text) - 1
		if line == at.Line {
			start = at.Col
		}
		for col := start; col >= 0; col-- {
			r := text[col]
			if (r != open && r != close) || escapedAt(text, col) {
				continue
			}
			if r == close {
				depth++
				continue
			}
			depth--
			if depth == 0 {
				return Pos{line, col}, true
			}
		}
	}
	return Pos{}, false
}

// bracketKind возвращает открывающую и закрывающую скобки для r.
func bracketKind(r rune) (open, close rune, ok bool) {
	switch r {
	case '(':
		return '(', ')', true
	case '[':
		return '[', ']', true
	case '{':
		return '{', '}', true
	case ')':
		return '(', ')', true
	case ']':
		return '[', ']', true
	case '}':
		return '{', '}', true
	}
	return 0, 0, false
}

// escapedAt — руна в col экранирована нечётным числом "\" перед ней.
func escapedAt(line []rune, col int) bool {
	n := 0
	for i := col - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// BreakPair разбивает пустую пару на три строки:
//
//	{
//	<indent><step>
//	<indent>}
//
// Курсор встаёт на внутренний отступ. Не между парой — false. Одна транзакция.
func (e *Editor) BreakPair(open, close rune, indent, step string) bool {
	if e.Buf.ReadOnly || e.HasSelection() {
		return false
	}
	o, c, ok := bracketKind(open)
	if !ok || o != open || c != close {
		return false
	}
	cur := e.cursor()
	line := e.Buf.Lines[cur.Line]
	if cur.Col == 0 || cur.Col >= len(line) || line[cur.Col-1] != open || line[cur.Col] != close {
		return false
	}

	// Сначала закрывающая часть, потом внутренняя: курсор после второй вставки
	// оказывается на внутреннем отступе. Строки собираются в правильном порядке.
	e.BeginGroup()
	e.editRange(Range{Start: cur, End: cur}, "\n"+indent, false, false, false)
	e.editRange(Range{Start: cur, End: cur}, "\n"+indent+step, false, true, false)
	e.EndGroup()
	return true
}

// SmartHome — на первый непробельный символ строки; если курсор уже там — в 0.
func (e *Editor) SmartHome() {
	cur := e.cursor()
	col := leadingWhitespace(e.Buf.Lines[cur.Line])
	if cur.Col == col {
		col = 0
	}
	e.moveTo(Pos{cur.Line, col}, false)
}

// TrimTrailingWhitespace убирает пробелы и табы в конце непустых строк,
// не трогая отступ и строки из одних пробелов. Одна транзакция.
func (e *Editor) TrimTrailingWhitespace() bool {
	if e.Buf.ReadOnly {
		return false
	}
	changed := false
	e.BeginGroup()
	for line := 0; line <= e.Buf.LastLine(); line++ {
		text := e.Buf.Lines[line]
		end := len(text)
		for end > 0 && (text[end-1] == ' ' || text[end-1] == '\t') {
			end--
		}
		if end == len(text) || end == 0 {
			continue
		}
		if e.editRange(Range{Start: Pos{line, end}, End: Pos{line, len(text)}}, "", false, false, true) {
			changed = true
		}
	}
	e.EndGroup()
	return changed
}

// EnsureFinalNewline добавляет завершающий перевод строки, если последняя
// строка непустая. В модели Lines это новая пустая строка в конце.
func (e *Editor) EnsureFinalNewline() bool {
	if e.Buf.ReadOnly {
		return false
	}
	last := e.Buf.LastLine()
	if len(e.Buf.Lines[last]) == 0 {
		return false
	}
	p := Pos{last, len(e.Buf.Lines[last])}
	return e.editRange(Range{Start: p, End: p}, "\n", false, false, true)
}
