package lsp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func requestConfiguration(t *testing.T, server *fakeServer, id int64, sections ...string) []any {
	t.Helper()
	items := make([]map[string]string, len(sections))
	for i, section := range sections {
		items[i] = map[string]string{"section": section}
	}
	server.request(id, "workspace/configuration", map[string]any{"items": items})

	response := server.next(t)
	if got, ok := messageID(response.ID); !ok || got != id {
		t.Fatalf("configuration response id = %s, want %d", response.ID, id)
	}
	var values []any
	if err := json.Unmarshal(response.Result, &values); err != nil {
		t.Fatalf("configuration result = %s: %v", response.Result, err)
	}
	return values
}

func TestWorkspaceConfiguration(t *testing.T) {
	client, server := newTestClient(t)
	client.SetConfiguration(map[string]any{
		"ltex":      map[string]any{"language": "ru-RU", "enabled": true},
		"harper-ls": map[string]any{"dialect": "British"},
	})

	got := requestConfiguration(t, server, 41, "ltex", "", "harper-ls", "ltex.language", "unknown")
	want := []any{
		map[string]any{"language": "ru-RU", "enabled": true},
		map[string]any{
			"ltex":      map[string]any{"language": "ru-RU", "enabled": true},
			"harper-ls": map[string]any{"dialect": "British"},
		},
		map[string]any{"dialect": "British"},
		"ru-RU",
		nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuration = %#v, want %#v", got, want)
	}
}

func TestWorkspaceConfigurationHandlerFallsBack(t *testing.T) {
	client, server := newTestClient(t)
	client.SetConfiguration(map[string]any{"ltex": map[string]any{"language": "ru-RU"}})
	client.SetConfigurationHandler(func(section string) any {
		switch section {
		case "custom":
			return map[string]any{"flag": true}
		case "bad":
			return make(chan int) // not JSON: must degrade to null
		}
		return nil
	})

	got := requestConfiguration(t, server, 42, "custom", "ltex", "bad", "missing")
	want := []any{
		map[string]any{"flag": true},
		map[string]any{"language": "ru-RU"},
		nil,
		nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuration = %#v, want %#v", got, want)
	}
}

func TestWorkspaceConfigurationWithoutSettings(t *testing.T) {
	_, server := newTestClient(t)
	got := requestConfiguration(t, server, 43, "ltex", "")
	if !reflect.DeepEqual(got, []any{nil, nil}) {
		t.Fatalf("configuration = %#v, want nulls", got)
	}
}

func TestExecuteCommand(t *testing.T) {
	client, server := newTestClient(t)
	args := json.RawMessage(`[{"uri":"file:///tmp/a.md","codeLanguageId":"markdown"}]`)

	done := make(chan error, 1)
	go func() {
		done <- client.ExecuteCommand(context.Background(), "_ltex.checkDocument", args)
	}()
	req := server.next(t)
	if req.Method != "workspace/executeCommand" {
		t.Fatalf("method = %q", req.Method)
	}
	var params struct {
		Command   string            `json:"command"`
		Arguments []json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Command != "_ltex.checkDocument" || len(params.Arguments) != 1 ||
		string(params.Arguments[0]) != `{"uri":"file:///tmp/a.md","codeLanguageId":"markdown"}` {
		t.Fatalf("params = %#v", params)
	}
	server.respond(req.ID, map[string]any{"success": true})
	if err := <-done; err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}

	// nil args must omit "arguments" instead of sending null.
	done = make(chan error, 1)
	go func() {
		done <- client.ExecuteCommand(context.Background(), "_ltex.getServerStatus", nil)
	}()
	req = server.next(t)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["arguments"]; ok {
		t.Fatalf("nil args became %s", req.Params)
	}
	server.respond(req.ID, map[string]any{"success": true})
	if err := <-done; err != nil {
		t.Fatalf("ExecuteCommand without args: %v", err)
	}
}

func TestExecuteCommandError(t *testing.T) {
	client, server := newTestClient(t)
	done := make(chan error, 1)
	go func() {
		done <- client.ExecuteCommand(context.Background(), "missing.command", json.RawMessage(`[]`))
	}()
	req := server.next(t)
	server.sendValue(rpcErrorResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &rpcError{Code: -32601, Message: "command not found"},
	})
	if err := <-done; err == nil {
		t.Fatal("ExecuteCommand returned nil for a server error")
	}
}

func TestCloseSendsShutdownAndExit(t *testing.T) {
	client, server := newTestClient(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	shutdown := server.next(t)
	if shutdown.Method != "shutdown" {
		t.Fatalf("first close message = %q, want shutdown", shutdown.Method)
	}
	exit := server.next(t)
	if exit.Method != "exit" {
		t.Fatalf("second close message = %q, want exit", exit.Method)
	}

	// A second Close is a no-op; it must not send shutdown twice.
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
