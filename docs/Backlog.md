---
type: backlog
status: done
updated: 2026-10-27
---

# Backlog

20 обязательных фич v1 из плана плюс командная строка Ctrl-E. `[x]` - есть в коде.
Ссылки ведут на экраны/примитивы.

## Ядро
- [x] 1. CLI: `pero file`, `pero dir`, `pero -`, новый буфер
- [x] 2. Каркас: открыть, скролл, выйти (M0)
- [x] 3. Буфер и курсор: UTF-8, слова, Home/End, PgUp/PgDn → [[Примитив - Курсор и выделение]]
- [x] 4. Правка: вставка, Backspace/Delete, Enter с автоотступом, Tab/Shift-Tab
- [x] 5. Undo/redo с группировкой, транзакции
- [x] 6. Выделение мышью/клавиатурой, copy/cut/paste, OSC52

## Файлы и вид
- [x] 7. Сохранение и dirty; атомарная запись; внешние изменения → [[Экран - Редактор]]
- [x] 8. Поиск и замена → [[Экран - Поиск и замена]]
- [x] 9. Fuzzy open → [[Экран - Открытие файла]]
- [x] 10. Подсветка синтаксиса - chroma (markdown/go/lua/nix/json/toml/sh/nim)
- [x] 11. Гаттер, статус-бар, счётчик ошибок
- [x] 12. Мышь: клик/драг/колесо/меню → [[Мышь]]

## LSP
- [x] 13. Клиент LSP: запуск по filetype, hover/definition/references/format
- [x] 14. Диагностики: подчёркивание, F8 → [[Примитив - Диагностика и ghost]]
- [x] 15. Quickfix `Alt-.` → [[Экран - Quickfix]]
- [x] 16. Completion `Ctrl-Space` → [[Экран - Completion]]
- [x] 17. Ghost text → [[Экран - Ghost text]]

## Текст и удобства
- [x] 18. Markdown: чекбоксы, списки, `Alt-s` → [[Примитив - Markdown-чекбокс и списки]]
- [x] 19. Конфиг TOML, темы, биндинги, сессия, авто-спеллчек → [[Экран - Настройки]]
- [x] 20. AI-дополнение через `Alt-a`/попап; автоматические ghost/completion
- [x] 21. Командная строка `Ctrl-E` - реестр команд, фильтр, режим пути

## Темы, язык, забота
- [x] 33 палитры, список с live-превью, сохранение в `config.toml`
- [x] EN/RU UI, по умолчанию English; `config.reload`
- [x] Ctrl-O вместо Alt-O
- [x] autopairs, обёртка выделения, smart Backspace/Enter/Home, подсветка скобок
- [x] trim/final newline, счётчик выделения, причина read-only, `› N` в вкладках
- [x] LSP capabilities, retry на Internal error, completion fallback

## M10 - ссылки и переходы (план)
- [ ] Ctrl+клик по идентификатору → LSP definition (с клавиатурным `Alt-d`)
- [ ] Ctrl+клик по `http(s)://` → браузер, только безопасные схемы
- [ ] Ctrl+клик по `[[Note]]` / `[[Note#Heading]]` → Obsidian
- [ ] Ctrl+клик по `# Заголовок` внутри vault → заметка на заголовке
- [ ] Ctrl+клик по `[текст](url)` → браузер/файл, hit-test с softwrap
- [ ] `[links]` в конфиге, vault auto-detect, безопасность и тесты

Подробный план: [[M10 - ссылки и переходы]].

## Ближайшие шаги

1. M10: `links.go` + Ctrl+клик по LSP definition, затем URL и Obsidian.
2. Живые проверки: zellij-мышь, ssh/OSC52, gopls/pyright/clangd.
3. Производительность на 100k строк: первый softwrap и поиск в фоне.

Подробные задачи - в `TODO.md` проекта; эта база описывает «что», а не «как».
