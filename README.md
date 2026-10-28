# pero

Терминальный редактор для себя: немодальный, вкладки, мышь, ghost-подсказки, LSP.
Go + [Bubble Tea v2](https://github.com/charmbracelet/bubbletea).

Почему свой, а не micro: в micro нет слоя виджетов — только буфер, однострочный инфобар
и сплиты. Ghost-текст и popup-меню там невозможны без патча на Go.

## Документация

Вся база знаний — Obsidian-vault в [`docs/`](docs/00%20MOC.md):

- [`docs/00 MOC.md`](docs/00%20MOC.md) — точка входа.
- `docs/UX/` — экраны, примитивы, клавиши, мышь (это главное).
- `docs/Домен/` — язык, Prose/Code, контексты.
- `docs/Инженерия/` — архитектура, стек, референс felix-tui, решения.

Порядок чтения: UX → домен → инженерия.

## Запуск

```sh
go run .                 # пустой буфер
go run . ~/todo.md       # файл
go run . .               # папка = проект
go build -o pero .
go test ./...
```

## Статус

M1–M8 реализованы: вкладки, undo/redo, выделение и мышь, поиск/замена, fuzzy open,
сессия, chroma, TOML-конфиг и 33 темы, softwrap, контекстное меню, LSP (диагностики,
hover/definition/references/format, quickfix, completion), ghost/t9 и AI через
`Alt-a`/`opencode`. Интерфейс двуязычный (`language = "en" | "ru"`, по умолчанию
английский), есть autopairs и обёртка выделения скобками. Живые проверки, которые
остались, — в `TODO.md`.

## Конфиг

- `~/.config/pero/config.toml` — тема (33 палитры), язык EN/RU, редактор, autopairs,
  markdown, LSP, ignore, биндинги.
- `~/.local/state/pero/state.toml` — вкладки, позиции, недавние файлы и команды.
- Смена темы в UI (`Ctrl-E` → `Theme…`) сразу сохраняется в `config.toml`.

## Референс

Архитектурные приёмы (оверлеи, геометрия, мышь, темы) взяты из
[`felix-tui`](docs/Инженерия/Референс%20—%20felix-tui.md) — локальная копия в
`~/Projects/felix-tui`, источник на `riga`.
