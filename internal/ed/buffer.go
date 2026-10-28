package ed

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ErrNoPath - буфер не привязан к файлу.
var ErrNoPath = errors.New("у буфера нет пути")

// ErrReadOnly - буфер нельзя записывать.
var ErrReadOnly = errors.New("буфер только для чтения")

// MaxFileSize - предел чтения файла: всё, что больше, открывается read-only.
const MaxFileSize = 10 << 20

// binarySample - сколько байт с начала файла проверяем на NUL.
const binarySample = 8 << 10

// Pos - логическая позиция: строка и индекс руны (не байт).
type Pos struct {
	Line, Col int
}

// Range - полуинтервал [Start, End).
type Range struct {
	Start, End Pos
}

// Buffer - текст одного файла.
type Buffer struct {
	Lines    [][]rune
	Path     string
	EOL      string // "\n" или "\r\n"
	BOM      bool
	Version  int
	ReadOnly bool

	dirty bool

	// revision - идентификатор текущего содержимого для dirty после undo/redo:
	// undo возвращает ровно то состояние, что было до правки. saveGen меняется
	// при сохранении/перезагрузке и не даёт группировать правки через save.
	revision      uint64
	nextRevision  uint64
	savedRevision uint64
	saveGen       int
}

// New создаёт пустой буфер.
func New(path string) *Buffer {
	return &Buffer{Lines: [][]rune{{}}, Path: path, EOL: "\n", nextRevision: 1}
}

// Open читает файл, запоминает EOL и BOM.
// Файлы больше MaxFileSize читаются не целиком и помечаются ReadOnly.
// Бинарные файлы (NUL в первых 8 КБ) тоже ReadOnly.
func Open(path string) (*Buffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize))
	if err != nil {
		return nil, err
	}
	b := OpenBytes(data, path)
	if info.Size() > MaxFileSize {
		b.ReadOnly = true
	}
	return b, nil
}

// OpenBytes разбирает данные так же, как Open.
func OpenBytes(data []byte, path string) *Buffer {
	b := &Buffer{Path: path, EOL: "\n", nextRevision: 1}
	if len(data) > MaxFileSize || bytes.IndexByte(sample(data), 0) >= 0 {
		b.ReadOnly = true
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		b.BOM = true
		data = data[3:]
	}
	if bytes.Contains(data, []byte("\r\n")) {
		b.EOL = "\r\n"
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	for _, line := range strings.Split(string(data), "\n") {
		b.Lines = append(b.Lines, []rune(line))
	}
	if len(b.Lines) == 0 {
		b.Lines = [][]rune{{}}
	}
	return b
}

func sample(data []byte) []byte {
	if len(data) > binarySample {
		return data[:binarySample]
	}
	return data
}

// Text - содержимое с LF, без BOM (для поиска и тестов).
func (b *Buffer) Text() string {
	var sb strings.Builder
	for i, line := range b.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(line))
	}
	return sb.String()
}

// FileBytes - ровно то, что уйдёт на диск: BOM + исходный EOL.
func (b *Buffer) FileBytes() []byte {
	var sb strings.Builder
	if b.BOM {
		sb.Write([]byte{0xEF, 0xBB, 0xBF})
	}
	for i, line := range b.Lines {
		if i > 0 {
			sb.WriteString(b.EOL)
		}
		sb.WriteString(string(line))
	}
	return []byte(sb.String())
}

// Save пишет буфер на диск и снимает dirty.
func (b *Buffer) Save() error {
	if b.Path == "" {
		return ErrNoPath
	}
	return b.SaveAs(b.Path)
}

// SaveAs пишет буфер в новый файл, делает его текущим и снимает dirty.
func (b *Buffer) SaveAs(path string) error {
	if path == "" {
		return ErrNoPath
	}
	if b.ReadOnly {
		return ErrReadOnly
	}
	if err := writeFileAtomic(path, b.FileBytes()); err != nil {
		return err
	}
	b.Path = path
	b.dirty = false
	b.savedRevision = b.revision
	b.saveGen++
	return nil
}

// writeFileAtomic не обрезает существующий файл до того, как новый полностью
// записан: падение/ENOSPC не должны оставить пустой файл.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if name != "" {
			_ = os.Remove(name)
		}
	}()
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	name = ""
	return nil
}

// Reload перечитывает файл с диска.
func (b *Buffer) Reload() error {
	if b.Path == "" {
		return ErrNoPath
	}
	nb, err := Open(b.Path)
	if err != nil {
		return err
	}
	b.Lines = nb.Lines
	b.EOL = nb.EOL
	b.BOM = nb.BOM
	b.ReadOnly = nb.ReadOnly
	b.Version = nb.Version
	// Новое содержимое - новое состояние: старые undo-записи к нему не относятся.
	b.revision = b.nextRevision
	b.nextRevision++
	b.savedRevision = b.revision
	b.saveGen++
	b.dirty = false
	return nil
}

// Dirty - были ли правки после открытия/сохранения.
func (b *Buffer) Dirty() bool { return b.dirty }

// LastLine - индекс последней строки.
func (b *Buffer) LastLine() int { return len(b.Lines) - 1 }

// Clamp приводит позицию к допустимой.
func (b *Buffer) Clamp(p Pos) Pos { return b.clamp(p) }

func (b *Buffer) clamp(p Pos) Pos {
	if len(b.Lines) == 0 {
		return Pos{}
	}
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Line > len(b.Lines)-1 {
		p.Line = len(b.Lines) - 1
	}
	if p.Col < 0 {
		p.Col = 0
	}
	if p.Col > len(b.Lines[p.Line]) {
		p.Col = len(b.Lines[p.Line])
	}
	return p
}

// Line возвращает строку n (nil за границей буфера).
func (b *Buffer) Line(n int) []rune {
	if n < 0 || n >= len(b.Lines) {
		return nil
	}
	return b.Lines[n]
}

// RuneCount - количество рун в строке, на которую указывает p.
func (b *Buffer) RuneCount(p Pos) int {
	return len(b.Lines[b.clamp(p).Line])
}

// RuneOffset - абсолютное смещение в рунах от начала буфера (LF = одна руна).
func (b *Buffer) RuneOffset(p Pos) int {
	p = b.clamp(p)
	off := p.Col
	for i := 0; i < p.Line; i++ {
		off += len(b.Lines[i]) + 1
	}
	return off
}

// PosFromRuneOffset - обратное к RuneOffset.
func (b *Buffer) PosFromRuneOffset(off int) Pos {
	if off <= 0 {
		return Pos{}
	}
	for i, line := range b.Lines {
		if off <= len(line) {
			return Pos{i, off}
		}
		off -= len(line) + 1
	}
	last := len(b.Lines) - 1
	return Pos{last, len(b.Lines[last])}
}

// UTF16Offset - позиция в UTF-16 кодельных единицах от начала строки (LSP Character).
func (b *Buffer) UTF16Offset(p Pos) int {
	p = b.clamp(p)
	n := 0
	for _, r := range b.Lines[p.Line][:p.Col] {
		n += utf16Len(r)
	}
	return n
}

// PosFromUTF16 - позиция по номеру строки и UTF-16 колонке.
// Внутри суррогатной пары позиция не создаётся: возвращается начало руны.
func (b *Buffer) PosFromUTF16(line, char int) Pos {
	if line < 0 {
		line = 0
	}
	if line > b.LastLine() {
		line = b.LastLine()
	}
	if char <= 0 {
		return Pos{line, 0}
	}
	n := 0
	for i, r := range b.Lines[line] {
		w := utf16Len(r)
		if n+w > char {
			return Pos{line, i}
		}
		n += w
		if n == char {
			return Pos{line, i + 1}
		}
	}
	return Pos{line, len(b.Lines[line])}
}

func utf16Len(r rune) int {
	if r > 0xFFFF {
		return 2
	}
	return 1
}

// Insert вставляет текст (может быть многострочным), возвращает позицию после вставки.
func (b *Buffer) Insert(at Pos, text string) Pos {
	at = b.clamp(at)
	return b.replace(Range{Start: at, End: at}, text)
}

// DeleteBackward удаляет графемный кластер перед позицией, склеивая строки на границе.
func (b *Buffer) DeleteBackward(p Pos) Pos {
	p = b.clamp(p)
	if b.ReadOnly {
		return p
	}
	prev, ok := b.prevPos(p)
	if !ok {
		return p
	}
	b.replace(Range{Start: prev, End: p}, "")
	return prev
}

// DeleteForward удаляет графемный кластер на позиции, склеивая строки на границе.
func (b *Buffer) DeleteForward(p Pos) Pos {
	p = b.clamp(p)
	if b.ReadOnly {
		return p
	}
	next, ok := b.nextPos(p)
	if !ok {
		return p
	}
	b.replace(Range{Start: p, End: next}, "")
	return p
}

// replace - единственный примитив правки: держит Version и dirty в синхроне.
// Не экспортируется: правки идут через Editor.
func (b *Buffer) replace(r Range, text string) Pos {
	r = b.orderedRange(r)
	if b.ReadOnly {
		return r.Start
	}
	if text == "" && r.Start == r.End {
		return r.Start
	}
	chunks := strings.Split(text, "\n")
	head := append([]rune{}, b.Lines[r.Start.Line][:r.Start.Col]...)
	tail := append([]rune{}, b.Lines[r.End.Line][r.End.Col:]...)

	if len(chunks) == 1 && r.Start.Line == r.End.Line {
		merged := append(head, []rune(chunks[0])...)
		merged = append(merged, tail...)
		b.Lines[r.Start.Line] = merged
		b.dirty = true
		b.Version++
		b.bumpRevision()
		return b.posAfter(r.Start, text)
	}

	repl := make([][]rune, 0, len(chunks))
	for i, chunk := range chunks {
		line := []rune(chunk)
		if i == 0 {
			line = append(append([]rune{}, head...), line...)
		}
		if i == len(chunks)-1 {
			line = append(line, tail...)
		}
		repl = append(repl, line)
	}

	out := make([][]rune, 0, len(b.Lines)-(r.End.Line-r.Start.Line)+len(repl))
	out = append(out, b.Lines[:r.Start.Line]...)
	out = append(out, repl...)
	out = append(out, b.Lines[r.End.Line+1:]...)
	b.Lines = out
	b.dirty = true
	b.Version++
	b.bumpRevision()
	return b.posAfter(r.Start, text)
}

// bumpRevision выдаёт каждому новому содержимому уникальный номер состояния.
func (b *Buffer) bumpRevision() {
	b.revision = b.nextRevision
	b.nextRevision++
}

// orderedRange нормализует и ограничивает диапазон.
func (b *Buffer) orderedRange(r Range) Range {
	r.Start = b.clamp(r.Start)
	r.End = b.clamp(r.End)
	if posLess(r.End, r.Start) {
		r.Start, r.End = r.End, r.Start
	}
	return r
}

// posAfter - позиция сразу после текста, вставленного в start.
func (b *Buffer) posAfter(start Pos, text string) Pos {
	chunks := strings.Split(text, "\n")
	if len(chunks) == 1 {
		return Pos{start.Line, start.Col + len([]rune(text))}
	}
	return Pos{start.Line + len(chunks) - 1, len([]rune(chunks[len(chunks)-1]))}
}

// textRange возвращает текст диапазона, включая переводы строк.
func (b *Buffer) textRange(r Range) string {
	r = b.orderedRange(r)
	if r.Start.Line == r.End.Line {
		return string(b.Lines[r.Start.Line][r.Start.Col:r.End.Col])
	}
	var sb strings.Builder
	sb.WriteString(string(b.Lines[r.Start.Line][r.Start.Col:]))
	for i := r.Start.Line + 1; i <= r.End.Line; i++ {
		sb.WriteByte('\n')
		line := b.Lines[i]
		if i == r.End.Line {
			line = line[:r.End.Col]
		}
		sb.WriteString(string(line))
	}
	return sb.String()
}

// mapPos пересчитывает позицию p после замены r на text.
func (b *Buffer) mapPos(p Pos, r Range, text string) Pos {
	r = b.orderedRange(r)
	p = b.clamp(p)
	end := b.posAfter(r.Start, text)
	switch {
	case posLess(p, r.Start):
		return p
	case posLess(p, r.End):
		return end
	case p.Line == r.End.Line:
		return Pos{end.Line, end.Col + p.Col - r.End.Col}
	default:
		return Pos{p.Line + end.Line - r.End.Line, p.Col}
	}
}

func posLess(a, b Pos) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

// Indent - ведущие пробелы/табы строки (для автоотступа).
func (b *Buffer) Indent(p Pos) string {
	line := b.Lines[b.clamp(p).Line]
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return string(line[:n])
}

// Next - следующая логическая позиция (по рунам).
func (b *Buffer) Next(p Pos) Pos {
	p = b.clamp(p)
	if p.Col < len(b.Lines[p.Line]) {
		return Pos{p.Line, p.Col + 1}
	}
	if p.Line < len(b.Lines)-1 {
		return Pos{p.Line + 1, 0}
	}
	return p
}

// Prev - предыдущая логическая позиция (по рунам).
func (b *Buffer) Prev(p Pos) Pos {
	p = b.clamp(p)
	if p.Col > 0 {
		return Pos{p.Line, p.Col - 1}
	}
	if p.Line > 0 {
		return Pos{p.Line - 1, len(b.Lines[p.Line-1])}
	}
	return p
}

// nextPos - следующая позиция по графемным кластерам.
func (b *Buffer) nextPos(p Pos) (Pos, bool) {
	p = b.clamp(p)
	if p.Col < len(b.Lines[p.Line]) {
		return Pos{p.Line, clusterEnd(b.Lines[p.Line], p.Col)}, true
	}
	if p.Line < len(b.Lines)-1 {
		return Pos{p.Line + 1, 0}, true
	}
	return p, false
}

// prevPos - предыдущая позиция по графемным кластерам.
func (b *Buffer) prevPos(p Pos) (Pos, bool) {
	p = b.clamp(p)
	if p.Col > 0 {
		return Pos{p.Line, clusterStart(b.Lines[p.Line], p.Col)}, true
	}
	if p.Line > 0 {
		return Pos{p.Line - 1, len(b.Lines[p.Line-1])}, true
	}
	return p, false
}

// LineStart - начало строки.
func (b *Buffer) LineStart(p Pos) Pos { return Pos{b.clamp(p).Line, 0} }

// LineEnd - конец строки.
func (b *Buffer) LineEnd(p Pos) Pos {
	p = b.clamp(p)
	return Pos{p.Line, len(b.Lines[p.Line])}
}

// WordRight - вправо через пробелы/пунктуацию и слово.
func (b *Buffer) WordRight(p Pos) Pos {
	p = b.clamp(p)
	for {
		r, _, ok := b.charAt(p)
		if !ok {
			return p
		}
		if isWord(r) {
			break
		}
		p = b.Next(p)
	}
	for {
		r, _, ok := b.charAt(p)
		if !ok {
			return p
		}
		if !isWord(r) {
			return p
		}
		p = b.Next(p)
	}
}

// WordLeft - влево через пробелы/пунктуацию и слово.
func (b *Buffer) WordLeft(p Pos) Pos {
	p = b.clamp(p)
	for {
		r, at, ok := b.charBefore(p)
		if !ok {
			return p
		}
		if isWord(r) {
			break
		}
		p = at
	}
	for {
		r, at, ok := b.charBefore(p)
		if !ok {
			return p
		}
		if !isWord(r) {
			return p
		}
		p = at
	}
}

func (b *Buffer) charAt(p Pos) (rune, Pos, bool) {
	p = b.clamp(p)
	if p.Col < len(b.Lines[p.Line]) {
		return b.Lines[p.Line][p.Col], p, true
	}
	if p.Line >= len(b.Lines)-1 {
		return 0, p, false
	}
	return '\n', Pos{p.Line, len(b.Lines[p.Line])}, true
}

func (b *Buffer) charBefore(p Pos) (rune, Pos, bool) {
	p = b.clamp(p)
	if p.Col > 0 {
		return b.Lines[p.Line][p.Col-1], Pos{p.Line, p.Col - 1}, true
	}
	if p.Line == 0 {
		return 0, p, false
	}
	return '\n', Pos{p.Line - 1, len(b.Lines[p.Line-1])}, true
}

func isWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
