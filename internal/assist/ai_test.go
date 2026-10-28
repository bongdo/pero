package assist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fakeOpencode кладёт исполняемый скрипт opencode в отдельный PATH.
func fakeOpencode(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestRunOpencodeStdinStdout(t *testing.T) {
	fakeOpencode(t, "[ \"$1\" = \"run\" ] || exit 9\nprintf 'получил:%s' \"$2\"\nprintf 'шум' >&2\n")

	out, err := RunOpencode(context.Background(), "привет мир")
	if err != nil {
		t.Fatal(err)
	}
	if out != "получил:привет мир" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestRunOpencodeExitErrorWithStderr(t *testing.T) {
	fakeOpencode(t, "printf 'нет соединения' >&2\nexit 3\n")

	out, err := RunOpencode(context.Background(), "вопрос")
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if out != "" {
		t.Fatalf("stdout = %q, want пусто", out)
	}
	if !strings.Contains(err.Error(), "нет соединения") {
		t.Fatalf("ошибка без stderr: %v", err)
	}
}

func TestRunOpencodeNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := RunOpencode(context.Background(), "вопрос")
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if !strings.Contains(err.Error(), "opencode") || !strings.Contains(err.Error(), "PATH") {
		t.Fatalf("непонятная ошибка: %v", err)
	}
}

func TestRunOpencodeTimeout(t *testing.T) {
	fakeOpencode(t, "while :; do :; done\n")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := RunOpencode(ctx, "вопрос")
	if err == nil {
		t.Fatal("ожидалась ошибка таймаута")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("таймаут сработал через %s", elapsed)
	}
}

func TestRunOpencodeTruncatesOutput(t *testing.T) {
	fakeOpencode(t, "printf 'a'\ni=0\nwhile [ \"$i\" -lt 5000 ]; do\n\tprintf 'ё'\n\ti=$((i+1))\ndone\n")

	out, err := RunOpencode(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxOutputBytes {
		t.Fatalf("длина %d > %d", len(out), maxOutputBytes)
	}
	if !utf8.ValidString(out) {
		t.Fatal("обрезка сломала UTF-8")
	}
	if !strings.HasPrefix(out, "a") || !strings.HasSuffix(out, "ё") {
		t.Fatalf("неожиданный результат: %q", out[:min(len(out), 20)])
	}
}

func TestRunOpencodeTruncatesStderrInError(t *testing.T) {
	fakeOpencode(t, "i=0\nwhile [ \"$i\" -lt 5000 ]; do\n\tprintf 'ё' >&2\n\ti=$((i+1))\ndone\nexit 1\n")

	_, err := RunOpencode(context.Background(), "вопрос")
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if len(err.Error()) > maxOutputBytes+100 {
		t.Fatalf("stderr в ошибке не обрезан: %d байт", len(err.Error()))
	}
	if !utf8.ValidString(err.Error()) {
		t.Fatal("обрезка stderr сломала UTF-8")
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := []struct {
		name   string
		sample string
		want   string
	}{
		{"русский", "Привет, мир! Как дела?", "ru"},
		{"английский", "Hello world, how are you?", "en"},
		{"пусто", "", "en"},
		{"без букв", "12345 --- !!!", "en"},
		{"ё", "Ёжик и ёлка", "ru"},
		{"порог ровно", "абвabcdefghijklmnopq", "ru"},
		{"ниже порога", "абabcdefghijklmnopqr", "en"},
		{"смешанный", "Hello world. Привет!", "ru"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectLanguage(tc.sample); got != tc.want {
				t.Fatalf("DetectLanguage(%q) = %q, want %q", tc.sample, got, tc.want)
			}
		})
	}
}
