package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestReadFrameHeaderCaseAndBufferedFrames(t *testing.T) {
	wire := "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\n" +
		"content-length: 5\r\n" +
		"\r\n" +
		"hello" +
		"CONTENT-LENGTH: 5\r\n\r\nworld"

	r := bufio.NewReader(strings.NewReader(wire))
	first, err := readFrame(r)
	if err != nil {
		t.Fatalf("first frame: %v", err)
	}
	if string(first) != "hello" {
		t.Fatalf("first = %q", first)
	}
	second, err := readFrame(r)
	if err != nil {
		t.Fatalf("second frame: %v", err)
	}
	if string(second) != "world" {
		t.Fatalf("second = %q", second)
	}
}

func TestReadFrameLargeMessage(t *testing.T) {
	body := bytes.Repeat([]byte("я"), 512*1024)
	var buf bytes.Buffer
	if err := writeFrame(&buf, body); err != nil {
		t.Fatal(err)
	}
	got, err := readFrame(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body length = %d, want %d", len(got), len(body))
	}
}

func TestWriteFrameFormat(t *testing.T) {
	var buf bytes.Buffer
	body := []byte(`{"jsonrpc":"2.0"}`)
	if err := writeFrame(&buf, body); err != nil {
		t.Fatal(err)
	}
	want := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + string(body)
	if buf.String() != want {
		t.Fatalf("frame = %q, want %q", buf.String(), want)
	}
}

func TestReadFrameErrors(t *testing.T) {
	cases := []string{
		"Content-Type: x\r\n\r\n{}",
		"Content-Length: nope\r\n\r\n",
		"Content-Length: -1\r\n\r\n",
		"Content-Length: 999999999999\r\n\r\n",
		"broken header\r\n\r\n",
	}
	for _, wire := range cases {
		if _, err := readFrame(bufio.NewReader(strings.NewReader(wire))); err == nil {
			t.Errorf("readFrame(%q) returned nil error", wire)
		}
	}
}

func TestMessageID(t *testing.T) {
	cases := map[string]int64{
		`1`:    1,
		`"42"`: 42,
	}
	for raw, want := range cases {
		got, ok := messageID(json.RawMessage(raw))
		if !ok || got != want {
			t.Errorf("messageID(%s) = %d, %v; want %d", raw, got, ok, want)
		}
	}
	if _, ok := messageID(json.RawMessage(`"x"`)); ok {
		t.Error("non-numeric string id accepted")
	}
	if _, ok := messageID(nil); ok {
		t.Error("empty id accepted")
	}
}
