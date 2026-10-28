package widgets

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Input - однострочное поле ввода. Cursor - индекс руны в Text, а не байт.
// Переводы строк и табы схлопываются в пробелы: поле всегда одной строкой.
type Input struct {
	Text        []rune
	Cursor      int
	Placeholder string
	SelAll      bool
}

// Set заменяет текст и ставит курсор в конец.
func (i *Input) Set(s string) {
	i.Text = []rune(inputText(s))
	i.Cursor = len(i.Text)
	i.SelAll = false
}

// Value возвращает текущий текст.
func (i *Input) Value() string {
	return string(i.Text)
}

// Insert вставляет текст в позицию курсора. Выделение all заменяется.
func (i *Input) Insert(s string) {
	s = inputText(s)
	if s == "" {
		return
	}
	if i.SelAll {
		i.Text = nil
		i.Cursor = 0
		i.SelAll = false
	}
	i.clampCursor()
	ins := []rune(s)
	out := make([]rune, 0, len(i.Text)+len(ins))
	out = append(out, i.Text[:i.Cursor]...)
	out = append(out, ins...)
	out = append(out, i.Text[i.Cursor:]...)
	i.Text = out
	i.Cursor += len(ins)
}

// Backspace удаляет руну слева от курсора; при SelAll очищает всё.
func (i *Input) Backspace() {
	if i.SelAll {
		i.Clear()
		return
	}
	i.clampCursor()
	if i.Cursor == 0 {
		return
	}
	i.Text = append(i.Text[:i.Cursor-1], i.Text[i.Cursor:]...)
	i.Cursor--
}

// Delete удаляет руну под курсором; при SelAll очищает всё.
func (i *Input) Delete() {
	if i.SelAll {
		i.Clear()
		return
	}
	i.clampCursor()
	if i.Cursor >= len(i.Text) {
		return
	}
	i.Text = append(i.Text[:i.Cursor], i.Text[i.Cursor+1:]...)
}

// MoveLeft сдвигает курсор влево; выделение all сворачивается в начало.
func (i *Input) MoveLeft() {
	if i.SelAll {
		i.SelAll = false
		i.Cursor = 0
		return
	}
	i.clampCursor()
	if i.Cursor > 0 {
		i.Cursor--
	}
}

// MoveRight сдвигает курсор вправо; выделение all сворачивается в конец.
func (i *Input) MoveRight() {
	if i.SelAll {
		i.SelAll = false
		i.Cursor = len(i.Text)
		return
	}
	i.clampCursor()
	if i.Cursor < len(i.Text) {
		i.Cursor++
	}
}

// Home ставит курсор в начало.
func (i *Input) Home() {
	i.SelAll = false
	i.Cursor = 0
}

// End ставит курсор в конец.
func (i *Input) End() {
	i.SelAll = false
	i.Cursor = len(i.Text)
}

// WordLeft переносит курсор на слово влево. Слово - непрерывный кусок без
// пробелов (пунктуация считается частью слова - удобно для путей и флагов).
func (i *Input) WordLeft() {
	i.SelAll = false
	i.clampCursor()
	n := i.Cursor
	for n > 0 && unicode.IsSpace(i.Text[n-1]) {
		n--
	}
	for n > 0 && !unicode.IsSpace(i.Text[n-1]) {
		n--
	}
	i.Cursor = n
}

// WordRight переносит курсор на слово вправо; пробелы после слова пропускаются.
func (i *Input) WordRight() {
	i.SelAll = false
	i.clampCursor()
	n := i.Cursor
	for n < len(i.Text) && !unicode.IsSpace(i.Text[n]) {
		n++
	}
	for n < len(i.Text) && unicode.IsSpace(i.Text[n]) {
		n++
	}
	i.Cursor = n
}

// DeleteWordLeft удаляет слово слева от курсора (Ctrl-W в строке ввода).
func (i *Input) DeleteWordLeft() {
	if i.SelAll {
		i.Clear()
		return
	}
	i.clampCursor()
	start := i.Cursor
	for start > 0 && unicode.IsSpace(i.Text[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(i.Text[start-1]) {
		start--
	}
	if start == i.Cursor {
		return
	}
	i.Text = append(i.Text[:start], i.Text[i.Cursor:]...)
	i.Cursor = start
}

// Clear очищает поле.
func (i *Input) Clear() {
	i.Text = nil
	i.Cursor = 0
	i.SelAll = false
}

// Render рисует ровно width ячеек с горизонтальной прокруткой, чтобы курсор
// оставался видимым. Терминальный курсор не рисуется.
func (i *Input) Render(width int, style Style) string {
	if width <= 0 {
		return ""
	}

	if len(i.Text) == 0 {
		line := ""
		if i.Placeholder != "" {
			line = ansi.Truncate(oneLine(i.Placeholder), width, "…")
		}
		out := paint(style.BG+style.Muted, line)
		return out + paint(style.BG, strings.Repeat(" ", width-ansi.StringWidth(line)))
	}

	i.clampCursor()
	cursorX := ansi.StringWidth(string(i.Text[:i.Cursor]))
	offset := 0
	if cursorX >= width {
		offset = cursorX - width + 1
	}

	visible := ansi.Cut(string(i.Text), offset, offset+width)
	// Cut может вернуть широкий кластер, начавшийся до offset, - подрезаем.
	visible = ansi.Truncate(visible, width, "")
	prefix := style.BG + style.Text
	fill := style.BG
	if i.SelAll {
		prefix = style.BG + style.SelectedBG + style.SelectedFG
		fill = style.BG + style.SelectedBG
	}

	out := paint(prefix, visible)
	if n := width - ansi.StringWidth(visible); n > 0 {
		out += paint(fill, strings.Repeat(" ", n))
	}
	return out
}

func (i *Input) clampCursor() {
	i.Cursor = min(max(i.Cursor, 0), len(i.Text))
}

// inputText делает текст одной строкой и убирает управляющие символы:
// вставка из буфера не должна пускать ESC-последовательности в терминал.
func inputText(s string) string {
	s = oneLine(s)
	if !strings.ContainsFunc(s, unicode.IsControl) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
