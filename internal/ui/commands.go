package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

// command - действие, доступное и с клавиши, и из командной строки (Ctrl-E).
type command struct {
	id        string
	title     string
	titleEN   string
	key       string
	section   string
	sectionEN string
	available func(*Model) bool
	run       func(m *Model) tea.Cmd
}

func (c command) titleFor(lang string) string {
	if lang == "ru" || c.titleEN == "" {
		return c.title
	}
	return c.titleEN
}

func (c command) sectionFor(lang string) string {
	if lang == "ru" || c.sectionEN == "" {
		return c.section
	}
	return c.sectionEN
}

func (c command) enabled(m *Model) bool {
	return c.available == nil || c.available(m)
}

func (m *Model) commands() []command {
	always := func(*Model) bool { return true }
	editOK := func(m *Model) bool { return m.tab() != nil && !m.tab().ed.Buf.ReadOnly }
	hasSel := func(m *Model) bool { return m.tab() != nil && m.tab().ed.HasSelection() }
	proseOK := func(m *Model) bool {
		t := m.tab()
		return t != nil && !t.ed.Buf.ReadOnly && m.isProsePath(t.ed.Buf.Path)
	}
	diagOK := func(m *Model) bool { return m.tab() != nil && len(m.tab().visibleDiags()) > 0 }
	lspOK := func(m *Model) bool { return m.lspReadyFor(m.tab()) }
	lspEditOK := func(m *Model) bool {
		t := m.tab()
		return t != nil && !t.ed.Buf.ReadOnly && m.lspReadyFor(t)
	}

	cs := []command{
		// Файлы.
		{id: "file.new", title: "Новый пустой буфер", titleEN: "New empty buffer", section: "Файлы", sectionEN: "Files", run: func(m *Model) tea.Cmd { m.newEmptyTab(); return nil }},
		{id: "file.open", title: "Открыть файл", titleEN: "Open file", key: "Ctrl-P", section: "Файлы", sectionEN: "Files", available: always, run: func(m *Model) tea.Cmd { return m.openOpenPalette() }},
		{id: "file.save", title: "Сохранить файл", titleEN: "Save file", key: "Ctrl-S", section: "Файлы", sectionEN: "Files", available: always, run: func(m *Model) tea.Cmd { return m.saveCurrent() }},
		{id: "file.saveAs", title: "Сохранить как", titleEN: "Save as", key: "Ctrl-Shift-S", section: "Файлы", sectionEN: "Files", available: editOK, run: func(m *Model) tea.Cmd { m.openSaveAs(); return nil }},

		// Вкладки.
		{id: "tab.new", title: "Новая вкладка и открыть файл", titleEN: "New tab and open file", key: "Ctrl-O", section: "Вкладки", sectionEN: "Tabs", available: always, run: func(m *Model) tea.Cmd { return m.newTabWithPalette() }},
		{id: "tab.close", title: "Закрыть вкладку", titleEN: "Close tab", key: "Alt-Q", section: "Вкладки", sectionEN: "Tabs", available: always, run: func(m *Model) tea.Cmd { return m.closeTabCmd(m.active) }},
		{id: "tab.closeOthers", title: "Закрыть остальные вкладки", titleEN: "Close other tabs", section: "Вкладки", sectionEN: "Tabs", available: func(m *Model) bool { return len(m.tabs) > 1 }, run: func(m *Model) tea.Cmd { return m.closeOthersCmd(m.active) }},
		{id: "tab.next", title: "Следующая вкладка", titleEN: "Next tab", key: "Alt-J / Ctrl-Tab", section: "Вкладки", sectionEN: "Tabs", available: func(m *Model) bool { return len(m.tabs) > 1 }, run: func(m *Model) tea.Cmd { m.switchTab(1); return nil }},
		{id: "tab.prev", title: "Предыдущая вкладка", titleEN: "Previous tab", key: "Alt-K / Ctrl-Shift-Tab", section: "Вкладки", sectionEN: "Tabs", available: func(m *Model) bool { return len(m.tabs) > 1 }, run: func(m *Model) tea.Cmd { m.switchTab(-1); return nil }},

		// Правка.
		{id: "edit.undo", title: "Отменить", titleEN: "Undo", key: "Ctrl-Z", section: "Правка", sectionEN: "Edit", available: func(m *Model) bool { return m.tab() != nil && m.tab().ed.CanUndo() }, run: func(m *Model) tea.Cmd { m.undo(); return nil }},
		{id: "edit.redo", title: "Повторить", titleEN: "Redo", key: "Ctrl-Y", section: "Правка", sectionEN: "Edit", available: func(m *Model) bool { return m.tab() != nil && m.tab().ed.CanRedo() }, run: func(m *Model) tea.Cmd { m.redo(); return nil }},
		{id: "edit.copy", title: "Копировать", titleEN: "Copy", key: "Ctrl-C", section: "Правка", sectionEN: "Edit", available: hasSel, run: func(m *Model) tea.Cmd { return m.copySelection(false) }},
		{id: "edit.cut", title: "Вырезать", titleEN: "Cut", key: "Ctrl-X", section: "Правка", sectionEN: "Edit", available: func(m *Model) bool { return editOK(m) && hasSel(m) }, run: func(m *Model) tea.Cmd { return m.copySelection(true) }},
		{id: "edit.paste", title: "Вставить", titleEN: "Paste", key: "Ctrl-V", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { return m.pasteClipboard() }},
		{id: "edit.cutLine", title: "Вырезать строку", titleEN: "Cut line", key: "Ctrl-K", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { return m.cutLine() }},
		{id: "edit.deleteWordLeft", title: "Удалить слово слева", titleEN: "Delete word left", key: "Alt-Backspace", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { m.deleteWordLeft(); return nil }},
		{id: "edit.duplicateLine", title: "Дублировать строку", titleEN: "Duplicate line", key: "Ctrl-D", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { m.duplicateLine(); return nil }},
		{id: "edit.selectAll", title: "Выделить всё", titleEN: "Select all", key: "Ctrl-A", section: "Правка", sectionEN: "Edit", available: always, run: func(m *Model) tea.Cmd { m.selectAll(); return nil }},
		{id: "edit.indent", title: "Отступ", titleEN: "Indent", key: "Tab", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { return m.indent() }},
		{id: "edit.outdent", title: "Снять отступ", titleEN: "Outdent", key: "Shift-Tab", section: "Правка", sectionEN: "Edit", available: editOK, run: func(m *Model) tea.Cmd { m.outdent(); return nil }},

		// Поиск.
		{id: "edit.find", title: "Найти", titleEN: "Find", key: "Ctrl-F", section: "Поиск", sectionEN: "Search", available: always, run: func(m *Model) tea.Cmd { m.openFind(false); return nil }},
		{id: "edit.replace", title: "Найти и заменить", titleEN: "Find and replace", key: "Ctrl-H", section: "Поиск", sectionEN: "Search", available: editOK, run: func(m *Model) tea.Cmd { m.openFind(true); return nil }},
		{id: "edit.findNext", title: "Следующее совпадение", titleEN: "Next match", key: "F3", section: "Поиск", sectionEN: "Search", available: func(m *Model) bool { return m.lastQuery != "" }, run: func(m *Model) tea.Cmd { m.findAgain(true); return nil }},
		{id: "edit.findPrev", title: "Предыдущее совпадение", titleEN: "Previous match", key: "Shift-F3", section: "Поиск", sectionEN: "Search", available: func(m *Model) bool { return m.lastQuery != "" }, run: func(m *Model) tea.Cmd { m.findAgain(false); return nil }},

		// Markdown.
		{id: "markdown.checkbox", title: "Переключить чекбокс", titleEN: "Toggle checkbox", key: "Alt-t", section: "Markdown", sectionEN: "Markdown", available: func(m *Model) bool { return proseOK(m) && m.cfg.Markdown.Checkbox }, run: func(m *Model) tea.Cmd { m.toggleCheckbox(); return nil }},
		{id: "markdown.spaces", title: "Схлопнуть повторные пробелы", titleEN: "Collapse repeated spaces", key: "Alt-s", section: "Markdown", sectionEN: "Markdown", available: proseOK, run: func(m *Model) tea.Cmd { m.collapseSpaces(); return nil }},

		// Вид.
		{id: "view.wrap", title: "Перенос строк вкл/выкл", titleEN: "Toggle soft wrap", section: "Вид", sectionEN: "View", available: always, run: func(m *Model) tea.Cmd { m.toggleWrap(); return nil }},
		{id: "view.ghost", title: "Подсказки вкл/выкл", titleEN: "Toggle suggestions", section: "Вид", sectionEN: "View", available: always, run: func(m *Model) tea.Cmd { m.toggleGhost(); return nil }},
		{id: "theme.set", title: "Сменить тему", titleEN: "Theme…", section: "Вид", sectionEN: "View", available: always, run: func(m *Model) tea.Cmd { m.openThemeList(); return nil }},

		// Интерфейс.
		{id: "palette.commands", title: "Командная строка", titleEN: "Command palette", key: "Ctrl-E", section: "Интерфейс", sectionEN: "Interface", available: always, run: func(m *Model) tea.Cmd { m.openCommandPalette(); return nil }},
		{id: "help.keys", title: "Показать все клавиши", titleEN: "Show all key bindings", key: "F1", section: "Интерфейс", sectionEN: "Interface", available: always, run: func(m *Model) tea.Cmd { m.openHelp(); return nil }},
		{id: "config.reload", title: "Перечитать конфиг", titleEN: "Reload config", section: "Интерфейс", sectionEN: "Interface", available: always, run: func(m *Model) tea.Cmd { return m.reloadConfig() }},
		{id: "app.quit", title: "Выйти из редактора", titleEN: "Quit editor", key: "Ctrl-Q", section: "Интерфейс", sectionEN: "Interface", available: always, run: func(m *Model) tea.Cmd { return m.quitCmd() }},

		// LSP и completion.
		{id: "lsp.hover", title: "Подсказка LSP", titleEN: "LSP hover", key: "Alt-u", section: "LSP", sectionEN: "LSP", available: lspOK, run: func(m *Model) tea.Cmd { return m.hoverCmd() }},
		{id: "lsp.definition", title: "Определение", titleEN: "Definition", key: "Alt-d", section: "LSP", sectionEN: "LSP", available: lspOK, run: func(m *Model) tea.Cmd { return m.locationsCmd(true) }},
		{id: "lsp.references", title: "Ссылки", titleEN: "References", key: "Alt-i", section: "LSP", sectionEN: "LSP", available: lspOK, run: func(m *Model) tea.Cmd { return m.locationsCmd(false) }},
		{id: "lsp.format", title: "Форматировать", titleEN: "Format", key: "Alt-G", section: "LSP", sectionEN: "LSP", available: lspEditOK, run: func(m *Model) tea.Cmd { return m.formatCmd() }},
		{id: "lsp.codeaction", title: "Исправить", titleEN: "Quick fix", key: "Alt-.", section: "LSP", sectionEN: "LSP", available: lspEditOK, run: func(m *Model) tea.Cmd { return m.quickfixCmd() }},
		{id: "lsp.nextDiag", title: "Следующая диагностика", titleEN: "Next diagnostic", key: "F8", section: "LSP", sectionEN: "LSP", available: diagOK, run: func(m *Model) tea.Cmd { return m.jumpDiagnostic(1) }},
		{id: "lsp.prevDiag", title: "Предыдущая диагностика", titleEN: "Previous diagnostic", key: "Shift-F8", section: "LSP", sectionEN: "LSP", available: diagOK, run: func(m *Model) tea.Cmd { return m.jumpDiagnostic(-1) }},
		{id: "lsp.restart", title: "Перезапустить LSP", titleEN: "Restart LSP", section: "LSP", sectionEN: "LSP", available: func(m *Model) bool { t := m.tab(); return t != nil && t.ed.Buf.Path != "" }, run: func(m *Model) tea.Cmd { return m.restartLSP(m.tab()) }},
		{id: "completion.trigger", title: "Дополнение", titleEN: "Completion", key: "Ctrl-Space", section: "LSP", sectionEN: "LSP", available: editOK, run: func(m *Model) tea.Cmd { return m.triggerCompletion() }},
		{id: "ai.suggest", title: "Подсказка AI", titleEN: "AI suggestion", key: "Alt-a", section: "LSP", sectionEN: "LSP", available: always, run: func(m *Model) tea.Cmd { return m.aiSuggest() }},
	}
	cs = append(cs, m.navCommands()...)
	return cs
}

// navCommands - движения курсора тоже команды: клавиши не содержат логики.
func (m *Model) navCommands() []command {
	type spec struct {
		id, title, titleEN, key string
		fn                      func(e *ed.Editor, extend bool)
		screen                  int
		page                    int
		para                    int
	}
	specs := []spec{
		{id: "cursor.left", title: "Курсор влево", titleEN: "Cursor left", key: "←", fn: (*ed.Editor).MoveLeft},
		{id: "cursor.right", title: "Курсор вправо", titleEN: "Cursor right", key: "→", fn: (*ed.Editor).MoveRight},
		{id: "cursor.up", title: "Курсор вверх", titleEN: "Cursor up", key: "↑", screen: -1},
		{id: "cursor.down", title: "Курсор вниз", titleEN: "Cursor down", key: "↓", screen: 1},
		{id: "cursor.wordLeft", title: "Курсор на слово влево", titleEN: "Word left", key: "Ctrl-←", fn: (*ed.Editor).MoveWordLeft},
		{id: "cursor.wordRight", title: "Курсор на слово вправо", titleEN: "Word right", key: "Ctrl-→", fn: (*ed.Editor).MoveWordRight},
		{id: "cursor.home", title: "В начало строки", titleEN: "Line start", key: "Home", fn: (*ed.Editor).MoveHome},
		{id: "cursor.end", title: "В конец строки", titleEN: "Line end", key: "End", fn: (*ed.Editor).MoveEnd},
		{id: "cursor.docStart", title: "В начало файла", titleEN: "File start", key: "Ctrl-Home", fn: (*ed.Editor).MoveDocStart},
		{id: "cursor.docEnd", title: "В конец файла", titleEN: "File end", key: "Ctrl-End", fn: (*ed.Editor).MoveDocEnd},
		{id: "cursor.pageUp", title: "Страница вверх", titleEN: "Page up", key: "PgUp", page: -1},
		{id: "cursor.pageDown", title: "Страница вниз", titleEN: "Page down", key: "PgDn", page: 1},
		{id: "cursor.paragraphPrev", title: "Абзац назад", titleEN: "Paragraph back", key: "Alt-{", para: -1},
		{id: "cursor.paragraphNext", title: "Абзац вперёд", titleEN: "Paragraph forward", key: "Alt-}", para: 1},
	}
	out := make([]command, 0, len(specs)*2)
	for _, s := range specs {
		s := s
		makeRun := func(extend bool) func(*Model) tea.Cmd {
			return func(m *Model) tea.Cmd {
				t := m.tab()
				if t == nil {
					return nil
				}
				switch {
				case s.id == "cursor.home":
					m.smartHome(extend)
				case s.para != 0:
					m.moveParagraph(s.para, extend)
				case s.page != 0:
					m.moveScreen(s.page*m.screenHeight(), extend)
				case s.screen != 0:
					m.moveScreen(s.screen, extend)
				case s.fn != nil:
					s.fn(t.ed, extend)
				}
				m.afterMove()
				return nil
			}
		}
		out = append(out, command{
			id: s.id, title: s.title, titleEN: s.titleEN, key: s.key,
			section: "Навигация", sectionEN: "Navigation",
			available: func(*Model) bool { return true },
			run:       makeRun(false),
		})
		out = append(out, command{
			id: s.id + ".extend", title: s.title + " (с выделением)", titleEN: s.titleEN + " (with selection)",
			key: "Shift-" + s.key, section: "Навигация", sectionEN: "Navigation",
			available: func(*Model) bool { return true },
			run:       makeRun(true),
		})
	}
	return out
}

// smartHome - Home на первый непробельный символ, повторно - в колонку 0.
func (m *Model) smartHome(extend bool) {
	t := m.tab()
	if t == nil {
		return
	}
	e := t.ed
	line := e.Buf.Line(e.Cursor.Line)
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	if e.Cursor.Col == col {
		col = 0
	}
	p := ed.Pos{Line: e.Cursor.Line, Col: col}
	if extend {
		anchor := e.Cursor
		if e.HasSelection() {
			anchor = e.Sel.Anchor
		}
		e.SetSelection(anchor, p)
		return
	}
	e.SetCursor(p)
}

func (m *Model) afterMove() {
	m.clampScroll()
	m.scheduleGhost()
}

// moveParagraph - Alt-{ / Alt-}: к предыдущему/следующему пустому разделителю.
func (m *Model) moveParagraph(delta int, extend bool) {
	t := m.tab()
	if t == nil || delta == 0 {
		return
	}
	last := t.ed.Buf.LastLine()
	line := t.ed.Cursor.Line
	if delta > 0 {
		line++
		for line <= last && strings.TrimSpace(string(t.ed.Buf.Line(line))) != "" {
			line++
		}
	} else {
		line--
		for line > 0 && strings.TrimSpace(string(t.ed.Buf.Line(line))) != "" {
			line--
		}
	}
	line = max(0, min(line, last))
	pos := ed.Pos{Line: line}
	if extend {
		anchor := t.ed.Cursor
		if t.ed.Sel.Active {
			anchor = t.ed.Sel.Anchor
		}
		t.ed.SetSelection(anchor, pos)
	} else {
		t.ed.SetCursor(pos)
	}
	t.free = false
	m.clampScroll()
}

// filterCommands - подстрока и подпоследовательность, префикс важнее.
// Ищем по id и обоим названиям: русский запрос находит английский UI и наоборот.
func filterCommands(all []command, query string) []command {
	q := strings.TrimSpace(query)
	if q == "" {
		return all
	}
	type row struct {
		c command
		s int
	}
	rows := make([]row, 0, len(all))
	for _, c := range all {
		if s := commandScore(c, q); s > 0 {
			rows = append(rows, row{c, s})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].s > rows[j].s })
	out := make([]command, len(rows))
	for i, r := range rows {
		out[i] = r.c
	}
	return out
}

func commandScore(c command, q string) int {
	s := matchScore(c.id, q)
	if t := matchScore(c.title, q); t > s {
		s = t
	}
	if t := matchScore(c.titleEN, q); t > s {
		s = t
	}
	return s
}

func matchScore(s, q string) int {
	s = strings.ToLower(s)
	q = strings.ToLower(q)
	switch {
	case strings.HasPrefix(s, q):
		return 3
	case strings.Contains(s, q):
		return 2
	case isSubsequence(s, q):
		return 1
	}
	return -1
}

func isSubsequence(s, q string) bool {
	rs, rq := []rune(s), []rune(q)
	j := 0
	for _, r := range rs {
		if j < len(rq) && r == rq[j] {
			j++
		}
	}
	return j == len(rq)
}

// runCommand - единственный путь выполнения: и клавиши, и палитра идут через реестр.
func (m *Model) runCommand(id string) tea.Cmd {
	for _, c := range m.commands() {
		if c.id == id {
			m.rememberCommand(id)
			return c.run(m)
		}
	}
	return nil
}

func (m *Model) rememberCommand(id string) {
	if id == "" || id == "app.quit" || id == "palette.commands" || id == "edit.indent" || id == "edit.outdent" ||
		strings.HasPrefix(id, "cursor.") {
		return
	}
	out := []string{id}
	for _, old := range m.recentCmds {
		if old != id && len(out) < 10 {
			out = append(out, old)
		}
	}
	m.recentCmds = out
}

// defaultBindings - карта клавиш из docs/UX/Карта клавиш.md.
func defaultBindings() map[string]string {
	return map[string]string{
		"ctrl+s":         "file.save",
		"ctrl+shift+s":   "file.saveAs",
		"ctrl+p":         "file.open",
		"ctrl+e":         "palette.commands",
		"ctrl+o":         "tab.new",
		"alt+q":          "tab.close",
		"alt+j":          "tab.next",
		"alt+k":          "tab.prev",
		"ctrl+tab":       "tab.next",
		"ctrl+shift+tab": "tab.prev",
		"ctrl+q":         "app.quit",

		"ctrl+z":        "edit.undo",
		"ctrl+y":        "edit.redo",
		"ctrl+a":        "edit.selectAll",
		"ctrl+c":        "edit.copy",
		"ctrl+x":        "edit.cut",
		"ctrl+v":        "edit.paste",
		"ctrl+k":        "edit.cutLine",
		"ctrl+d":        "edit.duplicateLine",
		"alt+backspace": "edit.deleteWordLeft",
		"tab":           "edit.indent",
		"shift+tab":     "edit.outdent",
		"ctrl+space":    "completion.trigger",
		"ctrl+@":        "completion.trigger",
		"alt+a":         "ai.suggest",

		"ctrl+f":   "edit.find",
		"ctrl+h":   "edit.replace",
		"f3":       "edit.findNext",
		"shift+f3": "edit.findPrev",

		"alt+t": "markdown.checkbox",
		"alt+s": "markdown.spaces",

		"f1":       "help.keys",
		"alt+u":    "lsp.hover",
		"alt+d":    "lsp.definition",
		"alt+i":    "lsp.references",
		"alt+g":    "lsp.format",
		"alt+.":    "lsp.codeaction",
		"f8":       "lsp.nextDiag",
		"shift+f8": "lsp.prevDiag",

		"left":             "cursor.left",
		"right":            "cursor.right",
		"up":               "cursor.up",
		"down":             "cursor.down",
		"shift+left":       "cursor.left.extend",
		"shift+right":      "cursor.right.extend",
		"shift+up":         "cursor.up.extend",
		"shift+down":       "cursor.down.extend",
		"ctrl+left":        "cursor.wordLeft",
		"ctrl+right":       "cursor.wordRight",
		"ctrl+shift+left":  "cursor.wordLeft.extend",
		"ctrl+shift+right": "cursor.wordRight.extend",
		"home":             "cursor.home",
		"end":              "cursor.end",
		"shift+home":       "cursor.home.extend",
		"shift+end":        "cursor.end.extend",
		"ctrl+home":        "cursor.docStart",
		"ctrl+end":         "cursor.docEnd",
		"ctrl+up":          "cursor.docStart",
		"ctrl+down":        "cursor.docEnd",
		"ctrl+shift+home":  "cursor.docStart.extend",
		"ctrl+shift+end":   "cursor.docEnd.extend",
		"ctrl+shift+up":    "cursor.docStart.extend",
		"ctrl+shift+down":  "cursor.docEnd.extend",
		"pgup":             "cursor.pageUp",
		"pgdown":           "cursor.pageDown",
		"shift+pgup":       "cursor.pageUp.extend",
		"shift+pgdown":     "cursor.pageDown.extend",
		"alt+{":            "cursor.paragraphPrev",
		"alt+}":            "cursor.paragraphNext",
		"alt+[":            "cursor.paragraphPrev",
		"alt+]":            "cursor.paragraphNext",
		"alt+shift+{":      "cursor.paragraphPrev.extend",
		"alt+shift+}":      "cursor.paragraphNext.extend",
	}
}

func (m *Model) buildBindings() []string {
	m.bindings = defaultBindings()
	if len(m.cfg.Keys) == 0 {
		return nil
	}
	ids := make(map[string]bool)
	for _, c := range m.commands() {
		ids[c.id] = true
	}
	keys := make([]string, 0, len(m.cfg.Keys))
	for k := range m.cfg.Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var warns []string
	for _, raw := range keys {
		id := strings.TrimSpace(m.cfg.Keys[raw])
		key := normalizeKey(raw)
		if key == "" {
			continue
		}
		if !ids[id] {
			warns = append(warns, m.tr("keys: неизвестная команда ", "keys: unknown command ")+id)
			continue
		}
		if old, ok := m.bindings[key]; ok && old != id {
			warns = append(warns, m.trf("keys: %s уже занята, не перетираю", "keys: %s is already bound, not overriding", raw))
			continue
		}
		m.bindings[key] = id
	}
	return warns
}

// normalizeKey приводит "Ctrl-Shift-S" и "ctrl+shift+s" к виду ctrl+shift+s.
func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '+' || r == '-' })
	if len(parts) == 0 {
		return ""
	}
	mods := map[string]string{"ctrl": "ctrl", "control": "ctrl", "alt": "alt", "opt": "alt", "shift": "shift", "meta": "meta", "cmd": "super", "super": "super"}
	order := []string{"ctrl", "alt", "shift", "meta", "super"}
	seen := map[string]bool{}
	var key string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if m, ok := mods[p]; ok {
			seen[m] = true
		} else {
			key = p
		}
	}
	if key == "" {
		return ""
	}
	var out []string
	for _, o := range order {
		if seen[o] {
			out = append(out, o)
		}
	}
	out = append(out, key)
	return strings.Join(out, "+")
}

func (m *Model) commandList() []command {
	all := m.commands()
	out := make([]command, 0, len(all))
	for _, c := range all {
		if c.enabled(m) {
			out = append(out, c)
		}
	}
	return out
}
