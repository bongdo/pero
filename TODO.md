# TODO

План и архитектура - в [docs/00 MOC.md](docs/00%20MOC.md). M1–M9 реализованы;
M10 (ссылки и Ctrl+клик) - в плане, дальше живые проверки.

## M1 - буфер и правка
- [x] `internal/ed`: строки `[]rune`, позиция (line, col), insert/delete, EOL/BOM
- [x] курсор: стрелки, Home/End, Ctrl-стрелки по словам, PgUp/PgDn
- [x] вставка UTF-8, Backspace/Delete, Enter с автоотступом, Tab
- [x] Ctrl-S сохранить (путь из CLI), dirty в статусе
- [x] Ctrl-Q с подтверждением при грязном буфере
- [x] командная строка `Ctrl-E`: фильтр команд, режим пути (`Ctrl-P`)
- [x] Shift-Tab и отступ по выделению
- [x] Tab в файлах с табами: `tabsize`/`tabstospaces` из конфига

## M2 - выделение, undo, поиск
- [x] undo/redo с группировкой 300 мс и транзакциями
- [x] выделение мышью и клавиатурой, двойной/тройной клик
- [x] copy/cut/paste: OSC52 + `atotto/clipboard`, bracketed paste
- [x] Ctrl-F поиск с подсветкой видимого, Enter/Shift-Enter
- [x] Ctrl-H замена (без regex)
- [x] «Сохранить как» (Ctrl-Shift-S) и имя файла у безымянного буфера

## M3 - вкладки и файлы
- [x] полоса вкладок: активная, dirty `●`, 20+ вкладок со скроллом
- [x] Ctrl-O/Alt-Q/Alt-J/K как команды `tab.*`
- [x] Ctrl-P fuzzy open: файлы папки + недавние
- [x] state-файл: восстановить вкладки и позиции
- [x] файл, изменённый извне

## M4 - вид и конфиг
- [x] chroma для markdown/go/lua/nix/json/toml/sh/nim
- [x] номера строк (гаттер), текущая строка, filetype в статусе
- [x] TOML-конфиг, темы everforest/vesper/nord/plain
- [x] биндинги из конфига: клавиша → id команды
- [x] softwrap и `view.wrap` в командной строке

## M5 - оверлеи и мышь
- [x] рендер в `[]string`, `overlayAt` (как в felix `dialog.go:551`)
- [x] rect/contains, центральный роутер мыши
- [x] popup-примитив: рамка, скролл, якорь
- [x] контекстное меню по правой кнопке

## M6 - LSP
- [x] JSON-RPC клиент, запуск по filetype
- [x] didOpen/didChange, диагностики → подчёркивание + счётчик
- [x] F8/Shift-F8
- [x] hover/definition/references/format
- [x] ltex (ru) и harper (en) не блокируют UI

## M7 - подсказки
- [x] completion popup (LSP + слова из буфера)
- [x] quickfix popup (Alt-., WorkspaceEdit одной транзакцией)
- [x] ghost text: debounce 250 мс, Tab принять, Esc погасить
- [x] t9: слова из буфера, Tab листает
- [x] команды `view.ghost`, `lsp.*` в палитре

## M8 - AI
- [x] Alt-a: выделение → `opencode run` → popup → Tab вставить
- [x] индикатор «думает», отмена

## M9 - темы, язык, забота
- [x] 33 темы (catppuccin/gruvbox/tokyonight/…) с синтаксической палитрой
- [x] список тем с live-превью; выбор сохраняется в `config.toml`
- [x] `language = "en" | "ru"`, по умолчанию English; `config.reload`
- [x] Ctrl-O вместо Alt-O для новой вкладки
- [x] autopairs, обёртка выделения, smart Backspace/Enter/Home, подсветка скобок
- [x] trim/final newline при сохранении (конфиг), счётчик выделения, read-only reason
- [x] LSP capabilities, retry на Internal error, completion fallback и понятные тосты

## M10 - ссылки и переходы (план, не начато)
- [ ] Ctrl+клик по переменной/функции → LSP definition в позиции клика
- [ ] Ctrl+клик по `http(s)://` → браузер (`xdg-open`, только http/https)
- [ ] Ctrl+клик по `[[Note]]` / `[[Note#Heading]]` → Obsidian
- [ ] Ctrl+клик по `# Заголовок` в vault → заметка на заголовке
- [ ] Ctrl+клик по `[текст](url)` → браузер / локальный файл
- [ ] подсказка при Ctrl+hover, `link.open` для клавиатуры
- [ ] конфиг `[links]`, vault auto-detect, безопасность схем и путей
- [ ] тесты ссылок, hit-test softwrap, fake `xdg-open`/Obsidian

Подробно: [docs/plans/M10 - ссылки и переходы.md](docs/plans/M10%20-%20ссылки%20и%20переходы.md).

## Что осталось проверить вживую

- Мышь в zellij (возможное смещение координат).
- Реальные gopls/pyright/clangd/nixd/nimony-lsp (в системе есть только ltex-ls/harper-ls).
- OSC52 в ssh-сессии.
- Очень большие (100k строк) файлы: первый расчёт softwrap ~0.7 с, дальше
  инкрементально; поиск пересчитывает всё совпадение на каждый ввод.
- `Alt-a` - явный вызов AI; ADR #9 по-прежнему про автоматические подсказки.
