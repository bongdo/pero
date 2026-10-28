package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/assist"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
	"github.com/bongdo/pero/internal/widgets"
)

type lspState int

const (
	lspNone lspState = iota
	lspStarting
	lspReady
	lspOff
	lspFailed
)

const (
	lspEventBuffer = 64
	lspStartupWait = 30 * time.Second
)

// lspEvent - диагностика, пришедшая от конкретного клиента. Клиент и URI
// проверяются в Update: события закрытой вкладки не попадают в UI.
type lspEvent struct {
	client     *lsp.Client
	uri        string
	diags      []lsp.Diagnostic
	version    int
	hasVersion bool
}

type lspApplyMsg struct {
	edit lsp.WorkspaceEdit
}

// waitApply отдаёт серверные workspace/applyEdit в UI-цикл.
func waitApply(ch <-chan lsp.WorkspaceEdit) tea.Cmd {
	return func() tea.Msg {
		return lspApplyMsg{edit: <-ch}
	}
}

type lspStartedMsg struct {
	tab     *Tab
	client  *lsp.Client
	gen     int
	version int
	err     error
}

type lspHoverMsg struct {
	tab     *Tab
	gen     int
	version int
	hover   *lsp.Hover
	err     error
}

type lspLocsMsg struct {
	tab       *Tab
	gen       int
	version   int
	direct    bool
	title     string
	locations []lsp.Location
	err       error
}

type lspFormatMsg struct {
	tab     *Tab
	gen     int
	version int
	edits   []lsp.TextEdit
	err     error
}

// waitLSP блокируется на канале событий и не даёт потерять диагностику.
func waitLSP(ch <-chan lspEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return ev
	}
}

func (m *Model) ensureActiveLSP() tea.Cmd {
	if !m.lspAuto || m.quitting {
		return nil
	}
	return m.lspEnsure(m.tab())
}

func (m *Model) tabAlive(t *Tab) bool {
	for _, have := range m.tabs {
		if have == t {
			return true
		}
	}
	return false
}

func (m *Model) tabForClient(client *lsp.Client, uri string) *Tab {
	for _, t := range m.tabs {
		if t.lsp == client && t.lspURI == uri {
			return t
		}
	}
	return nil
}

func (m *Model) tabForPath(path string) *Tab {
	path = filepath.Clean(path)
	for _, t := range m.tabs {
		if t.ed.Buf.Path != "" && filepath.Clean(t.ed.Buf.Path) == path {
			return t
		}
	}
	return nil
}

func (m *Model) lspReadyFor(t *Tab) bool {
	return t != nil && t.lsp != nil && t.lspState == lspReady
}

func (m *Model) lspOffToast(t *Tab) {
	if t == nil {
		return
	}
	off := "LSP off"
	if t.lspName != "" {
		off += ": " + t.lspName
	}
	switch t.lspState {
	case lspStarting:
		m.notifyInfo(m.tr("LSP ещё запускается…", "LSP is starting…"))
	case lspFailed, lspOff:
		m.notifyInfo(off)
	default:
		m.notifyInfo(off)
	}
}

// lspErrorText прячет внутреннюю ошибку сервера за понятной подсказкой.
func (m *Model) lspErrorText(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "internal error") || strings.Contains(msg, "-32603") {
		return m.tr("LSP ещё не готов, повторите", "LSP is not ready yet; try again")
	}
	return err.Error()
}

// lspEnsure поднимает сервер для вкладки один раз. Process start и initialize
// идут в tea.Cmd; модель меняется только при возврате lspStartedMsg.
func (m *Model) lspEnsure(t *Tab) tea.Cmd {
	if t == nil || !m.lspAuto || t.lsp != nil || t.lspState != lspNone || t.ed.Buf.Path == "" {
		return nil
	}
	command, lang := m.lspCommandFor(t)
	if command == "" {
		t.lspState = lspOff
		return nil
	}
	if m.lspEvents == nil {
		m.lspEvents = make(chan lspEvent, lspEventBuffer)
	}
	uri := lsp.PathToURI(t.ed.Buf.Path)
	cfg := lsp.ServerConfigFromCommand(lang, command)
	client := lsp.NewClient(cfg, m.root)
	if base := filepath.Base(cfg.Command); base == "ltex-ls" || base == "ltex" {
		language := "en-US"
		if assist.DetectLanguage(firstKB(t.ed.Buf.Text(), 8<<10)) == "ru" {
			language = "ru-RU"
		}
		// ltex спрашивает workspace/configuration во время initialize.
		client.SetConfiguration(map[string]any{
			"ltex": map[string]any{"language": language},
		})
	} else if base == "harper-ls" || base == "harper" {
		client.SetConfiguration(map[string]any{"harper-ls": map[string]any{}})
	}
	client.SetDiagnosticsHandlerVersioned(func(uri string, version int, hasVersion bool, diags []lsp.Diagnostic) {
		// Handler вызывается в своей горутине; канал буферизован, блокировка
		// здесь не тормозит редактор.
		m.lspEvents <- lspEvent{client: client, uri: uri, diags: diags, version: version, hasVersion: hasVersion}
	})
	client.SetApplyEditHandler(func(edit lsp.WorkspaceEdit) {
		// Reader не должен ждать UI: отдаём правку через буфер и отвечаем
		// серверу applied:true.
		select {
		case m.lspApply <- edit:
		default:
			go func() { m.lspApply <- edit }()
		}
	})

	t.lspGen++
	gen := t.lspGen
	version := t.ed.Buf.Version
	text := t.ed.Buf.Text()
	t.lsp = client
	t.lspURI = uri
	t.lspName = filepath.Base(cfg.Command)
	t.lspState = lspStarting
	t.lspSent = version
	t.diags = nil
	t.diagVersion = 0

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lspStartupWait)
		defer cancel()
		if err := client.Start(ctx); err != nil {
			client.Close()
			return lspStartedMsg{tab: t, client: client, gen: gen, version: version, err: err}
		}
		if err := client.Initialize(ctx); err != nil {
			client.Close()
			return lspStartedMsg{tab: t, client: client, gen: gen, version: version, err: err}
		}
		client.DidOpen(uri, lang, version, text)
		return lspStartedMsg{tab: t, client: client, gen: gen, version: version}
	}
}

// lspCommandFor - prose проверяется спелл-сервером, код - ServerFor.
func (m *Model) lspCommandFor(t *Tab) (command, languageID string) {
	path := t.ed.Buf.Path
	if path == "" {
		return "", ""
	}
	if m.isProsePath(path) {
		sample := firstKB(t.ed.Buf.Text(), 8<<10)
		switch m.cfg.SpellFor(path, sample) {
		case "ltex":
			return "ltex-ls", proseLanguageID(path)
		case "harper":
			return "harper-ls", proseLanguageID(path)
		default:
			return "", ""
		}
	}
	return m.cfg.ServerFor(path), codeLanguageID(path)
}

func firstKB(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func proseLanguageID(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown":
		return "markdown"
	case ".tex":
		return "latex"
	case ".rst":
		return "restructuredtext"
	case ".org":
		return "org"
	default:
		return "plaintext"
	}
}

func codeLanguageID(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py", ".pyi", ".pyw":
		return "python"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".rs":
		return "rust"
	case ".lua":
		return "lua"
	case ".nix":
		return "nix"
	case ".nim", ".nims":
		return "nim"
	case ".sh", ".bash", ".zsh":
		return "shellscript"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".yaml", ".yml":
		return "yaml"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".zig":
		return "zig"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	}
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."); ext != "" {
		return ext
	}
	return "plaintext"
}

func (m *Model) onLSPStarted(msg lspStartedMsg) tea.Cmd {
	t := msg.tab
	if !m.tabAlive(t) || t.lsp != msg.client || t.lspGen != msg.gen {
		go msg.client.Close()
		return nil
	}
	if msg.err != nil {
		t.lsp = nil
		t.lspURI = ""
		t.lspState = lspFailed
		m.notifyError("LSP off: " + t.lspName + ": " + m.lspErrorText(msg.err))
		return nil
	}
	t.lspState = lspReady
	if t.ed.Buf.Version != msg.version {
		// Пока сервер стартовал, в буфере уже другой текст.
		m.lspDidChange(t)
	}
	m.scheduleGhost()
	return nil
}

func (m *Model) onLSPEvent(ev lspEvent) tea.Cmd {
	if t := m.tabForClient(ev.client, ev.uri); t != nil {
		t.diags = ev.diags
		if ev.hasVersion {
			// Диагностики относятся к версии сервера. Если буфер уже ушёл
			// вперёд, visibleDiags их скроет.
			t.diagVersion = ev.version
		} else {
			t.diagVersion = t.ed.Buf.Version
		}
	}
	return waitLSP(m.lspEvents)
}

// lspDidChange сериализует полные снимки по версии: даже если горутины
// обгонят друг друга, на сервер последним уйдёт самый новый текст.
func (m *Model) lspDidChange(t *Tab) {
	if t == nil || t.lsp == nil || t.lspState != lspReady {
		return
	}
	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	text := t.ed.Buf.Text()
	go func() {
		t.lspSendMu.Lock()
		defer t.lspSendMu.Unlock()
		if version <= t.lspSent {
			return
		}
		t.lspSent = version
		client.DidChange(uri, version, text)
	}()
}

func (m *Model) lspDidSave(t *Tab) {
	if t == nil || t.lsp == nil || t.lspState != lspReady {
		return
	}
	client, uri := t.lsp, t.lspURI
	go func() {
		t.lspSendMu.Lock()
		defer t.lspSendMu.Unlock()
		client.DidSave(uri)
	}()
}

// detachLSP закрывает документ и клиент, не блокируя UI.
func (m *Model) detachLSP(t *Tab) {
	if t == nil {
		return
	}
	client, uri := t.lsp, t.lspURI
	t.lspGen++
	t.lsp = nil
	t.lspURI = ""
	t.lspName = ""
	t.lspState = lspNone
	t.diags = nil
	t.diagVersion = 0
	t.comp = completionState{}
	if client == nil {
		return
	}
	go func() {
		t.lspSendMu.Lock()
		defer t.lspSendMu.Unlock()
		if uri != "" {
			client.DidClose(uri)
		}
		client.Close()
	}()
}

func (m *Model) closeAllLSP() {
	type pair struct {
		client *lsp.Client
		uri    string
	}
	var clients []pair
	for _, t := range m.tabs {
		client, uri := t.lsp, t.lspURI
		t.lsp = nil
		t.lspURI = ""
		t.lspState = lspNone
		if client != nil {
			clients = append(clients, pair{client: client, uri: uri})
		}
	}
	if len(clients) == 0 {
		return
	}
	// Закрываем параллельно: иначе 20 вкладок = 20 × shutdown timeout.
	var wg sync.WaitGroup
	for _, p := range clients {
		wg.Add(1)
		go func(p pair) {
			defer wg.Done()
			p.client.DidClose(p.uri)
			p.client.Close()
		}(p)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		// Процессы всё равно добиваются Close при выходе из приложения.
	}
}

func (m *Model) restartLSP(t *Tab) tea.Cmd {
	if t == nil {
		return nil
	}
	m.detachLSP(t)
	t.lspState = lspNone
	return m.lspEnsure(t)
}

// failLSP переводит вкладку в LSP off после падения сервера. Редактор и
// буфер остаются рабочими.
func (m *Model) failLSP(t *Tab, err error) bool {
	if t == nil || err == nil {
		return false
	}
	dead := errors.Is(err, lsp.ErrClosed) || errors.Is(err, lsp.ErrNotStarted)
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{"server exited", "lsp: read", "broken pipe", "file already closed"} {
		if strings.Contains(msg, needle) {
			dead = true
			break
		}
	}
	if !dead {
		return false
	}
	t.lsp = nil
	t.lspURI = ""
	t.lspState = lspFailed
	t.diags = nil
	m.notifyError("LSP off: " + m.lspErrorText(err))
	return true
}

func (m *Model) lspStatusText(t *Tab) string {
	switch t.lspState {
	case lspStarting:
		return "LSP…"
	case lspReady:
		if t.lspName != "" {
			return t.lspName
		}
		return "LSP"
	case lspOff, lspFailed:
		return "LSP off"
	}
	return ""
}

func (m *Model) lspPosition(t *Tab) lsp.Position {
	p := t.ed.Cursor
	return lsp.Position{Line: p.Line, Character: t.ed.Buf.UTF16Offset(p)}
}

// ---- hover -------------------------------------------------------------

func (m *Model) hoverCmd() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if !m.lspReadyFor(t) {
		m.lspOffToast(t)
		return nil
	}
	if !t.lsp.Supports("hover") {
		m.notifyInfo(m.tr("LSP не поддерживает подсказки", "LSP does not support hover"))
		return nil
	}
	m.hoverGen++
	gen := m.hoverGen
	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	pos := m.lspPosition(t)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		h, err := client.Hover(ctx, uri, pos)
		return lspHoverMsg{tab: t, gen: gen, version: version, hover: h, err: err}
	}
}

func (m *Model) onHover(msg lspHoverMsg) tea.Cmd {
	if !m.tabAlive(msg.tab) || msg.tab != m.tab() || msg.gen != m.hoverGen || msg.tab.ed.Buf.Version != msg.version {
		return nil
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayHover {
		return nil
	}
	if msg.err != nil {
		if !m.failLSP(msg.tab, msg.err) {
			m.notifyError(m.tr("подсказка: ", "hover: ") + m.lspErrorText(msg.err))
		}
		return nil
	}
	if msg.hover == nil || strings.TrimSpace(msg.hover.Contents) == "" {
		m.notifyInfo(m.tr("нет подсказки", "no hover information"))
		return nil
	}
	m.closeOverlays()
	m.ov.kind = overlayHover
	m.ov.hover = msg.hover.Contents
	return nil
}

func (m *Model) hoverKey(msg tea.KeyPressMsg) tea.Cmd {
	m.closeOverlays()
	return nil
}

func (m *Model) hoverView() popupView {
	width := min(64, max(24, m.w-8))
	lines := markdownPopupLines(m.ov.hover, width)
	if len(lines) == 0 {
		return popupView{}
	}
	if len(lines) > 14 {
		lines = append(lines[:14], m.th.panelDim+"…"+sgrReset)
	}
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, m.th.panelOn+truncateTo(sanitizeOneLine(line), width)+sgrReset)
	}
	return m.popupAtCursor(m.tr("Подсказка", "Hover"), m.tr("Esc закрыть", "Esc close"), rows)
}

func markdownPopupLines(text string, width int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var out []string
	inCode := false
	for _, raw := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(raw), "```") || strings.HasPrefix(strings.TrimSpace(raw), "~~~") {
			inCode = !inCode
			continue
		}
		line := strings.TrimRight(raw, " \t")
		if !inCode {
			line = strings.TrimSpace(line)
			for strings.HasPrefix(line, "#") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			}
			for strings.HasPrefix(line, ">") {
				line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
			}
			line = strings.ReplaceAll(line, "**", "")
			line = strings.ReplaceAll(line, "__", "")
			line = strings.ReplaceAll(line, "`", "")
		}
		if line == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		if inCode {
			out = append(out, "  "+line)
			continue
		}
		out = append(out, wrapWords(line, width)...)
	}
	return out
}

// ---- definition / references ------------------------------------------

func (m *Model) locationsCmd(definition bool) tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if !m.lspReadyFor(t) {
		m.lspOffToast(t)
		return nil
	}
	feature := "references"
	if definition {
		feature = "definition"
	}
	if !t.lsp.Supports(feature) {
		if definition {
			m.notifyInfo(m.tr("LSP не поддерживает определения", "LSP does not support definition"))
		} else {
			m.notifyInfo(m.tr("LSP не поддерживает ссылки", "LSP does not support references"))
		}
		return nil
	}
	m.locsGen++
	gen := m.locsGen
	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	pos := m.lspPosition(t)
	title := m.tr("Ссылки", "References")
	if definition {
		title = m.tr("Определение", "Definition")
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var locs []lsp.Location
		var err error
		if definition {
			locs, err = client.Definition(ctx, uri, pos)
		} else {
			locs, err = client.References(ctx, uri, pos)
		}
		return lspLocsMsg{tab: t, gen: gen, version: version, direct: definition, title: title, locations: locs, err: err}
	}
}

func (m *Model) onLocs(msg lspLocsMsg) tea.Cmd {
	if !m.tabAlive(msg.tab) || msg.tab != m.tab() || msg.gen != m.locsGen || msg.tab.ed.Buf.Version != msg.version {
		return nil
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayLocs {
		return nil
	}
	if msg.err != nil {
		if !m.failLSP(msg.tab, msg.err) {
			m.notifyError(msg.title + ": " + m.lspErrorText(msg.err))
		}
		return nil
	}
	if len(msg.locations) == 0 {
		if msg.direct {
			m.notifyInfo(m.tr("определение не найдено", "definition not found"))
		} else {
			m.notifyInfo(m.tr("ссылок нет", "no references"))
		}
		return nil
	}
	if msg.direct && len(msg.locations) == 1 {
		return m.gotoLocation(msg.locations[0])
	}
	m.closeOverlays()
	m.ov.kind = overlayLocs
	m.ov.popTitle = msg.title
	m.ov.locs = msg.locations
	m.ov.sel = 0
	return nil
}

func (m *Model) gotoLocation(loc lsp.Location) tea.Cmd {
	path, err := lsp.URIToPath(loc.URI)
	if err != nil {
		m.notifyError(m.tr("не открыто: ", "not opened: ") + err.Error())
		return nil
	}
	m.closeOverlays()
	m.openFile(path)
	t := m.tab()
	if t == nil || t.ed.Buf.Path == "" || filepath.Clean(t.ed.Buf.Path) != filepath.Clean(path) {
		m.notifyError(m.tr("не открыто: ", "not opened: ") + path)
		return nil
	}
	p := t.ed.Buf.PosFromUTF16(loc.Range.Start.Line, loc.Range.Start.Character)
	t.ed.SetCursor(p)
	t.free = false
	m.clampScroll()
	m.scheduleGhost()
	return nil
}

func (m *Model) locationsKey(msg tea.KeyPressMsg) tea.Cmd {
	t := m.tab()
	switch msg.Keystroke() {
	case "esc":
		m.closeOverlays()
	case "up", "ctrl+p":
		m.locationsMove(-1)
	case "down", "ctrl+n":
		m.locationsMove(1)
	case "pgup":
		m.locationsMove(-8)
	case "pgdown":
		m.locationsMove(8)
	case "enter":
		if t != nil && m.ov.sel >= 0 && m.ov.sel < len(m.ov.locs) {
			return m.gotoLocation(m.ov.locs[m.ov.sel])
		}
	}
	return nil
}

func (m *Model) locationsMove(delta int) {
	n := len(m.ov.locs)
	if n == 0 {
		return
	}
	m.ov.sel = max(0, min(m.ov.sel+delta, n-1))
}

func (m *Model) locationsStart() int {
	height := m.popupHeight()
	n := len(m.ov.locs)
	if n == 0 {
		return 0
	}
	start := max(0, min(m.ov.sel-height+1, n-height))
	if m.ov.sel < height {
		start = 0
	}
	return start
}

func (m *Model) locationsView() popupView {
	if len(m.ov.locs) == 0 {
		return popupView{}
	}
	height := m.popupHeight()
	start := m.locationsStart()
	rows := make([]string, 0, height)
	for i := 0; i < height && start+i < len(m.ov.locs); i++ {
		loc := m.ov.locs[start+i]
		label := m.locationLabel(loc)
		line := fmt.Sprintf("%2d  %s", start+i+1, label)
		style := m.th.panelOn
		if start+i == m.ov.sel {
			style = m.th.panelSel
			line = "› " + strings.TrimPrefix(line, " ")
		}
		rows = append(rows, style+padANSI(truncateTo(sanitizeOneLine(line), min(72, m.w-6)), min(72, m.w-6), "")+sgrReset)
	}
	footer := m.trf("%d из %d · ↑↓ · Enter · Esc", "%d of %d · ↑↓ · Enter · Esc", m.ov.sel+1, len(m.ov.locs))
	return m.popupAtCursor(m.ov.popTitle, footer, rows)
}

func (m *Model) locationLabel(loc lsp.Location) string {
	path, err := lsp.URIToPath(loc.URI)
	if err != nil {
		return loc.URI
	}
	rel := path
	if m.root != "" {
		if r, err := filepath.Rel(m.root, path); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	return fmt.Sprintf("%s:%d:%d", rel, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
}

// ---- formatting --------------------------------------------------------

func (m *Model) formatCmd() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: форматирование запрещено", "read-only: formatting is disabled"))
		return nil
	}
	if !m.lspReadyFor(t) {
		m.lspOffToast(t)
		return nil
	}
	if !t.lsp.Supports("format") {
		m.notifyInfo(m.tr("LSP не поддерживает форматирование", "LSP does not support formatting"))
		return nil
	}
	m.fmtGen++
	gen := m.fmtGen
	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		edits, err := client.Format(ctx, uri)
		return lspFormatMsg{tab: t, gen: gen, version: version, edits: edits, err: err}
	}
}

func (m *Model) onFormat(msg lspFormatMsg) tea.Cmd {
	t := msg.tab
	if !m.tabAlive(t) || t != m.tab() || msg.gen != m.fmtGen || t.ed.Buf.Version != msg.version {
		return nil
	}
	if m.ov.kind != overlayNone {
		return nil
	}
	if msg.err != nil {
		if !m.failLSP(t, msg.err) {
			m.notifyError(m.tr("форматирование: ", "format: ") + m.lspErrorText(msg.err))
		}
		return nil
	}
	if len(msg.edits) == 0 {
		m.notifyInfo(m.tr("нечего форматировать", "nothing to format"))
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: форматирование запрещено", "read-only: formatting is disabled"))
		return nil
	}
	if n := applyTextEdits(t.ed, msg.edits); n == 0 {
		m.notifyInfo(m.tr("нечего форматировать", "nothing to format"))
		return nil
	}
	m.afterEditTab(t)
	m.clampScroll()
	m.notifySuccess(m.tr("отформатировано", "formatted"))
	return nil
}

// ---- text edits --------------------------------------------------------

type textEditOp struct {
	r    ed.Range
	text string
}

func lspRangeToEd(b *ed.Buffer, r lsp.Range) ed.Range {
	return ed.Range{
		Start: b.PosFromUTF16(r.Start.Line, r.Start.Character),
		End:   b.PosFromUTF16(r.End.Line, r.End.Character),
	}
}

func edRangeToLSP(b *ed.Buffer, r ed.Range) lsp.Range {
	start := b.Clamp(r.Start)
	end := b.Clamp(r.End)
	return lsp.Range{
		Start: lsp.Position{Line: start.Line, Character: b.UTF16Offset(start)},
		End:   lsp.Position{Line: end.Line, Character: b.UTF16Offset(end)},
	}
}

// textEditOps конвертирует правки и сортирует их с конца файла: применение
// по убыванию позиции не сдвигает ещё не применённые диапазоны.
func textEditOps(b *ed.Buffer, edits []lsp.TextEdit) []textEditOp {
	ops := make([]textEditOp, 0, len(edits))
	for _, e := range edits {
		ops = append(ops, textEditOp{r: lspRangeToEd(b, e.Range), text: e.NewText})
	}
	sort.SliceStable(ops, func(i, j int) bool {
		a, b := ops[i].r.Start, ops[j].r.Start
		if a.Line != b.Line {
			return a.Line > b.Line
		}
		return a.Col > b.Col
	})
	return ops
}

// applyTextEdits выполняет все правки одной транзакцией и возвращает число
// применённых. Вызывающий сам решает, когда уведомить LSP.
func applyTextEdits(e *ed.Editor, edits []lsp.TextEdit) int {
	if e == nil || e.Buf.ReadOnly || len(edits) == 0 {
		return 0
	}
	ops := textEditOps(e.Buf, edits)
	e.BeginGroup()
	for _, op := range ops {
		e.ReplaceRange(op.r, op.text)
	}
	e.EndGroup()
	return len(ops)
}

// ---- диагностика под курсором -----------------------------------------

func diagRangeInEd(t *Tab, d lsp.Diagnostic) ed.Range {
	return lspRangeToEd(t.ed.Buf, d.Range)
}

func diagsOnLine(t *Tab, line int) []lsp.Diagnostic {
	var out []lsp.Diagnostic
	for _, d := range t.visibleDiags() {
		if d.Range.Start.Line <= line && line <= d.Range.End.Line {
			out = append(out, d)
		}
	}
	return out
}

func (m *Model) diagAtCursor(t *Tab) *lsp.Diagnostic {
	if t == nil {
		return nil
	}
	diags := t.visibleDiags()
	if len(diags) == 0 {
		return nil
	}
	p := t.ed.Cursor
	var best *lsp.Diagnostic
	for i := range diags {
		d := &diags[i]
		r := diagRangeInEd(t, *d)
		inside := posLessEq(r.Start, p) && posLess(p, r.End)
		if r.Start == r.End && p == r.Start {
			inside = true
		}
		if !inside {
			continue
		}
		if best == nil || severityRank(d.Severity) < severityRank(best.Severity) {
			best = d
		}
	}
	return best
}

func severityRank(sev int) int {
	if sev < 1 || sev > 3 {
		return 3
	}
	return sev
}

// ---- общий popup --------------------------------------------------------

type popupView struct {
	shown     bool
	box       []string
	x, y      int
	itemTop   int
	itemCount int
	inner     int
}

func (m *Model) popupHeight() int {
	return max(3, min(8, m.h-6))
}

func (m *Model) cursorScreen() (int, int) {
	t := m.tab()
	if t == nil {
		return 0, 0
	}
	lay := m.layout()
	if lay.text.w <= 0 || lay.text.h <= 0 {
		return 0, 0
	}
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	row := rowForPos(rows, t.wrapCache.lineStart, t.ed.Cursor)
	y := lay.text.y + row - t.top
	y = max(lay.text.y, min(y, lay.text.y+lay.text.h-1))
	line := t.ed.Buf.Line(t.ed.Cursor.Line)
	x := screenColOf(line, t.ed.Cursor.Col, m.cfg.Editor.Tabsize)
	if t.wrap {
		x = rowScreenCol(t, rows, row, t.ed.Cursor.Col, m.cfg.Editor.Tabsize)
	} else {
		x -= t.left
	}
	x = max(lay.text.x, min(lay.text.x+x, lay.text.x+max(0, lay.text.w-1)))
	return x, y
}

func (m *Model) popupAtCursor(title, footer string, rows []string) popupView {
	if m.w < 40 || m.h < 10 || len(rows) == 0 {
		return popupView{}
	}
	box := widgets.RenderBox(widgets.Box{Title: title, Rows: rows, Footer: footer, Width: m.w}, m.th.widgetStyle())
	if len(box) == 0 {
		return popupView{}
	}
	bw := 0
	for _, line := range box {
		bw = max(bw, ansiWidth(line))
	}
	anchorX, anchorY := m.cursorScreen()
	x, y := widgets.PlaceBelow(anchorX, anchorY, bw, len(box), m.w, m.h)
	return popupView{
		shown: true, box: box, x: x, y: y,
		itemTop: y + 1, itemCount: len(rows), inner: max(0, bw-2),
	}
}
