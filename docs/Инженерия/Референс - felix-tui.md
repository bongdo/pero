---
type: reference
status: active
updated: 2026-10-26
---

# Референс - felix-tui

## Где лежит

| | |
|---|---|
| Источник | `riga:/home/bongdo/Projects/felix-tui` (bongdo на 31.59.104.88) |
| Локальная копия | `~/Projects/felix-tui` |
| Версия | jj `254698d91bc8`, 2026-10-26 |
| Стек | bubbletea v2.1.0, bubbles v2.2.1, lipgloss v2.0.6 |
| Объём | ~8.4k строк Go, 304 КБ без бинарника |

Копия - **снимок для чтения**, не для правок. Правки - на riga, потом обновляем копию:
`rsync -a --exclude .git --exclude .jj --exclude felix-tui -e ssh riga:/home/bongdo/Projects/felix-tui/ ~/Projects/felix-tui/`.

Лицензии в репозитории нет; проект твой, поэтому код переносить можно, но чужие куски
(иконки, темы) проверять отдельно.

## Что читать первым

| Файл | Зачем |
|---|---|
| `internal/ui/layout.go` | геометрия, `rect`, брейкпоинты |
| `internal/ui/dialog.go:551` | `renderOverlays` / `overlayAt` - композитор попапов |
| `internal/ui/keys.go:28` | порядок обработки клавиш и фокусов |
| `internal/ui/mouse.go` | центральный роутер мыши |
| `internal/ui/render.go` | рендер экрана в `[]string` |
| `internal/ui/theme.go` | быстрая ANSI-покраска |
| `internal/ui/complete.go` | dropdown с фильтром и циклом |
| `internal/config/config.go` | TOML + state |
| `internal/ui/model_test.go` | как тестировать экраны |

## Что берём

- Композицию оверлеев (Popup, меню, help).
- Детерминированную геометрию и хит-тест `rect.contains`.
- Порядок фокуса и центральный роутер мыши.
- Тёмную тему/палитру и быструю покраску.
- Конфиг TOML + state-файл, `restore_state`.

## Что не берём

- Файловый браузер как основной экран.
- 4 режима отображения, ASCII-иконки, chafa-превью.
- Открытие файлов через `xdg-open`.

## Связи

- [[Архитектура]] - как это ложится в pero.
- [[Стек и запуск]].
