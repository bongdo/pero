// Package workspace - проект, файлы, fuzzy-поиск и недавние файлы для pero.
package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// File - файл проекта. Rel считается от корня проекта.
type File struct {
	Path    string
	Rel     string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// defaultIgnoreDirs - игнор, если вызывающий не передал свой список.
var defaultIgnoreDirs = []string{
	".git", ".jj", "node_modules", "target", ".direnv",
	"result*", "dist", "build", ".cache",
}

// defaultListLimit и listTimeout - предохранители для очень больших проектов.
const (
	defaultListLimit = 100_000
	listTimeout      = 2 * time.Second
)

// Root возвращает абсолютный корень проекта: вверх до каталога с .git или .jj,
// иначе стартовый каталог. Если start - файл, берётся его каталог.
func Root(start string) string {
	if start == "" {
		start = "."
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		abs = filepath.Clean(start)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	for dir := abs; ; {
		for _, marker := range []string{".git", ".jj"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		dir = parent
	}
}

// ShouldIgnore сообщает, попадает ли имя каталога под ignore-список.
// Пустой список означает дефолтный. Поддерживаются glob-шаблоны ("result*").
func ShouldIgnore(name string, ignoreDirs []string) bool {
	list := ignoreDirs
	if len(list) == 0 {
		list = defaultIgnoreDirs
	}
	for _, pattern := range list {
		if pattern == "" {
			continue
		}
		if pattern == name {
			return true
		}
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
	}
	return false
}

// Walk обходит root рекурсивно и отправляет файлы в out. Обход идёт в
// горутине, поэтому вызывающий не блокируется; канал закрывается ровно один
// раз. Симлинки-каталоги не обходятся и в список не попадают, ошибки доступа
// пропускаются, содержимое файлов не читается.
func Walk(ctx context.Context, root string, ignoreDirs []string, out chan<- File) {
	cleanRoot := filepath.Clean(root)
	go func() {
		defer close(out)
		walkDir(ctx, cleanRoot, cleanRoot, ignoreDirs, out)
	}()
}

func walkDir(ctx context.Context, root, dir string, ignoreDirs []string, out chan<- File) {
	if ctx.Err() != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		name := entry.Name()
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			if !ShouldIgnore(name, ignoreDirs) {
				walkDir(ctx, root, path, ignoreDirs, out)
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		size, modTime := info.Size(), info.ModTime()
		// Симлинк на обычный файл показываем с размером цели; на каталог,
		// битый и прочие цели - пропускаем, внутрь не идём.
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Stat(path)
			if err != nil || !target.Mode().IsRegular() {
				continue
			}
			size, modTime = target.Size(), target.ModTime()
		}
		f := File{Path: path, Size: size, ModTime: modTime}
		if rel, err := filepath.Rel(root, path); err == nil {
			f.Rel = rel
		} else {
			f.Rel = path
		}
		select {
		case out <- f:
		case <-ctx.Done():
			return
		}
	}
}

// List собирает до limit файлов (0 - 100k) не дольше двух секунд и сортирует
// их по Rel. Порядок детерминирован.
func List(ctx context.Context, root string, ignoreDirs []string, limit int) []File {
	if limit <= 0 {
		limit = defaultListLimit
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	out := make(chan File, 256)
	Walk(ctx, root, ignoreDirs, out)
	files := make([]File, 0, min(limit, 4096))
	for f := range out {
		files = append(files, f)
		if len(files) >= limit {
			cancel()
			for range out {
			}
			break
		}
	}
	slices.SortFunc(files, func(a, b File) int {
		if c := strings.Compare(a.Rel, b.Rel); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return files
}

// IsBinary - файл бинарный, если в первых 8 КБ есть NUL.
func IsBinary(data []byte) bool {
	if len(data) > 8192 {
		data = data[:8192]
	}
	return bytes.IndexByte(data, 0) >= 0
}

var proseExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".rst": true, ".org": true,
}

var codeExts = map[string]bool{
	".go": true, ".js": true, ".ts": true, ".rs": true, ".c": true, ".h": true,
	".lua": true, ".nix": true, ".nim": true, ".py": true, ".sh": true,
	".mojo": true, ".json": true, ".toml": true, ".yaml": true, ".yml": true,
	".ini": true,
}

// IsProse - текстовый формат (markdown/rst/org/plain).
func IsProse(path string) bool {
	return proseExts[strings.ToLower(filepath.Ext(path))]
}

// IsCode - язык программирования или конфиг.
func IsCode(path string) bool {
	return codeExts[strings.ToLower(filepath.Ext(path))]
}
