package widgets

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Overlay вставляет box в base начиная с (x, y). width - ширина экрана;
// строки base дополняются до неё. Если box не помещается справа или снизу,
// позиция прижимается внутрь. Строки вне base отбрасываются, чужие ANSI не
// разрезаются. Если width <= 0, ширина берётся из base.
func Overlay(base []string, box []string, x, y, width int) []string {
	if len(base) == 0 || len(box) == 0 {
		return base
	}

	if width <= 0 {
		for _, line := range base {
			width = max(width, ansi.StringWidth(line))
		}
		if width <= 0 {
			return base
		}
	}

	boxW := 0
	for _, line := range box {
		boxW = max(boxW, ansi.StringWidth(line))
	}
	if boxW <= 0 {
		return base
	}
	boxW = min(boxW, width)

	x = max(0, x)
	if x+boxW > width {
		x = max(0, width-boxW)
	}

	y = max(0, y)
	if y+len(box) > len(base) {
		y = max(0, len(base)-len(box))
	}

	out := make([]string, len(base))
	for i, line := range base {
		// Не перекрытые попапом строки тоже должны быть шириной экрана.
		if w := ansi.StringWidth(line); w < width {
			line += strings.Repeat(" ", width-w)
		}
		out[i] = line
	}
	for i, boxLine := range box {
		row := y + i
		if row < 0 || row >= len(out) {
			continue
		}

		line := out[row]
		if w := ansi.StringWidth(line); w < width {
			line += strings.Repeat(" ", width-w)
		}

		left := ansi.Cut(line, 0, x)
		if w := ansi.StringWidth(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		// Отрезанная часть могла оставить открытый SGR - закрываем, чтобы
		// цвет подложки не перекрасил попап.
		if strings.Contains(left, "\x1b") {
			left += sgrReset
		}

		cell := padRight(ansi.Truncate(boxLine, boxW, ""), boxW)
		right := ""
		if n := width - x - boxW; n > 0 {
			// Cut может вернуть широкий кластер, начавшийся до границы, -
			// подрезаем, чтобы строка не стала шире экрана.
			right = ansi.Truncate(ansi.Cut(line, x+boxW, width), n, "")
		}
		// shortcut: правый остаток base теряет SGR-префикс, начавшийся до x;
		// отслеживать активный SGR, если понадобится сохранять цвет фона.
		out[row] = padRight(left+cell+right, width)
	}
	return out
}
