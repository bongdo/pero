package ui

import (
	"fmt"
	"time"
)

// tr возвращает русскую строку для language = "ru", иначе английскую.
func (m *Model) tr(ru, en string) string {
	if m.cfg.Language == "ru" {
		return ru
	}
	return en
}

func (m *Model) trf(formatRu, formatEn string, args ...any) string {
	return fmt.Sprintf(m.tr(formatRu, formatEn), args...)
}

// count - число с существительным: в русском три формы, в английском две.
func (m *Model) count(n int, ruOne, ruFew, ruMany, enOne, enMany string) string {
	if m.cfg.Language == "ru" {
		return itoa(n) + " " + pluralRu(n, ruOne, ruFew, ruMany)
	}
	if n == 1 {
		return itoa(n) + " " + enOne
	}
	return itoa(n) + " " + enMany
}

func (m *Model) noCompletionsToast() {
	text := m.tr("нет подсказок", "no completions")
	// Один и тот же тост не повторяем, пока он ещё виден.
	if m.toast.Text == text && (m.toast.Until.IsZero() || time.Now().Before(m.toast.Until)) {
		return
	}
	m.notifyInfo(text)
}
