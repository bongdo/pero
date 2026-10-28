package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collectFiles(t *testing.T, out <-chan File) []File {
	t.Helper()
	var files []File
	deadline := time.After(5 * time.Second)
	for {
		select {
		case file, ok := <-out:
			if !ok {
				return files
			}
			files = append(files, file)
		case <-deadline:
			t.Fatal("Walk не закрыл канал за 5 секунд")
			return files
		}
	}
}

func rels(files []File) []string {
	out := make([]string, len(files))
	for i, file := range files {
		out[i] = file.Rel
	}
	return out
}

func TestRootGit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".git", "config"), "")
	sub := filepath.Join(root, "a", "b")
	writeTestFile(t, filepath.Join(sub, "main.go"), "package main")

	if got := Root(sub); got != root {
		t.Fatalf("Root(%q) = %q, want %q", sub, got, root)
	}
	if got := Root(filepath.Join(sub, "main.go")); got != root {
		t.Fatalf("Root от файла = %q, want %q", got, root)
	}
}

func TestRootGitFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".git"), "gitdir: /elsewhere\n")
	sub := filepath.Join(root, "pkg")
	writeTestFile(t, filepath.Join(sub, "x.go"), "")

	if got := Root(sub); got != root {
		t.Fatalf("Root(%q) = %q, want %q", sub, got, root)
	}
}

func TestRootJJ(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Root(sub); got != root {
		t.Fatalf("Root(%q) = %q, want %q", sub, got, root)
	}
}

func TestRootNoMarker(t *testing.T) {
	start := t.TempDir()
	if got := Root(start); got != start {
		t.Fatalf("Root(%q) = %q, want стартовый каталог", start, got)
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name    string
		ignore  []string
		ignored bool
	}{
		{"node_modules", nil, true},
		{".git", nil, true},
		{".jj", nil, true},
		{"target", nil, true},
		{"result-bin", nil, true},
		{"src", nil, false},
		{"src", []string{"src"}, true},
		{"SRC", []string{"src"}, false},
		{"vendor", []string{"vendor", "tmp"}, true},
	}
	for _, tc := range tests {
		if got := ShouldIgnore(tc.name, tc.ignore); got != tc.ignored {
			t.Errorf("ShouldIgnore(%q, %v) = %v, want %v", tc.name, tc.ignore, got, tc.ignored)
		}
	}
}

func TestWalkIgnores(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src", "main.go"), "")
	for _, dir := range []string{
		".git", ".jj", "node_modules", "target", ".direnv",
		"dist", "build", ".cache", "result", "result-bin",
	} {
		writeTestFile(t, filepath.Join(root, dir, "x.txt"), "")
	}

	out := make(chan File)
	Walk(context.Background(), root, nil, out)
	files := collectFiles(t, out)
	if got, want := rels(files), []string{"src/main.go"}; !slices.Equal(got, want) {
		t.Fatalf("дефолтный ignore: получили %v, want %v", got, want)
	}

	out = make(chan File)
	Walk(context.Background(), root, []string{"src"}, out)
	files = collectFiles(t, out)
	got := rels(files)
	if slices.Contains(got, "src/main.go") {
		t.Fatalf("свой ignore не сработал: %v", got)
	}
	if !slices.Contains(got, filepath.Join("node_modules", "x.txt")) {
		t.Fatalf("свой ignore должен заменить дефолтный, получили %v", got)
	}
}

func TestWalkSymlinkCycle(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "file.txt"), "x")
	if err := os.Symlink(root, filepath.Join(root, "a", "loop")); err != nil {
		t.Skipf("симлинки недоступны: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "a", "file.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	out := make(chan File)
	Walk(context.Background(), root, nil, out)
	files := collectFiles(t, out)
	got := rels(files)
	if !slices.Contains(got, filepath.Join("a", "file.txt")) {
		t.Fatalf("файл потерялся: %v", got)
	}
	if !slices.Contains(got, "link.txt") {
		t.Fatalf("симлинк на файл потерялся: %v", got)
	}
	if slices.Contains(got, filepath.Join("a", "loop")) {
		t.Fatalf("симлинк на каталог попал в список файлов: %v", got)
	}
	for _, file := range files {
		if file.Rel == "link.txt" && file.Size != 1 {
			t.Fatalf("размер симлинка = %d, want размер цели 1", file.Size)
		}
	}
}

func TestWalkCancelled(t *testing.T) {
	root := t.TempDir()
	for i := range 20 {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("f%02d.txt", i)), "")
	}

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan File)
	Walk(ctx, root, nil, out)
	select {
	case _, ok := <-out:
		if !ok {
			t.Fatal("обход закрылся до отмены")
		}
	case <-time.After(time.Second):
		t.Fatal("Walk ничего не прислал")
	}
	cancel()

	done := make(chan struct{})
	go func() {
		for range out {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Walk не остановился по отмене ctx")
	}
}

func TestWalkCancelledBeforeStart(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "x.txt"), "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := make(chan File)
	Walk(ctx, root, nil, out)
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("отменённый обход прислал файл")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("канал не закрылся")
	}
}

func TestWalkDoesNotBlockCaller(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "x.txt"), "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan File)
	returned := make(chan struct{})
	go func() {
		Walk(ctx, root, nil, out)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Walk заблокировал вызывающего")
	}
	for range out {
	}
}

func TestWalkPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root читает каталоги без прав")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "ok.txt"), "")
	blocked := filepath.Join(root, "blocked")
	writeTestFile(t, filepath.Join(blocked, "secret.txt"), "")
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })

	out := make(chan File)
	Walk(context.Background(), root, nil, out)
	files := collectFiles(t, out)
	if !slices.Contains(rels(files), "ok.txt") {
		t.Fatalf("потеряли доступный файл: %v", rels(files))
	}
}

func TestListDeterministic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "b.go"), "")
	writeTestFile(t, filepath.Join(root, "a.go"), "")
	writeTestFile(t, filepath.Join(root, "sub", "c.go"), "")
	writeTestFile(t, filepath.Join(root, "node_modules", "x.js"), "")

	files := List(context.Background(), root, nil, 0)
	want := []string{"a.go", "b.go", filepath.Join("sub", "c.go")}
	if got := rels(files); !slices.Equal(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
	again := List(context.Background(), root, nil, 0)
	if !reflect.DeepEqual(files, again) {
		t.Fatalf("List недетерминирован:\n%v\n%v", files, again)
	}

	limited := List(context.Background(), root, nil, 2)
	if got := rels(limited); !slices.Equal(got, want[:2]) {
		t.Fatalf("List с limit = %v, want %v", got, want[:2])
	}
}

func TestListCancelled(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "x.txt"), "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if files := List(ctx, root, nil, 0); len(files) != 0 {
		t.Fatalf("отменённый List вернул %v", rels(files))
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"текст", []byte("привет, мир\n"), false},
		{"пусто", nil, false},
		{"нуль в начале", []byte("\x00abc"), true},
		{"нуль в конце окна", append(bytes.Repeat([]byte("a"), 8191), 0), true},
		{"нуль после окна", append(bytes.Repeat([]byte("a"), 8192), 0), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBinary(tc.data); got != tc.want {
				t.Fatalf("IsBinary = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProseAndCode(t *testing.T) {
	prose := []string{"a.md", "b.markdown", "c.txt", "d.rst", "e.org", "ПИСЬМО.MD"}
	for _, path := range prose {
		if !IsProse(path) {
			t.Errorf("IsProse(%q) = false", path)
		}
	}
	code := []string{
		"a.go", "b.js", "c.ts", "d.rs", "e.c", "f.h", "g.lua", "h.nix",
		"i.nim", "j.py", "k.sh", "l.mojo", "m.json", "n.toml", "o.yaml",
		"p.yml", "q.ini", "R.GO",
	}
	for _, path := range code {
		if !IsCode(path) {
			t.Errorf("IsCode(%q) = false", path)
		}
	}
	if IsProse("main.go") || IsCode("README.md") || IsCode("LICENSE") {
		t.Fatal("режимы пересеклись")
	}
}
