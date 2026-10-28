# pero v1 — контракты пакетов и владение файлами

Это рабочий контракт для параллельной реализации. UX-документы — источник истины.
Пакеты ниже не импортируют друг друга не по слоям и не меняют чужие файлы.

## Общие правила

- Go 1.26, `gofmt`, тесты рядом с кодом.
- Зависимости уже в `go.mod`: bubbletea v2, bubbles v2, lipgloss v2, chroma v2,
  BurntSushi/toml, atotto/clipboard, uniseg. **Не менять `go.mod`/`go.sum`.**
- Позиции в редакторе — логические: строка + индекс руны (`ed.Pos`). В LSP — UTF-16
  (`Character`), конвертация только на границе пакета `lsp`.
- Всё, что меняет текст, идёт через `ed.Editor` (транзакции). UI не пишет в `Buffer.Lines`.
- Никаких горутин, меняющих UI-модель: только `tea.Msg`/`tea.Cmd`.
- Внешние процессы (LSP, opencode) не блокируют UI.
- Ошибки — внятные, на русском в UI-слое; пакеты возвращают `error`.

## Волна 1 — независимые пакеты

### A. `internal/ed` — ядро редактора (владелец: агент A)

Файлы: `buffer.go`, `editor.go`, `undo.go`, `selection.go`, `search.go`,
`markdown.go`, `*_test.go`.

Публично:

```go
type Pos struct{ Line, Col int }
type Range struct{ Start, End Pos }

type Buffer struct {
    Lines   [][]rune
    Path    string
    EOL     string // "\n" | "\r\n"
    BOM     bool
    Version int
    ReadOnly bool
}
func New(path string) *Buffer
func Open(path string) (*Buffer, error)          // + лимит 10 МБ, бинарник → ReadOnly
func OpenBytes(data []byte, path string) *Buffer
func (b *Buffer) Text() string
func (b *Buffer) FileBytes() []byte
func (b *Buffer) Save() error
func (b *Buffer) SaveAs(path string) error
func (b *Buffer) Dirty() bool
func (b *Buffer) Reload() error
func (b *Buffer) LastLine() int
func (b *Buffer) Clamp(p Pos) Pos
func (b *Buffer) Line(n int) []rune
func (b *Buffer) RuneCount(p Pos) int
func (b *Buffer) PosFromRuneOffset(off int) Pos
func (b *Buffer) RuneOffset(p Pos) int
func (b *Buffer) UTF16Offset(p Pos) int
func (b *Buffer) PosFromUTF16(line, char int) Pos
```

`Editor` — курсор, выделение, undo, правки:

```go
type Selection struct{ Anchor, Cursor Pos; Active bool } // полуинтервал [min,max)

type Editor struct {
    Buf    *Buffer
    Cursor Pos
    Sel    Selection
}
func NewEditor(b *Buffer) *Editor
func (e *Editor) Insert(text string)                    // заменяет выделение, одна транзакция
func (e *Editor) DeleteBackward() bool
func (e *Editor) DeleteForward() bool
func (e *Editor) ReplaceRange(r Range, text string)     // транзакция
func (e *Editor) SetCursor(p Pos)
func (e *Editor) MoveLeft(extend bool)
func (e *Editor) MoveRight(extend bool)
func (e *Editor) MoveWordLeft(extend bool)
func (e *Editor) MoveWordRight(extend bool)
func (e *Editor) MoveUp(extend bool)
func (e *Editor) MoveDown(extend bool)
func (e *Editor) MoveHome(extend bool)
func (e *Editor) MoveEnd(extend bool)
func (e *Editor) MoveDocStart(extend bool)
func (e *Editor) MoveDocEnd(extend bool)
func (e *Editor) MovePage(delta int, extend bool)       // delta в экранных строках/логических
func (e *Editor) SelectAll()
func (e *Editor) SelectWord()                           // двойной клик
func (e *Editor) SelectLine()                           // тройной клик
func (e *Editor) SetSelection(anchor, cursor Pos)
func (e *Editor) ClearSelection()
func (e *Editor) HasSelection() bool
func (e *Editor) SelectionRange() Range
func (e *Editor) SelectedText() string
func (e *Editor) Cut() string
func (e *Editor) Copy() string
func (e *Editor) Paste(text string)
func (e *Editor) Undo() bool
func (e *Editor) Redo() bool
func (e *Editor) CanUndo() bool
func (e *Editor) CanRedo() bool
func (e *Editor) BeginGroup()                            // явная группа (replace-all, LSP fix)
func (e *Editor) EndGroup()
func (e *Editor) ToggleCheckbox() bool                   // Alt-t, только markdown
func (e *Editor) ContinueList()                           // Enter: вернуть вставку
func (e *Editor) CollapseSpaces() bool                    // Alt-s
func (e *Editor) IndentSelection() bool                    // Tab
func (e *Editor) OutdentSelection() bool                   // Shift-Tab
func (e *Editor) FindAll(query string, foldCase bool) []Range
func (e *Editor) FindNext(query string, from Pos, foldCase bool) (Range, bool)
func (e *Editor) FindPrev(query string, from Pos, foldCase bool) (Range, bool)
```

Undo: обратные операции, группировка правок в пределах 300 мс; `BeginGroup/EndGroup`
обязательны для replace-all/quickfix. Undo не теряет курсор/выделение (восстанавливает
позицию до правки). `ReadOnly` блокирует все изменения.

Markdown: `ToggleCheckbox` циклит `- [ ]` ↔ `- [x]`, понимает отступ и голый `[ ] `,
не трогает код-блоки (по ограждению ``` в начале строки). `ContinueList` понимает
`- `, `* `, `+ `, `1. `; пустой пункт удаляет маркер. `CollapseSpaces` не трогает отступ.
`IndentSelection`/`OutdentSelection` работают по строкам выделения или текущей строке.

Тесты: unicode (кириллица/эмодзи), undo/redo с группировкой, replace-all одной
транзакцией, сохранение EOL/BOM, read-only, markdown-цикл, поиск по кругу.

### B. `internal/config` — TOML + state + темы (владелец: агент B)

Файлы: `config.go`, `theme.go`, `state.go`, `default.toml`, `*_test.go`.

Публично:

```go
type Config struct { Theme string; Editor Editor; Markdown Markdown; LSP LSP; Ignore Ignore; Keys map[string]string; Files map[string]string }
type Editor struct { Tabsize int; Tabstospaces bool; Softwrap string; Ghost bool; GhostDelayMS int; ScrollMargin int; RestoreSession bool }
type Markdown struct { Checkbox bool; ContinueList bool; Spell string }
type LSP struct { Servers map[string]string } // filetype/glob → command line
type Ignore struct { Dirs []string }

type TabState struct{ Path string; CursorLine, CursorCol int }
type State struct {
    Tabs []TabState
    Recent []string
    RecentCommands []string
    LastFile string
}
func Default() Config
func Load() (Config, error)          // нет файла → создать дефолт; битый → ошибка + дефолты
func LoadOrDefault() (Config, string) // cfg, warning
func (c *Config) Normalize()
func ConfigPath() (string, error)
func StatePath() (string, error)
func LoadState() (State, bool)
func SaveState(State) error
func (c Config) SoftwrapFor(path string) bool // auto по Prose/Code
func (c Config) SpellFor(path string, sample string) string // auto: ltex/harper/off
func (c Config) ServerFor(path string) string
```

Темы: `everforest`, `vesper`, `nord`, `plain`. Публично:

```go
type Theme struct {
    Name string
    BG, Panel, Border, Text, Muted, Accent, SelectionBG, CurrentLine, Gutter,
    StatusBG, StatusFG, TabActive, TabInactive, Error, Warning, Info, Ghost,
    SearchBG, SearchCurrent, DiagnosticError, DiagnosticWarning, DiagnosticInfo string
}
func ThemeByName(name string) Theme
func ThemeNames() []string
```

Все цвета — ANSI/hex-строки, пригодные для lipgloss. `Load` не падает на неизвестной
теме (plain + warning). `SaveState` атомарный (`tmp` + rename), права 0644,
каталоги 0755. `state.toml` читается даже при битом `config.toml`.

Тесты: дефолт создаётся, битый TOML, неизвестный ключ (предупреждение, не паника),
softwrap auto для .md/.go, EOL/state round-trip, темы.

### C. `internal/workspace` — проект, файлы, fuzzy, сессия (владелец: агент C)

Файлы: `project.go`, `fuzzy.go`, `names.go`, `session.go`, `*_test.go`.

Публично:

```go
type File struct{ Path, Rel string; Size int64; ModTime time.Time; IsDir bool }
func Root(start string) string                  // вверх до .git/.jj, иначе start (abs)
func Walk(ctx context.Context, root string, ignoreDirs []string, out chan<- File) // не блокирует, закрывает канал
func List(ctx context.Context, root string, ignoreDirs []string, limit int) []File
func IsBinary(data []byte) bool
func ShouldIgnore(name string, ignoreDirs []string) bool
func IsProse(path string) bool                  // md/txt/rst/org
func IsCode(path string) bool
func DisplayName(path string) string            // basename
func Disambiguate(paths []string) map[string]string // минимально различающийся путь
type Match struct{ File File; Score int; Positions []int }
func Filter(files []File, query string, limit int) []Match // регистронезависимо, подпоследовательность, префикс важнее
func LoadRecent(path string) ([]string, error)  // state-файл совместим с config.State
func SaveRecent(path string, recent []string) error
```

`Walk` не идёт по симлинкам-циклам, не читает содержимое файлов, не падает на правах.
`Filter` по умолчанию возвращает до `limit` (0 = 200), сначала совпадения по имени,
потом по пути. Кириллица/ё — по рунам. `List` ограничен 100k файлов/2 сек.

Тесты: ignore-каталоги, symlink-цикл, fuzzy на кириллице, disambiguate, бинарник,
`Walk` закрывает канал и отменяется по ctx.

### D. `internal/lsp` — JSON-RPC и LSP (владелец: агент D)

Файлы: `types.go`, `jsonrpc.go`, `client.go`, `manager.go`, `*_test.go`.

Публично:

```go
type Position struct{ Line, Character int } // UTF-16
type Range struct{ Start, End Position }
type Diagnostic struct{ Range Range; Severity int; Message, Source string; Code json.RawMessage; Data json.RawMessage }
type CompletionItem struct{ Label, Detail, Kind string; InsertText string; TextEdit *TextEdit; Documentation string }
type TextEdit struct{ Range Range; NewText string }
type Location struct{ URI string; Range Range }
type Hover struct{ Contents string; Range *Range }
type CodeAction struct{ Title, Kind string; Diagnostics []Diagnostic; Edit *WorkspaceEdit; Command *Command }
type Command struct{ Title, Command string; Arguments json.RawMessage }
type WorkspaceEdit struct{ Changes map[string][]TextEdit }

type ServerConfig struct{ Command string; Args []string; LanguageID string; Extensions []string; InitOptions map[string]any }
type Client struct{ ... }
func NewClient(cfg ServerConfig, root string) *Client
func NewClientWithStreams(cfg ServerConfig, root string, in io.Reader, out io.Writer) *Client // тесты
func (c *Client) Start(ctx context.Context) error
func (c *Client) Close() error
func (c *Client) Initialize(ctx context.Context) error
func (c *Client) DidOpen(uri, languageID string, version int, text string)
func (c *Client) DidChange(uri string, version int, text string)
func (c *Client) DidSave(uri string)
func (c *Client) DidClose(uri string)
func (c *Client) Diagnostics(uri string) []Diagnostic
func (c *Client) Completion(ctx context.Context, uri string, pos Position) ([]CompletionItem, error)
func (c *Client) Hover(ctx context.Context, uri string, pos Position) (*Hover, error)
func (c *Client) Definition(ctx context.Context, uri string, pos Position) ([]Location, error)
func (c *Client) References(ctx context.Context, uri string, pos Position) ([]Location, error)
func (c *Client) Format(ctx context.Context, uri string) ([]TextEdit, error)
func (c *Client) CodeActions(ctx context.Context, uri string, rng Range, diags []Diagnostic) ([]CodeAction, error)
func (c *Client) ResolveCodeAction(ctx context.Context, a CodeAction) (CodeAction, error)
func (c *Client) ApplyWorkspaceEdit(ctx context.Context, e WorkspaceEdit) error // отправляет workspace/applyEdit
func (c *Client) SetDiagnosticsHandler(func(uri string, diags []Diagnostic))
func (c *Client) SetApplyEditHandler(func(e WorkspaceEdit))

type Manager struct{ ... }
func NewManager(root string, servers map[string]string) *Manager
func (m *Manager) ClientFor(path string, languageID string) (*Client, error)
func (m *Manager) Close()
```

- Запросы по id, ожидание ответа с контекстом; уведомления не блокируют.
- Диагностики кэшируются по URI + версии, не теряются при `publishDiagnostics`.
- `initialize`/`initialized`, `textDocument/didOpen|didChange|didSave|didClose`,
  full-sync (`TextDocumentSyncKind=Full`) — проще и надёжнее.
- `workspace/applyEdit` от сервера → handler, не блокирует reader.
- Падение сервера → `Close` и ошибка, редактор живёт.
- `ServerConfig` из строки команды: `strings.Fields`, поддержка флагов. `Manager`
  подбирает по расширению/языку; `gopls`, `pyright-langserver --stdio`,
  `typescript-language-server --stdio`, `clangd`, `lua-language-server`,
  `nixd`, `nimony-lsp`, `ltex-ls`, `harper-ls`.
- `uri` — `file://` с корректным percent-encoding (кириллица/пробелы).
- Hover: Markdown/plaintext contents в строку. Completion: `insertText`/`textEdit`.
- Таймауты: Start/Initialize 10 с; запросы — контекст вызывающего.

Тесты: фейковый сервер на `io.Pipe` (Content-Length кадры), id-мультиплекс,
диагностики, completion/hover/codeaction, битый JSON, EOF, URI с кириллицей.

### E. `internal/assist` — t9, ghost, AI (владелец: агент E)

Файлы: `t9.go`, `ghost.go`, `ai.go`, `*_test.go`.

```go
type WordIndex struct{ ... }
func NewWordIndex() *WordIndex
func (w *WordIndex) Index(text string)            // пересобрать (лениво, по версии)
func (w *WordIndex) Candidates(prefix string, limit int) []string // частота, затем алфавит
type Suggestion struct{ Text string; Source string; Version int }
func GhostCandidate(prefix string, candidates []string, minLen int) (Suggestion, bool)
func IsWordRune(r rune) bool
func WordPrefix(line []rune, col int) string
func RunOpencode(ctx context.Context, prompt string) (string, error) // opencode run --stdin? см. ниже
```

- t9: слова из буфера, длина ≥ 3, без цифр/пунктуации; частота; кириллица.
- ghost: только если курсор в конце слова/строки, кандидат длиннее префикса,
  не равен уже набранному, ≥ 2 символов. Источник один.
- AI: `opencode` запускается через `exec.CommandContext`, промпт на stdin (флаг
  `run`), stderr отдельно, таймаут 30 с, результат обрезается до 8 КБ.
  Если `opencode` нет — понятная ошибка. **Ничего не пишет в буфер сам.**

Тесты: t9-частота/кириллица, ghost-фильтры, AI с фейковым `opencode` в PATH.

### F. `internal/highlight` — chroma (владелец: агент F)

Файлы: `highlight.go`, `*_test.go`.

```go
type Span struct{ Start, End int; SGR string } // руны, полуинтервал
type Highlighter struct{ ... }
func New() *Highlighter
func (h *Highlighter) SetFile(path string)      // выбор лексера по расширению/имени
func (h *Highlighter) SetTheme(name string)     // everforest/vesper/nord/plain → chroma style
func (h *Highlighter) Lines(text string) [][]Span // по логическим строкам; кэш по тексту
```

- Поддержка минимум: markdown, go, lua, nix, json, toml, yaml, sh, python, js, ts,
  rust, c, nim. Неизвестный файл → пустые спаны.
- `SGR` — готовые ANSI-последовательности (например `\x1b[38;2;...m`), чтобы UI не
  зависел от chroma. Сброс — `\x1b[0m`; UI сам добавляет reset после спана.
- Кэш: один входной текст/версия; повторный вызов без изменений — те же спаны.
- Не паниковать на битом UTF-8/пустом файле.

Тесты: go/md/json дают непустые спаны, неизвестный файл пуст, повторный вызов из кэша,
эмодзи/кириллица не сдвигают индексы.

### G. `internal/widgets` — UI-примитивы (владелец: агент G)

Файлы: `rect.go`, `box.go`, `overlay.go`, `input.go`, `list.go`, `toast.go`, `*_test.go`.

```go
type Rect struct{ X, Y, W, H int }
func (r Rect) Contains(x, y int) bool
func (r Rect) Empty() bool

type Box struct{ Title, Footer string; Rows []string; Width int }
func RenderBox(b Box, theme Style) []string    // рамка ╭ ╮ ╰ ╯, обрезка по ANSI
func Overlay(base []string, box []string, x, y, width int) []string // ANSI-безопасно
func Center(w, h, boxW, boxH int) (x, y int)
func PlaceBelow(anchorX, anchorY, boxW, boxH, termW, termH int) (x, y int)

type Style struct{ BG, Border, Text, Muted, SelectedBG, SelectedFG, Error string }

type Input struct{ Text []rune; Cursor int; Placeholder string; SelAll bool }
func (i *Input) Set(string); Value() string
func (i *Input) Insert(string); Backspace(); Delete(); MoveLeft(); MoveRight()
func (i *Input) Home(); End(); WordLeft(); WordRight(); Clear()
func (i *Input) Render(width int, style Style) string // горизонтальный скролл, курсор не рисуем

type List struct{ Items []string; Sel, Scroll, Height, Width int }
func (l *List) Move(delta int); Page(delta int); Home(); End()
func (l *List) Visible() []string
type Toast struct{ Text string; Kind string; Until time.Time }
func RenderToast(t Toast, width int, style Style) string
```

- Ширина/обрезка — через `github.com/charmbracelet/x/ansi` (уже в зависимостях,
  `ansi.StringWidth`, `ansi.Truncate`, `ansi.Cut`), не по `len`.
- `Overlay` не выходит за границы base; если не помещается — прижимает.
- Frame: внутренняя ширина = max(24, ширина содержимого), не шире `width-4`.
- Input не принимает `\n` (схлопывает в пробел), `Tab` не вставляет.
- List не рисует больше `Height`, выбранный всегда виден.

Тесты: эмодзи/кириллица, overlay у краёв, ввод/backspace, список 1000 строк, toast.

### H. `internal/markdown` — чистые markdown-функции (владелец: агент H)

Файлы: `markdown.go`, `*_test.go`.

```go
func IsProse(path string) bool
func IsCheckbox(line string) bool
func ToggleCheckbox(line string) (string, bool) // - [ ] ↔ - [x], голый [ ] , отступ
func ListPrefix(line string) (prefix string, ok bool) // "- ", "* ", "1. "
func ContinueList(line string) (insert string, clear bool) // Enter
func CollapseSpaces(line string) string // не трогает ведущий отступ
func FenceState(lines [][]rune) []bool // true = строка внутри ``` / ~~~
func Sentence(text string) bool // эвристика prose для spell auto
func CyrillicShare(sample string) float64
```

- `ToggleCheckbox` на не-строке-списке добавляет `- [ ] ` после отступа.
- Внутри code fence не трогает.
- `ContinueList` на пустом пункте (`- `) → `clear=true`, вставка пустая.
- `CollapseSpaces` сохраняет отступ, табы не конвертирует.
- `CyrillicShare` — доля кириллических букв среди букв (0..1), для ltex/harper.

Тесты: все циклы чекбокса, вложенность, code fence, нумерованные списки, ё.

## Волна 2 — UI и интеграция (после волны 1)

Владелец UI-ядра получает все пакеты; файлы UI делятся по зонам:
`model.go/update.go/commands.go`, `layout.go/render.go/softwrap.go`,
`overlays.go/palette.go/open.go/find.go/help.go`,
`lsp_ui.go/completion.go/quickfix.go`, `mouse.go/menu.go`, `tabs.go/status.go`.
Общий контракт `Model` пишется до запуска агентов волны 2 и не меняется без
согласования.

## Волна 3 — QA

`go build ./...`, `go test ./...`, `go vet ./...`, прогон сценариев из критериев
готовности UX-документов (40×10, 80×24, 160×50), исправление интеграции.
