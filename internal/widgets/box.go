package widgets

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const sgrReset = "\x1b[0m"

// Style — ANSI SGR-префиксы для виджетов. Пустое поле означает «без стиля».
// Префиксы просто конкатенируются, поэтому BG и Text обычно задают вместе.
type Style struct {
	BG, Border, Text, Muted, SelectedBG, SelectedFG, Error, Warning string
}

// Box — содержимое попапа. Width — доступная ширина (например, экран):
// внутренняя ширина не превышает Width-4. Width <= 0 — без ограничения.
type Box struct {
	Title, Footer string
	Rows          []string
	Width         int
}

// RenderBox рисует рамку ╭─╮│╰╯ с заголовком слева и футером в нижней рамке.
// Каждая строка результата имеет одинаковую ширину. Содержимое обрезается по
// ANSI-ширине, а не по рунам.
func RenderBox(b Box, style Style) []string {
	contentW := 0
	for _, row := range b.Rows {
		contentW = max(contentW, ansi.StringWidth(oneLine(row)))
	}

	inner := max(24, contentW)
	if b.Width > 0 {
		inner = min(inner, b.Width-4)
	}
	inner = max(0, inner)

	border := style.BG + style.Border
	body := style.BG + style.Text

	lines := make([]string, 0, len(b.Rows)+2)
	lines = append(lines, topBorder(b.Title, inner, border, body))
	for _, row := range b.Rows {
		cell := padRight(ansi.Truncate(oneLine(row), inner, ""), inner)
		lines = append(lines,
			paint(border, "│")+paint(body, cell)+paint(border, "│"))
	}
	lines = append(lines, bottomBorder(b.Footer, inner, border, style.BG+style.Muted))
	return lines
}

func topBorder(title string, inner int, border, label string) string {
	mid := ""
	if inner > 0 && title != "" {
		t := ansi.Truncate(oneLine(title), max(0, inner-4), "…")
		if t != "" {
			head := paint(border, "─ ") + paint(label, t) + paint(border, " ")
			if d := inner - ansi.StringWidth(head); d > 0 {
				head += paint(border, strings.Repeat("─", d))
			}
			mid = head
		}
	}
	if mid == "" {
		mid = paint(border, strings.Repeat("─", inner))
	}
	return paint(border, "╭") + mid + paint(border, "╮")
}

func bottomBorder(footer string, inner int, border, muted string) string {
	mid := ""
	if inner > 0 && footer != "" {
		f := ansi.Truncate(oneLine(footer), max(0, inner-3), "…")
		if f != "" {
			part := paint(muted, " "+f+" ")
			if d := inner - ansi.StringWidth(part); d > 0 {
				part += paint(border, strings.Repeat("─", d))
			}
			mid = part
		}
	}
	if mid == "" {
		mid = paint(border, strings.Repeat("─", inner))
	}
	return paint(border, "╰") + mid + paint(border, "╯")
}

// paint оборачивает текст SGR-префиксом и сбросом. Пустой префикс — без стиля.
func paint(prefix, s string) string {
	if prefix == "" || s == "" {
		return s
	}
	return prefix + s + sgrReset
}

// padRight дополняет строку пробелами до заданной ANSI-ширины.
func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if w := ansi.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// oneLine превращает возможные переводы строк и табы в пробелы: виджеты
// рисуют одну строку на элемент.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\t", " ")
}
