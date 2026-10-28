package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderToastKinds(t *testing.T) {
	cases := []struct {
		kind, icon string
	}{
		{"success", "✓"},
		{"info", "i"},
		{"warning", "!"},
		{"error", "×"},
		{"progress", "…"},
	}
	for _, c := range cases {
		out := RenderToast(Toast{Text: "сохранено", Kind: c.kind}, 40, Style{})
		plain := ansi.Strip(out)
		if !strings.HasPrefix(plain, c.icon) {
			t.Errorf("%s: %q does not start with %q", c.kind, plain, c.icon)
		}
		if !strings.Contains(plain, "сохранено") {
			t.Errorf("%s: text lost: %q", c.kind, plain)
		}
		if strings.Contains(out, "\n") {
			t.Errorf("%s: toast is multiline", c.kind)
		}
	}
}

func TestRenderToastWidthAndTruncation(t *testing.T) {
	long := strings.Repeat("очень длинное уведомление ", 10)
	for _, width := range []int{20, 40, 80, 200} {
		out := RenderToast(Toast{Text: long, Kind: "error"}, width, Style{})
		want := min(40, width-2)
		if got := ansi.StringWidth(out); got > want {
			t.Errorf("width %d: toast width = %d, want <= %d", width, got, want)
		}
		if !strings.Contains(ansi.Strip(out), "…") {
			t.Errorf("width %d: no truncation marker", width)
		}
	}

	for _, width := range []int{0, 1, 2} {
		if got := RenderToast(Toast{Text: "x"}, width, Style{}); got != "" {
			t.Errorf("width %d: got %q, want empty", width, got)
		}
	}
}

func TestRenderToastExpiry(t *testing.T) {
	if got := RenderToast(Toast{Text: "старое", Until: time.Now().Add(-time.Second)}, 40, Style{}); got != "" {
		t.Errorf("expired toast = %q, want empty", got)
	}
	future := time.Now().Add(time.Hour)
	if got := RenderToast(Toast{Text: "живое", Until: future}, 40, Style{}); got == "" {
		t.Error("live toast disappeared")
	}
	if got := RenderToast(Toast{Text: "живое"}, 40, Style{}); got == "" {
		t.Error("toast without deadline disappeared")
	}
}

func TestRenderToastStyleAndSanitize(t *testing.T) {
	out := RenderToast(Toast{Text: "a\nb\tc", Kind: "error"}, 40, Style{Error: "\x1b[31m"})
	if !strings.HasPrefix(out, "\x1b[31m") {
		t.Errorf("error color missing: %q", out)
	}
	if !strings.Contains(out, "\x1b[0m") {
		t.Errorf("reset missing: %q", out)
	}
	plain := ansi.Strip(out)
	if strings.ContainsAny(plain, "\n\t") {
		t.Errorf("toast not single-line: %q", plain)
	}
	if plain != "× a b c" {
		t.Errorf("toast = %q, want %q", plain, "× a b c")
	}
}
