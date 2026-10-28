package widgets

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// List - вертикальный список фиксированной видимой высоты.
//
// Move/Page/Home/End прижимают выбор к границам (clamp), циклического перехода
// нет: с последней строки Move(+1) остаётся на месте.
type List struct {
	Items         []string
	Sel, Scroll   int
	Height, Width int
}

// Move сдвигает выбор на delta строк и держит его в видимой части.
func (l *List) Move(delta int) {
	if len(l.Items) == 0 {
		l.Sel, l.Scroll = 0, 0
		return
	}
	l.Sel = min(max(l.Sel+delta, 0), len(l.Items)-1)
	l.fix()
}

// Page листает на delta страниц (страница = Height строк, минимум 1).
func (l *List) Page(delta int) {
	step := max(1, l.Height)
	l.Move(delta * step)
}

// Home выбирает первую строку.
func (l *List) Home() {
	l.Sel, l.Scroll = 0, 0
}

// End выбирает последнюю строку.
func (l *List) End() {
	if len(l.Items) == 0 {
		l.Sel, l.Scroll = 0, 0
		return
	}
	l.Sel = len(l.Items) - 1
	l.fix()
}

// Visible возвращает ровно Height строк: недостающие - пустые. Выбранная
// строка всегда попадает в окно, Scroll подправляется при необходимости.
func (l *List) Visible() []string {
	if l.Height <= 0 {
		return nil
	}
	l.fix()

	rows := make([]string, l.Height)
	for i := range rows {
		if idx := l.Scroll + i; idx >= 0 && idx < len(l.Items) {
			rows[i] = l.Items[idx]
		}
	}
	return rows
}

// Render рисует Visible в width колонок: обрезает по ANSI-ширине, дополняет
// пробелами и подсвечивает выбранную строку. width <= 0 - берётся l.Width.
func (l *List) Render(width int, style Style) string {
	if width <= 0 {
		width = l.Width
	}
	if width <= 0 {
		return ""
	}

	rows := l.Visible()
	if len(rows) == 0 {
		return ""
	}

	selected := l.Sel - l.Scroll
	lines := make([]string, len(rows))
	for idx, item := range rows {
		text := ansi.Truncate(oneLine(item), width, "…")

		prefix, fill := style.BG+style.Text, style.BG
		if idx == selected && idx < len(l.Items) {
			prefix = style.BG + style.SelectedBG + style.SelectedFG
			fill = style.BG + style.SelectedBG
		}

		line := paint(prefix, text)
		if n := width - ansi.StringWidth(text); n > 0 {
			line += paint(fill, strings.Repeat(" ", n))
		}
		lines[idx] = line
	}
	return strings.Join(lines, "\n")
}

// fix прижимает Sel/Scroll к допустимым значениям и прокручивает окно так,
// чтобы выбранная строка была видна.
func (l *List) fix() {
	if len(l.Items) == 0 {
		l.Sel, l.Scroll = 0, 0
		return
	}
	l.Sel = min(max(l.Sel, 0), len(l.Items)-1)

	if l.Height <= 0 {
		l.Scroll = min(max(l.Scroll, 0), len(l.Items)-1)
		return
	}

	maxScroll := max(0, len(l.Items)-l.Height)
	l.Scroll = min(max(l.Scroll, 0), maxScroll)
	if l.Sel < l.Scroll {
		l.Scroll = l.Sel
	}
	if l.Sel >= l.Scroll+l.Height {
		l.Scroll = l.Sel - l.Height + 1
	}
	l.Scroll = min(max(l.Scroll, 0), maxScroll)
}
