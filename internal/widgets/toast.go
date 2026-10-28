package widgets

import (
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Toast — одно уведомление. Kind: success, info, warning, error, progress.
// Until — момент исчезновения; нулевое значение означает «не истекает».
type Toast struct {
	Text  string
	Kind  string
	Until time.Time
}

// RenderToast рисует одну строку с иконкой и цветом по типу. Ширина не больше
// min(40, width-2); длинный текст обрезается. Истёкший тост — пустая строка.
func RenderToast(t Toast, width int, style Style) string {
	if !t.Until.IsZero() && !t.Until.After(time.Now()) {
		return ""
	}

	limit := min(40, width-2)
	if limit < 1 {
		return ""
	}

	icon, color := "i", style.Text
	switch strings.ToLower(strings.TrimSpace(t.Kind)) {
	case "success":
		icon = "✓"
	case "warning":
		icon = "!"
		if style.Warning != "" {
			color = style.Warning
		} else {
			color = style.Error
		}
	case "error":
		icon, color = "×", style.Error
	case "progress":
		icon, color = "…", style.Muted
	}

	line := icon
	if text := strings.TrimSpace(oneLine(t.Text)); text != "" {
		line += " " + text
	}
	line = ansi.Truncate(line, limit, "…")
	return paint(style.BG+color, line)
}
