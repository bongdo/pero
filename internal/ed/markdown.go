package ed

import (
	"path/filepath"
	"strconv"
	"strings"
)

func isMarkdownPath(path string) bool {
	if path == "" { // новый безымянный буфер считаем markdown-совместимым
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown", ".mkd", ".mdx":
		return true
	}
	return false
}

// lineInFence — строка внутри ``` или ~~~ (сами ограждения тоже считаются кодом).
func (e *Editor) lineInFence(line int) bool {
	open := false
	var fence byte
	openLen := 0
	for i := 0; i <= line && i < len(e.Buf.Lines); i++ {
		trimmed := strings.TrimLeft(string(e.Buf.Lines[i]), " \t")
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			ch := trimmed[0]
			n := 0
			for n < len(trimmed) && trimmed[n] == ch {
				n++
			}
			if n >= 3 {
				if !open {
					open, fence, openLen = true, ch, n
				} else if fence == ch && n >= openLen {
					open = false
				}
				if i == line {
					return true
				}
				continue
			}
		}
		if i == line {
			return open
		}
	}
	return false
}

// ToggleCheckbox циклит чекбокс: нет → - [ ] → - [x] → - [ ].
func (e *Editor) ToggleCheckbox() bool {
	if e.Buf.ReadOnly || !isMarkdownPath(e.Buf.Path) {
		return false
	}
	cur := e.cursor()
	if e.lineInFence(cur.Line) {
		return false
	}
	line := e.Buf.Lines[cur.Line]
	indent := leadingWhitespace(line)
	rest := line[indent:]

	// Уже есть чекбокс (голый или после маркера списка).
	cb := -1
	switch {
	case len(rest) >= 3 && rest[0] == '[' && rest[2] == ']' &&
		(rest[1] == ' ' || rest[1] == 'x' || rest[1] == 'X'):
		if len(rest) == 3 || rest[3] == ' ' {
			cb = indent
		}
	case len(rest) >= 5 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') &&
		rest[1] == ' ' && rest[2] == '[' && rest[4] == ']' &&
		(rest[3] == ' ' || rest[3] == 'x' || rest[3] == 'X'):
		if len(rest) == 5 || rest[5] == ' ' {
			cb = indent + 2
		}
	}
	if cb >= 0 {
		ch := "x"
		if line[cb+1] != ' ' {
			ch = " "
		}
		return e.editRange(Range{Start: Pos{cur.Line, cb + 1}, End: Pos{cur.Line, cb + 2}}, ch, false, false, true)
	}

	// Список без чекбокса: вставляем после маркера.
	if n := listMarkerLen(rest); n > 0 {
		at := Pos{cur.Line, indent + n}
		return e.editRange(Range{Start: at, End: at}, "[ ] ", false, false, true)
	}

	// Не список: добавляем в начало после отступа.
	at := Pos{cur.Line, indent}
	return e.editRange(Range{Start: at, End: at}, "- [ ] ", false, false, true)
}

func leadingWhitespace(line []rune) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

// listMarkerLen — длина маркера списка ("- ", "1. ") в начале rest.
func listMarkerLen(rest []rune) int {
	if len(rest) >= 2 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && rest[1] == ' ' {
		return 2
	}
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(rest) && rest[i] == '.' && rest[i+1] == ' ' {
		return i + 2
	}
	return 0
}

// continueMarker возвращает длину полного префикса и маркер следующей строки.
func continueMarker(rest []rune) (int, string, bool) {
	if len(rest) >= 2 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && rest[1] == ' ' {
		marker := string(rest[0])
		if len(rest) >= 6 && rest[2] == '[' && (rest[3] == ' ' || rest[3] == 'x' || rest[3] == 'X') &&
			rest[4] == ']' && rest[5] == ' ' {
			return 6, marker + " [ ] ", true
		}
		return 2, marker + " ", true
	}
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(rest) && rest[i] == '.' && rest[i+1] == ' ' {
		n, err := strconv.Atoi(string(rest[:i]))
		if err != nil {
			return 0, "", false
		}
		return i + 2, strconv.Itoa(n+1) + ". ", true
	}
	return 0, "", false
}

// ContinueList — Enter в строке списка: сам вставляет перевод строки и маркер
// (или убирает маркер пустого пункта). Сигнатура без возврата — по контракту.
func (e *Editor) ContinueList() {
	if e.Buf.ReadOnly || !isMarkdownPath(e.Buf.Path) {
		return
	}
	cur := e.cursor()
	if e.lineInFence(cur.Line) {
		return
	}
	line := e.Buf.Lines[cur.Line]
	indent := leadingWhitespace(line)
	markerLen, next, ok := continueMarker(line[indent:])
	if !ok {
		return
	}

	empty := true
	for _, r := range line[indent+markerLen:] {
		if r != ' ' && r != '\t' {
			empty = false
			break
		}
	}
	if empty && cur.Col >= indent+markerLen {
		e.editRange(Range{Start: Pos{cur.Line, indent}, End: Pos{cur.Line, indent + markerLen}}, "", false, true, false)
		return
	}

	insert := "\n" + string(line[:indent]) + next
	e.editRange(Range{Start: cur, End: cur}, insert, false, true, false)
}

// CollapseSpaces схлопывает повторные пробелы, не трогая отступ и табы.
func (e *Editor) CollapseSpaces() bool {
	if e.Buf.ReadOnly {
		return false
	}
	start, end := e.selectedLines()
	changed := false
	e.BeginGroup()
	for line := start; line <= end; line++ {
		text := e.Buf.Lines[line]
		indent := leadingWhitespace(text)
		var runs [][2]int
		for i := indent; i < len(text); {
			if text[i] != ' ' {
				i++
				continue
			}
			j := i
			for j < len(text) && text[j] == ' ' {
				j++
			}
			if j-i > 1 {
				runs = append(runs, [2]int{i, j})
			}
			i = j
		}
		// Справа налево: индексы ещё не обработанных пробегов не сдвигаются.
		for k := len(runs) - 1; k >= 0; k-- {
			r := runs[k]
			if e.editRange(Range{Start: Pos{line, r[0]}, End: Pos{line, r[1]}}, " ", false, false, true) {
				changed = true
			}
		}
	}
	e.EndGroup()
	return changed
}

func (e *Editor) selectedLines() (int, int) {
	cur := e.cursor()
	if !e.HasSelection() {
		return cur.Line, cur.Line
	}
	r := e.SelectionRange()
	start, end := r.Start.Line, r.End.Line
	if end > start && r.End.Col == 0 {
		end--
	}
	return start, end
}

func indentUnit(line []rune, width int) string {
	for _, r := range line {
		switch r {
		case ' ':
			continue
		case '\t':
			return "\t"
		default:
			return strings.Repeat(" ", width)
		}
	}
	return strings.Repeat(" ", width)
}

// IndentSelection добавляет отступ всем строкам выделения (или текущей).
func (e *Editor) IndentSelection() bool {
	if e.Buf.ReadOnly {
		return false
	}
	width := e.TabWidth
	if width <= 0 {
		width = 4
	}
	start, end := e.selectedLines()
	changed := false
	e.BeginGroup()
	for line := start; line <= end; line++ {
		unit := indentUnit(e.Buf.Lines[line], width)
		if e.editRange(Range{Start: Pos{line, 0}, End: Pos{line, 0}}, unit, false, false, true) {
			changed = true
		}
	}
	e.EndGroup()
	return changed
}

// OutdentSelection убирает один уровень отступа (таб или до TabWidth пробелов).
func (e *Editor) OutdentSelection() bool {
	if e.Buf.ReadOnly {
		return false
	}
	width := e.TabWidth
	if width <= 0 {
		width = 4
	}
	start, end := e.selectedLines()
	changed := false
	e.BeginGroup()
	for line := start; line <= end; line++ {
		n := outdentCount(e.Buf.Lines[line], width)
		if n == 0 {
			continue
		}
		if e.editRange(Range{Start: Pos{line, 0}, End: Pos{line, n}}, "", false, false, true) {
			changed = true
		}
	}
	e.EndGroup()
	return changed
}

func outdentCount(line []rune, width int) int {
	if len(line) == 0 {
		return 0
	}
	if line[0] == '\t' {
		return 1
	}
	n := 0
	for n < len(line) && n < width && line[n] == ' ' {
		n++
	}
	return n
}
