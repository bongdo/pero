package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/assist"
	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/highlight"
	"github.com/bongdo/pero/internal/lsp"
	"github.com/bongdo/pero/internal/widgets"
	"github.com/bongdo/pero/internal/workspace"
)

// Options - что открыть при старте.
type Options struct {
	Root    string     // корень проекта (абсолютный)
	Initial *ed.Buffer // явный файл (nil - восстановить сессию или пустой буфер)
	Empty   bool       // явно пустой буфер (аргумент "-" или каталог)
	Warning string     // предупреждение конфига - тостом после старта
}

type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayCommands
	overlayOpen
	overlaySaveAs
	overlayFind
	overlayThemes
	overlayHelp
	overlayConfirm
	overlayMenu
	overlayCompletion
	overlayHover
	overlayLocs
	overlayQuickfix
	overlayAI
)

// Tab - вкладка = буфер = файл.
type Tab struct {
	ed   *ed.Editor
	top  int // первая видимая экранная строка
	left int // горизонтальный сдвиг в ячейках (без softwrap)
	wrap bool

	mtime   time.Time
	deleted bool
	free    bool // вид прокручен колесом: не притягивать курсор

	// LSP/ghost.
	diags       []lsp.Diagnostic
	diagVersion int
	ghost       ghostState
	comp        completionState

	// t9: индекс слов буфера, лениво по версии.
	words    *assist.WordIndex
	wordsVer int

	lsp       *lsp.Client
	lspURI    string
	lspName   string
	lspState  lspState
	lspGen    int
	lspSendMu sync.Mutex
	lspSent   int

	hlVersion int
	hlLines   [][]highlight.Span

	// Пара скобок под курсором: считается в View, живёт только для отрисовки.
	bracketA, bracketB ed.Pos
	bracketOK          bool

	// roReason - почему буфер read-only ("binary"/"large"); пусто - без причины.
	roReason string

	wrapCache wrapCache
}

// visibleDiags - диагностики, которые относятся к текущей версии буфера.
// LSP присылает их асинхронно; старые позиции после правки показывать нельзя.
func (t *Tab) visibleDiags() []lsp.Diagnostic {
	if t == nil || t.diagVersion != t.ed.Buf.Version {
		return nil
	}
	return t.diags
}

type ghostState struct {
	text    string
	source  string
	version int
}

type completionState struct {
	open   bool
	items  []lsp.CompletionItem
	all    []lsp.CompletionItem
	sel    int
	scroll int

	query  string
	prefix string
	start  ed.Pos
	end    ed.Pos
	gen    int
}

type overlay struct {
	kind   overlayKind
	input  widgets.Input
	input2 widgets.Input
	focus2 bool
	sel    int
	scroll int

	// Открытие файла.
	files   []workspace.File
	matches []workspace.Match
	total   int
	loading bool
	ch      chan workspace.File
	cancel  context.CancelFunc
	gen     int

	// Поиск.
	findMatches []ed.Range
	findLines   map[int][]ed.Range
	cur         int
	replace     bool

	// Темы: исходная тема для Esc.
	themeOrig string

	confirm    confirmState
	menu       menuState
	helpScroll int

	// LSP-popup: наведение, определения/ссылки, quickfix.
	popTitle string
	hover    string
	locs     []lsp.Location
	acts     []lsp.CodeAction
	actTitle string
	actLine  int

	// AI: результат ждёт вставки.
	aiText    string
	aiTab     *Tab
	aiRange   ed.Range
	aiPos     ed.Pos
	aiReplace bool
	aiVersion int
	aiScroll  int
}

type dragState struct {
	active bool
	anchor ed.Pos
}

type clickState struct {
	when  time.Time
	count int
	pos   ed.Pos
	tab   int
}

// Model - состояние редактора.
type Model struct {
	cfg   config.Config
	theme config.Theme
	th    sgrTheme
	hl    *highlight.Highlighter

	tabs   []*Tab
	active int
	root   string
	w, h   int

	ov         overlay
	bindings   map[string]string
	recent     []string
	recentCmds []string
	lastQuery  string
	openGen    int

	toast    widgets.Toast
	toastSeq int
	toastCmd tea.Cmd

	quitting bool

	pendingPaste bool
	pasteSeq     int
	pasteTab     *Tab // вкладка, в которую вставляем: Ctrl-V не должен уехать в другую
	pendingClose int  // вкладка, которую закрыть после сохранения; -1 - нет

	drag      dragState
	click     clickState
	tabScroll int

	lspAuto    bool
	lspEvents  chan lspEvent
	lspApply   chan lsp.WorkspaceEdit
	hoverGen   int
	locsGen    int
	fmtGen     int
	fixGen     int
	resolveGen int
	compGen    int

	// Ghost: дебаунс и асинхронный LSP-запрос.
	ghostGen int
	ghostCmd tea.Cmd

	// AI: контекст отмены и поколение запроса.
	aiGen    int
	aiCancel context.CancelFunc
}

// New создаёт модель. Явный файл сильнее восстановления сессии.
func New(cfg config.Config, opts Options) *Model {
	cfg.Normalize()
	m := &Model{
		cfg:          cfg,
		w:            80,
		h:            24,
		hl:           highlight.New(),
		pendingClose: -1,
		theme:        config.ThemeByName(cfg.Theme),
		lspAuto:      !runningTests(),
		lspEvents:    make(chan lspEvent, lspEventBuffer),
		lspApply:     make(chan lsp.WorkspaceEdit, 8),
	}
	m.th = newSGRTheme(m.theme)
	m.hl.SetPalette(themePalette(m.theme))
	m.root = opts.Root
	if m.root == "" {
		if wd, err := os.Getwd(); err == nil {
			m.root = workspace.Root(wd)
		}
	}
	m.root = expandHome(m.root)

	var warning string
	if w := m.buildBindings(); len(w) > 0 {
		warning = strings.Join(w, "; ")
	}

	st, haveState := config.LoadState()
	if haveState {
		m.recent = append([]string(nil), st.Recent...)
		m.recentCmds = append([]string(nil), st.RecentCommands...)
	}

	skipped := 0
	switch {
	case opts.Initial != nil:
		m.tabs = append(m.tabs, m.newTab(opts.Initial))
	case !opts.Empty && m.cfg.Editor.RestoreSession && haveState:
		m.tabs, skipped = m.restoreTabs(st.Tabs)
	}
	if len(m.tabs) == 0 {
		m.tabs = append(m.tabs, m.newTab(ed.New("")))
	}
	m.active = 0
	if haveState && st.LastFile != "" {
		for i, t := range m.tabs {
			if t.ed.Buf.Path == st.LastFile {
				m.active = i
				break
			}
		}
	}

	if skipped > 0 {
		m.setToast("info", m.trf("пропущено вкладок: %d", "skipped tabs: %d", skipped))
	}
	if warning == "" {
		warning = opts.Warning
	} else if opts.Warning != "" {
		warning += "; " + opts.Warning
	}
	if warning != "" {
		m.setToast("warning", warning)
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitLSP(m.lspEvents), waitApply(m.lspApply)}
	if c := m.takeToastCmd(); c != nil {
		cmds = append(cmds, c)
	}
	if c := m.ensureActiveLSP(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

func (m *Model) tab() *Tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

func (m *Model) newTab(b *ed.Buffer) *Tab {
	e := ed.NewEditor(b)
	e.TabWidth = m.cfg.Editor.Tabsize
	t := &Tab{ed: e, wrap: m.cfg.SoftwrapFor(b.Path), hlVersion: -1, words: assist.NewWordIndex(), wordsVer: -1}
	if b.ReadOnly {
		t.roReason = readOnlyReason(b.Path)
	}
	t.mtime = fileMTime(b.Path)
	return t
}

func (m *Model) restoreTabs(tabs []config.TabState) ([]*Tab, int) {
	var out []*Tab
	skipped := 0
	for _, st := range tabs {
		if strings.TrimSpace(st.Path) == "" {
			skipped++
			continue
		}
		path := expandHome(st.Path)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			skipped++
			continue
		}
		b, err := ed.Open(path)
		if err != nil {
			skipped++
			continue
		}
		t := m.newTab(b)
		t.ed.Cursor = b.Clamp(ed.Pos{Line: st.CursorLine, Col: st.CursorCol})
		out = append(out, t)
	}
	return out, skipped
}

// Update - единственный цикл; горутины общаются только сообщениями.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.ov.kind == overlayMenu {
			m.closeMenu()
		}
		m.clampScroll()
	case tea.KeyPressMsg:
		cmd = m.key(msg)
	case tea.FocusMsg:
		// Вернулись из другого окна - файл на диске мог измениться.
		if m.ov.kind == overlayNone {
			m.checkExternalActive()
		}
	case tea.PasteMsg:
		cmd = m.pasteText(msg.Content)
	case tea.PasteStartMsg, tea.PasteEndMsg:
		// bracketed paste: содержимое приходит отдельным PasteMsg
	case tea.MouseClickMsg:
		cmd = m.mouseClick(msg.Mouse())
	case tea.MouseReleaseMsg:
		cmd = m.mouseRelease(msg.Mouse())
	case tea.MouseMotionMsg:
		cmd = m.mouseMotion(msg.Mouse())
	case tea.MouseWheelMsg:
		cmd = m.mouseWheel(msg.Mouse())
	case tea.ClipboardMsg:
		cmd = m.onClipboard(msg.Content)
	case pasteWaitMsg:
		cmd = m.onPasteWait(msg)
	case pasteFallbackMsg:
		cmd = m.onPasteFallback(msg)
	case toastTickMsg:
		if msg.seq == m.toastSeq && !m.toast.Until.IsZero() && !time.Now().Before(m.toast.Until) {
			m.toast = widgets.Toast{}
		}
	case openFilesMsg:
		cmd = m.onOpenFiles(msg)
	case openDoneMsg:
		if msg.gen == m.ov.gen {
			m.ov.loading = false
		}
	case lspEvent:
		cmd = m.onLSPEvent(msg)
	case lspApplyMsg:
		if !m.quitting {
			cmd = tea.Batch(m.applyWorkspaceEdit(&msg.edit, "LSP"), waitApply(m.lspApply))
		}
	case lspStartedMsg:
		cmd = m.onLSPStarted(msg)
	case lspHoverMsg:
		cmd = m.onHover(msg)
	case lspLocsMsg:
		cmd = m.onLocs(msg)
	case lspFormatMsg:
		cmd = m.onFormat(msg)
	case lspActionsMsg:
		cmd = m.onActions(msg)
	case lspResolveMsg:
		cmd = m.onResolve(msg)
	case lspCommandMsg:
		if msg.err != nil {
			m.notifyError(m.tr("команда LSP: ", "LSP command: ") + m.lspErrorText(msg.err))
		} else {
			m.notifySuccess(m.tr("выполнено: ", "done: ") + msg.title)
		}
	case completionMsg:
		cmd = m.onCompletion(msg)
	case ghostTickMsg:
		cmd = m.onGhostTick(msg)
	case ghostMsg:
		cmd = m.onGhostResult(msg)
	case aiMsg:
		cmd = m.onAI(msg)
	}
	// Любой оверлей прячет ghost и не даёт старому таймеру его вернуть.
	if m.ov.kind != overlayNone {
		for _, t := range m.tabs {
			t.ghost = ghostState{}
		}
	}
	// Ghost живёт только в активной вкладке.
	if cur := m.tab(); cur != nil {
		for _, t := range m.tabs {
			if t != cur {
				t.ghost = ghostState{}
			}
		}
	}
	if lspCmd := m.ensureActiveLSP(); lspCmd != nil {
		if cmd == nil {
			cmd = lspCmd
		} else {
			cmd = tea.Batch(cmd, lspCmd)
		}
	}
	if t := m.takeToastCmd(); t != nil {
		if cmd == nil {
			cmd = t
		} else {
			cmd = tea.Batch(cmd, t)
		}
	}
	if g := m.takeGhostCmd(); g != nil {
		if cmd == nil {
			cmd = g
		} else {
			cmd = tea.Batch(cmd, g)
		}
	}
	return m, cmd
}

func (m *Model) takeGhostCmd() tea.Cmd {
	c := m.ghostCmd
	m.ghostCmd = nil
	return c
}

func (m *Model) takeToastCmd() tea.Cmd {
	c := m.toastCmd
	m.toastCmd = nil
	return c
}

type toastTickMsg struct {
	seq   int
	until time.Time
}

func (m *Model) setToast(kind, text string) {
	text = sanitizeOneLine(text)
	if strings.TrimSpace(text) == "" {
		return
	}
	// Ошибка не вытесняется менее важным сообщением до Esc.
	if m.toast.Kind == "error" && kind != "error" && (m.toast.Until.IsZero() || !time.Now().After(m.toast.Until)) {
		return
	}
	m.toastSeq++
	now := time.Now()
	t := widgets.Toast{Text: text, Kind: kind}
	var d time.Duration
	switch kind {
	case "success":
		d = 2 * time.Second
	case "info":
		d = 3 * time.Second
	case "warning":
		d = 5 * time.Second
	case "error", "progress":
		// error живёт до Esc; progress - до завершения.
	default:
		d = 3 * time.Second
	}
	if d > 0 {
		t.Until = now.Add(d)
		seq := m.toastSeq
		until := t.Until
		m.toastCmd = tea.Tick(d, func(time.Time) tea.Msg {
			return toastTickMsg{seq: seq, until: until}
		})
	}
	m.toast = t
}

func (m *Model) notifySuccess(text string) { m.setToast("success", text) }
func (m *Model) notifyInfo(text string)    { m.setToast("info", text) }
func (m *Model) notifyWarn(text string)    { m.setToast("warning", text) }
func (m *Model) notifyError(text string)   { m.setToast("error", text) }

// canEdit проверяет read-only и отвечает тостом.
func (m *Model) canEdit() bool {
	t := m.tab()
	if t == nil {
		return false
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: изменения запрещены", "read-only: editing is disabled"))
		return false
	}
	return true
}

func (m *Model) afterEdit() {
	if t := m.tab(); t != nil {
		m.afterEditTab(t)
	}
}

func (m *Model) afterEditTab(t *Tab) {
	if t == nil {
		return
	}
	t.ghost = ghostState{}
	t.deleted = false
	m.lspDidChange(t)
	// Ghost - только для активной вкладки: фоновая правка не должна
	// показывать подсказку поверх чужого текста.
	if t == m.tab() {
		m.scheduleGhost()
	}
}

func (m *Model) insertText(s string) {
	if s == "" {
		return
	}
	t := m.tab()
	if t == nil || t.ed.Buf.ReadOnly {
		m.canEdit()
		return
	}
	t.ed.Insert(s)
	m.afterEdit()
}

func (m *Model) replaceRange(r ed.Range, s string) {
	t := m.tab()
	if t == nil || t.ed.Buf.ReadOnly {
		m.canEdit()
		return
	}
	t.ed.ReplaceRange(r, s)
	m.afterEdit()
}

func (m *Model) clampScroll() {
	t := m.tab()
	if t == nil {
		return
	}
	lay := m.layout()
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	maxTop := max(0, len(rows)-lay.text.h)
	t.top = max(0, min(t.top, maxTop))
	if t.left < 0 {
		t.left = 0
	}
}

// runningTests не даёт тестам UI поднять настоящий язык-сервер: в go test
// бинарь всегда оканчивается на .test.
func runningTests() bool {
	name := strings.ToLower(os.Args[0])
	return strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe")
}

func fileMTime(path string) time.Time {
	if path == "" {
		return time.Time{}
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
