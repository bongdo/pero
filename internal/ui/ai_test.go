package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

func aiResult(m *Model, text string, replace bool) {
	t := m.tab()
	pos := t.ed.Cursor
	rng := ed.Range{Start: pos, End: pos}
	if t.ed.HasSelection() {
		rng = t.ed.SelectionRange()
	}
	m.Update(aiMsg{
		tab: t, gen: m.aiGen, version: t.ed.Buf.Version,
		pos: pos, rng: rng, replace: replace, text: text,
	})
}

func TestAICommandNeedsText(t *testing.T) {
	m := testModel(t)
	if cmd := m.aiSuggest(); cmd != nil {
		t.Fatal("AI стартовал на пустой строке")
	}
	if m.toast.Kind != "info" {
		t.Fatalf("тост: %+v", m.toast)
	}
	// В палитре команда видна всегда.
	seen := false
	for _, c := range m.commandList() {
		if c.id == "ai.suggest" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("ai.suggest нет в палитре")
	}
}

func TestAltAStartsProgressAndEscCancels(t *testing.T) {
	m := testModel(t)
	typeText(m, "текст")
	_, cmd := m.Update(keyMsg('a', "a", tea.ModAlt))
	if cmd == nil {
		t.Fatal("Alt-a не запустил AI")
	}
	if m.toast.Kind != "progress" || !strings.Contains(m.toast.Text, "AI") {
		t.Fatalf("нет прогресса: %+v", m.toast)
	}
	if m.aiCancel == nil {
		t.Fatal("нет контекста отмены")
	}
	before := text(m)
	pressEsc(m)
	if m.aiCancel != nil {
		t.Fatal("Esc не отменил запрос")
	}
	if text(m) != before {
		t.Fatal("Esc изменил буфер")
	}
}

func TestAIResultReplacesSelectionOneUndo(t *testing.T) {
	m := testModel(t)
	typeText(m, "старый текст")
	m.tab().ed.SelectAll()
	m.aiSuggest()
	aiResult(m, "новый", true)

	if m.ov.kind != overlayAI || m.ov.aiText != "новый" {
		t.Fatalf("попап: kind=%v text=%q", m.ov.kind, m.ov.aiText)
	}
	if text(m) != "старый текст" {
		t.Fatal("результат попал в буфер до Tab")
	}
	m.Update(keyMsg('\t', "", 0))
	if got := text(m); got != "новый" {
		t.Fatalf("вставка: %q", got)
	}
	if m.ov.kind != overlayNone {
		t.Fatal("попап не закрылся")
	}
	pressCtrl(m, 'z')
	if got := text(m); got != "старый текст" {
		t.Fatalf("undo: %q", got)
	}
}

func TestAIResultInsertsAtCursor(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	aiResult(m, " + ai", false)
	m.Update(keyMsg('\t', "", 0))
	if got := text(m); got != "abc + ai" {
		t.Fatalf("вставка в курсор: %q", got)
	}
}

func TestAIEscClosesPopup(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	aiResult(m, "результат", false)
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("Esc не закрыл попап")
	}
	if got := text(m); got != "abc" {
		t.Fatalf("буфер изменился: %q", got)
	}
}

func TestAIErrorKeepsBuffer(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	gen := m.aiGen
	m.Update(aiMsg{
		tab: m.tab(), gen: gen, version: m.tab().ed.Buf.Version,
		err: errors.New("opencode не найден в PATH"),
	})
	if got := text(m); got != "abc" {
		t.Fatalf("ошибка изменила буфер: %q", got)
	}
	if m.toast.Kind != "error" {
		t.Fatalf("нет error-тоста: %+v", m.toast)
	}
	if m.ov.kind != overlayNone {
		t.Fatal("ошибка открыла попап")
	}
}

func TestAIStaleResultIgnoredAfterCancel(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	gen := m.aiGen
	pressEsc(m)
	m.Update(aiMsg{tab: m.tab(), gen: gen, version: m.tab().ed.Buf.Version, text: "x"})
	if m.ov.kind == overlayAI {
		t.Fatal("отменённый результат открыл попап")
	}
}

func TestAIPopupRendersAtSizes(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	long := strings.Repeat("длинная строка результата ", 30)
	aiResult(m, long+"\nвторая строка", false)
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}} {
		setSize(m, sz.w, sz.h)
		lines := viewLines(t, m)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "AI") {
			t.Fatalf("%dx%d: нет заголовка AI", sz.w, sz.h)
		}
	}
}

func TestAIClickOutsideCloses(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.aiSuggest()
	aiResult(m, "результат", false)
	m.View()
	m.mouseClick(tea.Mouse{Button: tea.MouseLeft, X: 0, Y: 0})
	if m.ov.kind != overlayNone {
		t.Fatal("клик вне попапа не закрыл AI")
	}
	if got := text(m); got != "abc" {
		t.Fatalf("клик изменил буфер: %q", got)
	}
}
