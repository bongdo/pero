package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

func bufWith(t *testing.T, text string) *ed.Buffer {
	t.Helper()
	return ed.OpenBytes([]byte(text), "test.md")
}

func TestLSPRangeConversionsUnicode(t *testing.T) {
	b := bufWith(t, "привет 😀 мир\nвторая строка")
	// Перед «м»: «привет » = 7 UTF-16 units, эмодзи = 2, пробел = 1.
	if got := b.UTF16Offset(ed.Pos{Line: 0, Col: 9}); got != 10 {
		t.Fatalf("UTF16Offset(м)=%d, want 10", got)
	}
	r := lspRangeToEd(b, lsp.Range{
		Start: lsp.Position{Line: 0, Character: 10},
		End:   lsp.Position{Line: 0, Character: 13},
	})
	if r.Start != (ed.Pos{Line: 0, Col: 9}) || r.End != (ed.Pos{Line: 0, Col: 12}) {
		t.Fatalf("range из UTF-16: %+v", r)
	}
	back := edRangeToLSP(b, r)
	if back.Start.Character != 10 || back.End.Character != 13 {
		t.Fatalf("обратная конвертация: %+v", back)
	}
	// Позиция внутри суррогатной пары прижимается к началу эмодзи.
	if p := b.PosFromUTF16(0, 8); p != (ed.Pos{Line: 0, Col: 7}) {
		t.Fatalf("суррогатная пара: %+v", p)
	}
	// Кириллица во второй строке.
	r2 := lspRangeToEd(b, lsp.Range{
		Start: lsp.Position{Line: 1, Character: 0},
		End:   lsp.Position{Line: 1, Character: 6},
	})
	if r2.End != (ed.Pos{Line: 1, Col: 6}) {
		t.Fatalf("кириллица: %+v", r2)
	}
}

func TestApplyTextEditsReverseOrder(t *testing.T) {
	b := bufWith(t, "abc\ndef\nghi")
	e := ed.NewEditor(b)
	edits := []lsp.TextEdit{
		{Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 0}}, NewText: "1"},
		{Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 3}, End: lsp.Position{Line: 0, Character: 3}}, NewText: "2"},
		{Range: lsp.Range{Start: lsp.Position{Line: 1, Character: 0}, End: lsp.Position{Line: 1, Character: 3}}, NewText: "XYZ"},
		{Range: lsp.Range{Start: lsp.Position{Line: 2, Character: 3}, End: lsp.Position{Line: 2, Character: 3}}, NewText: "!"},
	}
	if n := applyTextEdits(e, edits); n != 4 {
		t.Fatalf("применено %d", n)
	}
	if got := b.Text(); got != "1abc2\nXYZ\nghi!" {
		t.Fatalf("текст %q", got)
	}
}

func TestFilterCompletion(t *testing.T) {
	items := []lsp.CompletionItem{
		{Label: "Update"}, {Label: "UpdateMsg"}, {Label: "printf"},
	}
	if got := filterCompletions(items, ""); len(got) != 3 {
		t.Fatalf("пустой фильтр: %d", len(got))
	}
	if got := filterCompletions(items, "up"); len(got) != 2 {
		t.Fatalf("префикс up: %+v", got)
	}
	if got := filterCompletions(items, "Msg"); len(got) != 1 || got[0].Label != "UpdateMsg" {
		t.Fatalf("подстрока: %+v", got)
	}
	if got := filterCompletions(items, "zzz"); len(got) != 0 {
		t.Fatalf("мусор: %+v", got)
	}
}

func TestWordBoundsMidWordAndHyphen(t *testing.T) {
	line := []rune("print printer")
	if s, e := wordBounds(line, 5); s != 0 || e != 5 {
		t.Fatalf("конец слова: %d..%d", s, e)
	}
	if s, e := wordBounds(line, 3); s != 0 || e != 5 {
		t.Fatalf("середина слова: %d..%d", s, e)
	}
	hy := []rune("foo-bar baz")
	if s, e := wordBounds(hy, 4); s != 0 || e != 7 {
		t.Fatalf("дефис: %d..%d", s, e)
	}
}

func TestAcceptCompletionReplacesPrefixOneUndo(t *testing.T) {
	m := testModel(t)
	typeText(m, "pri")
	tab := m.tab()
	tab.comp = completionState{
		open: true, start: ed.Pos{Line: 0, Col: 0}, end: ed.Pos{Line: 0, Col: 3},
		items: []lsp.CompletionItem{{Label: "print", InsertText: "print"}},
	}
	m.acceptCompletion(tab, tab.comp.items[0])
	if got := text(m); got != "print" {
		t.Fatalf("после принятия: %q", got)
	}
	m.runCommand("edit.undo")
	if got := text(m); got != "pri" {
		t.Fatalf("undo: %q", got)
	}
}

func TestAcceptCompletionMidWord(t *testing.T) {
	m := testModel(t)
	typeText(m, "printer")
	tab := m.tab()
	tab.ed.SetCursor(ed.Pos{Line: 0, Col: 5})
	start, end := wordBounds(tab.ed.Buf.Line(0), 5)
	tab.comp = completionState{
		open: true, start: ed.Pos{Line: 0, Col: start}, end: ed.Pos{Line: 0, Col: end},
		items: []lsp.CompletionItem{{Label: "printf", InsertText: "printf"}},
	}
	m.acceptCompletion(tab, tab.comp.items[0])
	if got := text(m); got != "printf" {
		t.Fatalf("середина слова: %q", got)
	}
}

func TestAcceptCompletionTextEditUnicode(t *testing.T) {
	m := testModel(t)
	typeText(m, "привет мир")
	tab := m.tab()
	b := tab.ed.Buf
	item := lsp.CompletionItem{
		Label:      "мир",
		InsertText: "свет",
		TextEdit: &lsp.TextEdit{
			Range: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 7},
				End:   lsp.Position{Line: 0, Character: 10},
			},
			NewText: "свет",
		},
	}
	tab.comp = completionState{open: true, start: ed.Pos{Line: 0, Col: 7}, end: ed.Pos{Line: 0, Col: 10}}
	if got := b.UTF16Offset(ed.Pos{Line: 0, Col: 7}); got != 7 {
		t.Fatalf("подготовка: %d", got)
	}
	m.acceptCompletion(tab, item)
	if got := text(m); got != "привет свет" {
		t.Fatalf("TextEdit: %q", got)
	}
}

func TestJumpDiagnosticsCycleAndScroll(t *testing.T) {
	m := testModel(t)
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "строка " + itoa(i)
	}
	typeText(m, strings.Join(lines, "\n"))
	setSize(m, 40, 10)
	m.tab().diags = []lsp.Diagnostic{
		{Range: lsp.Range{Start: lsp.Position{Line: 0}, End: lsp.Position{Line: 0, Character: 3}}, Severity: 2, Message: "первая"},
		{Range: lsp.Range{Start: lsp.Position{Line: 20}, End: lsp.Position{Line: 20, Character: 3}}, Severity: 1, Message: "двадцатая"},
	}
	m.tab().diagVersion = m.tab().ed.Buf.Version
	m.tab().ed.SetCursor(ed.Pos{})
	m.jumpDiagnostic(1)
	if m.tab().ed.Cursor.Line != 20 {
		t.Fatalf("F8: %+v", m.tab().ed.Cursor)
	}
	if m.tab().top == 0 {
		t.Fatal("F8 не проскроллил к диагностике")
	}
	m.jumpDiagnostic(1)
	if m.tab().ed.Cursor.Line != 0 {
		t.Fatalf("F8 по кругу: %+v", m.tab().ed.Cursor)
	}
	m.jumpDiagnostic(-1)
	if m.tab().ed.Cursor.Line != 20 {
		t.Fatalf("Shift-F8 по кругу: %+v", m.tab().ed.Cursor)
	}
}

func TestQuickfixNoFixesToast(t *testing.T) {
	m := testModel(t)
	typeText(m, "текст")
	tab := m.tab()
	tab.lspState = lspReady
	m.fixGen = 7
	m.Update(lspActionsMsg{tab: tab, gen: 7, version: tab.ed.Buf.Version, line: 0, actions: nil})
	if !strings.Contains(m.toast.Text, "нет фиксов") {
		t.Fatalf("тост: %+v", m.toast)
	}
	if got := text(m); got != "текст" {
		t.Fatalf("буфер изменён: %q", got)
	}
}

func TestHoverMessagesNoPanic(t *testing.T) {
	m := testModel(t)
	typeText(m, "слово")
	tab := m.tab()
	m.hoverGen = 3
	m.Update(lspHoverMsg{tab: tab, gen: 3, version: tab.ed.Buf.Version, hover: nil})
	if !strings.Contains(m.toast.Text, "нет подсказки") {
		t.Fatalf("пустой hover: %+v", m.toast)
	}
	m.Update(lspHoverMsg{tab: tab, gen: 3, version: tab.ed.Buf.Version, hover: &lsp.Hover{Contents: "**подсказка**\n\n```go\nfunc f()\n```"}})
	if m.ov.kind != overlayHover {
		t.Fatalf("hover не открылся: %v", m.ov.kind)
	}
	setSize(m, 80, 24)
	viewLines(t, m)
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("Esc не закрыл hover")
	}
	// Ошибка не должна ронять редактор.
	m.Update(lspHoverMsg{tab: tab, gen: 3, version: tab.ed.Buf.Version, err: os.ErrDeadlineExceeded})
}

func TestLocationsMessagesNoPanic(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	tab := m.tab()
	m.locsGen = 2
	m.Update(lspLocsMsg{tab: tab, gen: 2, version: tab.ed.Buf.Version, direct: true, title: "Определение", locations: []lsp.Location{{URI: "untitled:1"}}})
	if m.toast.Kind != "error" {
		t.Fatalf("плохой URI: %+v", m.toast)
	}
	m.Update(lspLocsMsg{tab: tab, gen: 2, version: tab.ed.Buf.Version, direct: false, title: "Ссылки", err: os.ErrDeadlineExceeded})
	if m.toast.Kind != "error" {
		t.Fatalf("ошибка ссылок: %+v", m.toast)
	}
}

func TestCompletionPopupSmallLayout(t *testing.T) {
	m := testModel(t)
	setSize(m, 40, 10)
	typeText(m, "up")
	tab := m.tab()
	m.showCompletion(tab, 0, []lsp.CompletionItem{
		{Label: "update", Kind: "func", Detail: "(m) error"},
		{Label: "UpdateMsg", Kind: "type"},
		{Label: "updateView", Kind: "func", Detail: "() string"},
	}, "up", 0, 2)
	if !tab.comp.open || m.ov.kind != overlayCompletion {
		t.Fatalf("popup не открылся: %+v", tab.comp)
	}
	viewLines(t, m)
	// Фильтр по вводу.
	m.Update(keyMsg('m', "m", 0))
	if len(tab.comp.items) != 1 || tab.comp.items[0].Label != "UpdateMsg" {
		t.Fatalf("фильтр: %+v", tab.comp.items)
	}
	viewLines(t, m)
	pressEnter(m)
	if got := text(m); got != "UpdateMsg" {
		t.Fatalf("принятие: %q", got)
	}
}

func TestCompletionSingleCandidateNoPopup(t *testing.T) {
	m := testModel(t)
	typeText(m, "upd")
	tab := m.tab()
	m.showCompletion(tab, 0, []lsp.CompletionItem{{Label: "update", InsertText: "update"}}, "upd", 0, 3)
	if m.ov.kind == overlayCompletion {
		t.Fatal("один кандидат не должен открывать popup")
	}
	if got := text(m); got != "update" {
		t.Fatalf("подстановка: %q", got)
	}
}

func TestCompletionZeroCandidatesShowsToast(t *testing.T) {
	m := testModel(t)
	typeText(m, "zz")
	tab := m.tab()
	m.showCompletion(tab, 0, nil, "zz", 0, 2)
	if m.ov.kind == overlayCompletion {
		t.Fatal("пустой список открыл popup")
	}
	if !strings.Contains(m.toast.Text, "нет подсказок") {
		t.Fatalf("нет info-тоста: %+v", m.toast)
	}
}

func TestWorkspaceEditOpensOtherFile(t *testing.T) {
	m, first := testModelFile(t, "a.md", "один")
	dir := filepath.Dir(first)
	second := filepath.Join(dir, "b.md")
	writeFile(t, second, "два")
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		lsp.PathToURI(first): {{
			Range:   lsp.Range{Start: lsp.Position{}, End: lsp.Position{Line: 0, Character: 4}},
			NewText: "1",
		}},
		lsp.PathToURI(second): {{
			Range:   lsp.Range{Start: lsp.Position{}, End: lsp.Position{Line: 0, Character: 3}},
			NewText: "2",
		}},
	}}
	m.applyWorkspaceEdit(edit, "тест")
	if got := m.tabForPath(first).ed.Buf.Text(); got != "1" {
		t.Fatalf("первый файл: %q", got)
	}
	bt := m.tabForPath(second)
	if bt == nil || bt.ed.Buf.Text() != "2" {
		t.Fatalf("второй файл: %+v", bt)
	}
	if data, _ := os.ReadFile(first); string(data) != "один" {
		t.Fatalf("файл на диске изменён без сохранения: %q", data)
	}
	if !m.tabForPath(first).ed.Buf.Dirty() {
		t.Fatal("буфер не помечен грязным")
	}
}

func TestLSPCommandForProse(t *testing.T) {
	m, _ := testModelFile(t, "ru.md", "привет мир, это текст")
	tb := m.tab()
	cmd, lang := m.lspCommandFor(tb)
	if cmd != "ltex-ls" || lang != "markdown" {
		t.Fatalf("кириллица: %q %q", cmd, lang)
	}
	writeFile(t, tb.ed.Buf.Path, "hello world, plain text")
	tb.ed.Buf.Reload()
	cmd, _ = m.lspCommandFor(tb)
	if cmd != "harper-ls" {
		t.Fatalf("английский: %q", cmd)
	}
	m.cfg.Markdown.Spell = "off"
	if cmd, _ := m.lspCommandFor(tb); cmd != "" {
		t.Fatalf("off: %q", cmd)
	}
}

func TestLSPEventFiltersClientAndURI(t *testing.T) {
	m := testModel(t)
	typeText(m, "текст")
	tab := m.tab()
	client := lsp.NewClient(lsp.ServerConfig{}, "")
	tab.lsp = client
	tab.lspURI = "file:///a"
	tab.lspState = lspReady
	diag := lsp.Diagnostic{
		Range:    lsp.Range{Start: lsp.Position{}, End: lsp.Position{Line: 0, Character: 3}},
		Severity: 1,
		Message:  "ошибка",
	}
	m.Update(lspEvent{client: client, uri: "file:///b", diags: []lsp.Diagnostic{diag}})
	if len(tab.diags) != 0 {
		t.Fatal("диагностика чужого URI попала во вкладку")
	}
	m.Update(lspEvent{client: client, uri: "file:///a", diags: []lsp.Diagnostic{diag}})
	if len(tab.diags) != 1 || tab.diags[0].Message != "ошибка" {
		t.Fatalf("своя диагностика не применена: %+v", tab.diags)
	}
	// Событие старого клиента после закрытия/замены не применяется.
	other := lsp.NewClient(lsp.ServerConfig{}, "")
	tab.lsp = other
	m.Update(lspEvent{client: client, uri: "file:///a", diags: nil})
	if len(tab.diags) != 1 {
		t.Fatal("событие старого клиента перетёрло диагностики")
	}
	m.detachLSP(tab)
	if tab.lsp != nil || len(tab.diags) != 0 || tab.lspState != lspNone {
		t.Fatalf("detach не сбросил LSP: %+v", tab)
	}
}

func TestLSPStatusText(t *testing.T) {
	m, _ := testModelFile(t, "x.go", "package x\n")
	tb := m.tab()
	tb.lspState = lspStarting
	if got := m.lspStatusText(tb); got != "LSP…" {
		t.Fatalf("старт: %q", got)
	}
	tb.lspState = lspReady
	tb.lspName = "gopls"
	if got := m.lspStatusText(tb); got != "gopls" {
		t.Fatalf("готов: %q", got)
	}
	tb.lspState = lspOff
	if got := m.lspStatusText(tb); got != "LSP off" {
		t.Fatalf("off: %q", got)
	}
}

func TestQuitDoesNotRestartLSP(t *testing.T) {
	m, _ := testModelFile(t, "x.md", "hello\n")
	m.lspAuto = true
	tb := m.tab()
	tb.lspState = lspReady
	tb.lspName = "harper-ls"
	cmd := m.quitCmd()
	if cmd == nil || !m.quitting {
		t.Fatal("quit не выставил quitting")
	}
	if got := m.ensureActiveLSP(); got != nil {
		t.Fatal("после quit снова стартует LSP")
	}
	if tb.lsp != nil || tb.lspState != lspNone {
		t.Fatalf("LSP перезапущен при выходе: state=%v client=%v", tb.lspState, tb.lsp)
	}
}

func TestHoverUnsupportedDoesNotRequest(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	tab := m.tab()
	tab.lsp = &lsp.Client{}
	tab.lspState = lspReady
	tab.lspName = "fake"
	if cmd := m.hoverCmd(); cmd != nil {
		t.Fatal("запрос ушёл серверу без hover")
	}
	if !strings.Contains(m.toast.Text, "не поддерживает подсказки") {
		t.Fatalf("тост: %+v", m.toast)
	}
}

func TestFormatUnsupportedDoesNotRequest(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	tab := m.tab()
	tab.lsp = &lsp.Client{}
	tab.lspState = lspReady
	if cmd := m.formatCmd(); cmd != nil {
		t.Fatal("запрос ушёл серверу без форматирования")
	}
	if !strings.Contains(m.toast.Text, "не поддерживает форматирование") {
		t.Fatalf("тост: %+v", m.toast)
	}
}

func TestInternalErrorIsFriendly(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	tab := m.tab()
	m.hoverGen = 9
	m.Update(lspHoverMsg{
		tab: tab, gen: 9, version: tab.ed.Buf.Version,
		err: errors.New("-32603 Internal error"),
	})
	if strings.Contains(strings.ToLower(m.toast.Text), "internal error") {
		t.Fatalf("сырая ошибка: %+v", m.toast)
	}
	if !strings.Contains(m.toast.Text, "не готов") {
		t.Fatalf("нет понятной ошибки: %+v", m.toast)
	}
}

func TestCompletionFallsBackWithoutLSP(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("printf print\n"), "x.go"))
	m.tab().hlVersion = -1
	m.tab().wrapCache.valid = false
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 3})
	if cmd := m.triggerCompletion(); cmd != nil {
		t.Fatal("без LSP не должно быть запроса")
	}
	tab := m.tab()
	if !tab.comp.open || len(tab.comp.items) == 0 {
		t.Fatalf("нет fallback-кандидатов: %+v", tab.comp)
	}
	found := false
	for _, it := range tab.comp.items {
		if it.Label == "printf" {
			found = true
		}
	}
	if !found {
		t.Fatalf("кандидаты %+v", tab.comp.items)
	}
}

func TestCompletionStartingShowsStatus(t *testing.T) {
	m := testModel(t)
	typeText(m, "zz")
	tab := m.tab()
	tab.lspState = lspStarting
	tab.lspName = "gopls"
	m.triggerCompletion()
	if !strings.Contains(m.toast.Text, "запускается") {
		t.Fatalf("тост: %+v", m.toast)
	}
}

func TestCompletionAliasesAndCtrlO(t *testing.T) {
	b := defaultBindings()
	if b["ctrl+@"] != "completion.trigger" || b["ctrl+space"] != "completion.trigger" {
		t.Fatalf("completion aliases: %v", b)
	}
	if b["ctrl+o"] != "tab.new" {
		t.Fatalf("ctrl+o: %q", b["ctrl+o"])
	}
	if _, ok := b["alt+o"]; ok {
		t.Fatal("alt+o остался в дефолте")
	}
}

func TestCompletionEmptyPrefixAtDot(t *testing.T) {
	m := testModel(t)
	typeText(m, "fmt.")
	tab := m.tab()
	start, end := wordBounds(tab.ed.Buf.Line(0), tab.ed.Cursor.Col)
	if start != end || start != 4 {
		t.Fatalf("wordBounds после точки: %d..%d", start, end)
	}
	m.showCompletion(tab, 0, []lsp.CompletionItem{
		{Label: "Println", Kind: "func"},
		{Label: "Printf", Kind: "func"},
	}, "", start, end)
	if !tab.comp.open || m.ov.kind != overlayCompletion {
		t.Fatalf("popup с пустым префиксом не открылся: %+v", tab.comp)
	}
	if len(tab.comp.items) != 2 {
		t.Fatalf("элементы: %+v", tab.comp.items)
	}
}
