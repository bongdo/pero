package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeServer struct {
	out     io.Writer
	msgs    chan rpcMessage
	stop    chan struct{}
	stopOne sync.Once
	writeMu sync.Mutex
}

func newFakeServer(in io.Reader, out io.Writer) *fakeServer {
	f := &fakeServer{
		out:  out,
		msgs: make(chan rpcMessage, 64),
		stop: make(chan struct{}),
	}
	go func() {
		r := bufio.NewReader(in)
		for {
			body, err := readFrame(r)
			if err != nil {
				return
			}
			var msg rpcMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				continue
			}
			// Close sends shutdown and waits for a response before exit.
			if msg.Method == "shutdown" && len(msg.ID) > 0 {
				f.sendValue(rpcResultResponse{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")})
			}
			select {
			case f.msgs <- msg:
			case <-f.stop:
				return
			}
		}
	}()
	return f
}

func (f *fakeServer) close() {
	f.stopOne.Do(func() {
		close(f.stop)
		if closer, ok := f.out.(io.Closer); ok {
			_ = closer.Close()
		}
	})
}

func (f *fakeServer) next(t *testing.T) rpcMessage {
	t.Helper()
	select {
	case msg := <-f.msgs:
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("fake server: timed out waiting for a message")
		return rpcMessage{}
	}
}

func (f *fakeServer) sendRaw(body []byte) {
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	_ = writeFrame(f.out, body)
}

func (f *fakeServer) sendValue(value any) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	f.sendRaw(body)
}

func (f *fakeServer) sendBatch(bodies ...[]byte) {
	var buf bytes.Buffer
	for _, body := range bodies {
		_ = writeFrame(&buf, body)
	}
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	_, _ = f.out.Write(buf.Bytes())
}

func (f *fakeServer) respond(id json.RawMessage, result any) {
	f.sendValue(rpcResultResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (f *fakeServer) notify(method string, params any) {
	f.sendValue(rpcNotification{JSONRPC: "2.0", Method: method, Params: params})
}

func (f *fakeServer) request(id int64, method string, params any) {
	f.sendValue(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
}

func newTestClient(t *testing.T) (*Client, *fakeServer) {
	t.Helper()
	return newTestClientWithResult(t, map[string]any{
		"capabilities": map[string]any{
			"hoverProvider":              true,
			"completionProvider":         map[string]any{},
			"definitionProvider":         true,
			"referencesProvider":         true,
			"documentFormattingProvider": true,
			"codeActionProvider":         true,
		},
	})
}

func newTestClientWithResult(t *testing.T, initializeResult any) (*Client, *fakeServer) {
	t.Helper()
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	server := newFakeServer(serverIn, serverOut)
	client := NewClientWithStreams(ServerConfig{Command: "fake-lsp", LanguageID: "go"}, t.TempDir(), clientIn, clientOut)
	t.Cleanup(func() {
		_ = client.Close()
		server.close()
	})

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	initDone := make(chan error, 1)
	go func() {
		initDone <- client.Initialize(context.Background())
	}()

	init := server.next(t)
	if init.Method != "initialize" {
		t.Fatalf("first request = %q, want initialize", init.Method)
	}
	var params struct {
		RootURI      string         `json:"rootUri"`
		RootPath     string         `json:"rootPath"`
		ClientInfo   map[string]any `json:"clientInfo"`
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(init.Params, &params); err != nil {
		t.Fatalf("initialize params: %v", err)
	}
	if params.RootURI != PathToURI(client.root) {
		t.Errorf("rootUri = %q, want %q", params.RootURI, PathToURI(client.root))
	}
	if params.ClientInfo["name"] != "pero" {
		t.Errorf("clientInfo = %#v", params.ClientInfo)
	}
	server.respond(init.ID, initializeResult)

	initialized := server.next(t)
	if initialized.Method != "initialized" {
		t.Fatalf("second message = %q, want initialized", initialized.Method)
	}
	if err := <-initDone; err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return client, server
}

func TestInitializeDocumentSyncAndDiagnostics(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	diagnostics := make(chan []Diagnostic, 2)
	client.SetDiagnosticsHandler(func(gotURI string, diags []Diagnostic) {
		if gotURI == uri {
			diagnostics <- diags
		}
	})

	client.DidOpen(uri, "go", 1, "package main\n")
	open := server.next(t)
	if open.Method != "textDocument/didOpen" {
		t.Fatalf("method = %q", open.Method)
	}
	var openParams struct {
		TextDocument struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Version    int    `json:"version"`
			Text       string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(open.Params, &openParams); err != nil {
		t.Fatal(err)
	}
	if openParams.TextDocument.URI != uri || openParams.TextDocument.LanguageID != "go" ||
		openParams.TextDocument.Version != 1 || openParams.TextDocument.Text != "package main\n" {
		t.Fatalf("didOpen params = %#v", openParams)
	}

	server.notify("textDocument/publishDiagnostics", map[string]any{
		"uri":     uri,
		"version": 2,
		"diagnostics": []any{map[string]any{
			"range":    map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 7}},
			"severity": 1,
			"message":  "undefined: package",
			"source":   "gopls",
			"code":     "E1",
		}},
	})
	select {
	case got := <-diagnostics:
		if len(got) != 1 || got[0].Message != "undefined: package" || got[0].Severity != 1 {
			t.Fatalf("handler diags = %#v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("diagnostics handler was not called")
	}
	cached := client.Diagnostics(uri)
	if len(cached) != 1 || cached[0].Source != "gopls" || string(cached[0].Code) != `"E1"` {
		t.Fatalf("cached diagnostics = %#v", cached)
	}

	// A stale publish must not replace the newer version.
	server.notify("textDocument/publishDiagnostics", map[string]any{
		"uri": uri, "version": 1,
		"diagnostics": []any{map[string]any{
			"range":    map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
			"severity": 2, "message": "stale",
		}},
	})
	select {
	case got := <-diagnostics:
		t.Fatalf("stale diagnostics reached handler: %#v", got)
	case <-time.After(100 * time.Millisecond):
	}
	if got := client.Diagnostics(uri); len(got) != 1 || got[0].Message != "undefined: package" {
		t.Fatalf("stale publish replaced cache: %#v", got)
	}

	client.DidChange(uri, 3, "package main\n\nfunc main() {}\n")
	change := server.next(t)
	if change.Method != "textDocument/didChange" {
		t.Fatalf("method = %q", change.Method)
	}
	var changeParams struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
		} `json:"textDocument"`
		ContentChanges []map[string]string `json:"contentChanges"`
	}
	if err := json.Unmarshal(change.Params, &changeParams); err != nil {
		t.Fatal(err)
	}
	if changeParams.TextDocument.Version != 3 || len(changeParams.ContentChanges) != 1 ||
		changeParams.ContentChanges[0]["text"] != "package main\n\nfunc main() {}\n" {
		t.Fatalf("didChange params = %#v", changeParams)
	}

	client.DidSave(uri)
	if msg := server.next(t); msg.Method != "textDocument/didSave" {
		t.Fatalf("method = %q", msg.Method)
	}
	client.DidClose(uri)
	if msg := server.next(t); msg.Method != "textDocument/didClose" {
		t.Fatalf("method = %q", msg.Method)
	}
}

func TestCompletion(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	type result struct {
		items []CompletionItem
		err   error
	}
	done := make(chan result, 1)
	go func() {
		items, err := client.Completion(context.Background(), uri, Position{Line: 3, Character: 7})
		done <- result{items, err}
	}()

	req := server.next(t)
	if req.Method != "textDocument/completion" {
		t.Fatalf("method = %q", req.Method)
	}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position Position `json:"position"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.TextDocument.URI != uri || params.Position != (Position{Line: 3, Character: 7}) {
		t.Fatalf("completion params = %#v", params)
	}

	server.respond(req.ID, map[string]any{
		"isIncomplete": false,
		"items": []any{
			map[string]any{
				"label":         "FindUser",
				"detail":        "func(id int) (*User, error)",
				"kind":          3,
				"insertText":    "FindUser",
				"documentation": map[string]string{"kind": "markdown", "value": "Finds a user."},
			},
			map[string]any{
				"label": "User",
				"kind":  "22",
				"textEdit": map[string]any{
					"range":   map[string]any{"start": map[string]int{"line": 3, "character": 0}, "end": map[string]int{"line": 3, "character": 4}},
					"newText": "User",
				},
				"documentation": "A user.",
			},
			map[string]any{"label": "WithoutInsert"},
		},
	})

	got := <-done
	if got.err != nil {
		t.Fatalf("Completion: %v", got.err)
	}
	if len(got.items) != 3 {
		t.Fatalf("items = %#v", got.items)
	}
	if got.items[0].Kind != "func" || got.items[0].Documentation != "Finds a user." {
		t.Errorf("first item = %#v", got.items[0])
	}
	if got.items[1].Kind != "type" || got.items[1].InsertText != "User" || got.items[1].TextEdit == nil {
		t.Errorf("second item = %#v", got.items[1])
	}
	if got.items[2].InsertText != "WithoutInsert" {
		t.Errorf("label fallback = %#v", got.items[2])
	}
}

func TestHoverDefinitionReferencesFormat(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))
	pos := Position{Line: 1, Character: 2}

	hoverDone := make(chan struct {
		hover *Hover
		err   error
	}, 1)
	go func() {
		hover, err := client.Hover(context.Background(), uri, pos)
		hoverDone <- struct {
			hover *Hover
			err   error
		}{hover, err}
	}()
	req := server.next(t)
	if req.Method != "textDocument/hover" {
		t.Fatalf("method = %q", req.Method)
	}
	server.respond(req.ID, map[string]any{
		"contents": []any{
			map[string]string{"language": "go", "value": "func FindUser(id int)"},
			"plain text",
		},
		"range": map[string]any{"start": map[string]int{"line": 1, "character": 2}, "end": map[string]int{"line": 1, "character": 8}},
	})
	hoverResult := <-hoverDone
	if hoverResult.err != nil {
		t.Fatalf("Hover: %v", hoverResult.err)
	}
	if !strings.Contains(hoverResult.hover.Contents, "```go\nfunc FindUser(id int)\n```") ||
		!strings.Contains(hoverResult.hover.Contents, "plain text") {
		t.Fatalf("hover contents = %q", hoverResult.hover.Contents)
	}
	if hoverResult.hover.Range == nil || hoverResult.hover.Range.Start.Character != 2 {
		t.Fatalf("hover range = %#v", hoverResult.hover.Range)
	}

	defDone := make(chan struct {
		locations []Location
		err       error
	}, 1)
	go func() {
		locations, err := client.Definition(context.Background(), uri, pos)
		defDone <- struct {
			locations []Location
			err       error
		}{locations, err}
	}()
	req = server.next(t)
	if req.Method != "textDocument/definition" {
		t.Fatalf("method = %q", req.Method)
	}
	target := PathToURI(filepath.Join(client.root, "user.go"))
	server.respond(req.ID, []any{map[string]any{
		"targetUri":            target,
		"targetRange":          map[string]any{"start": map[string]int{"line": 10, "character": 0}, "end": map[string]int{"line": 20, "character": 0}},
		"targetSelectionRange": map[string]any{"start": map[string]int{"line": 12, "character": 5}, "end": map[string]int{"line": 12, "character": 12}},
	}})
	defResult := <-defDone
	if defResult.err != nil {
		t.Fatalf("Definition: %v", defResult.err)
	}
	if len(defResult.locations) != 1 || defResult.locations[0].URI != target ||
		defResult.locations[0].Range.Start.Line != 12 {
		t.Fatalf("definition = %#v", defResult.locations)
	}

	refDone := make(chan struct {
		locations []Location
		err       error
	}, 1)
	go func() {
		locations, err := client.References(context.Background(), uri, pos)
		refDone <- struct {
			locations []Location
			err       error
		}{locations, err}
	}()
	req = server.next(t)
	if req.Method != "textDocument/references" {
		t.Fatalf("method = %q", req.Method)
	}
	server.respond(req.ID, map[string]any{
		"uri":   uri,
		"range": map[string]any{"start": map[string]int{"line": 5, "character": 1}, "end": map[string]int{"line": 5, "character": 4}},
	})
	refResult := <-refDone
	if refResult.err != nil || len(refResult.locations) != 1 || refResult.locations[0].URI != uri {
		t.Fatalf("references = %#v, err %v", refResult.locations, refResult.err)
	}

	formatDone := make(chan struct {
		edits []TextEdit
		err   error
	}, 1)
	go func() {
		edits, err := client.Format(context.Background(), uri)
		formatDone <- struct {
			edits []TextEdit
			err   error
		}{edits, err}
	}()
	req = server.next(t)
	if req.Method != "textDocument/formatting" {
		t.Fatalf("method = %q", req.Method)
	}
	server.respond(req.ID, []any{map[string]any{
		"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 1, "character": 0}},
		"newText": "package main\n",
	}})
	formatResult := <-formatDone
	if formatResult.err != nil || len(formatResult.edits) != 1 || formatResult.edits[0].NewText != "package main\n" {
		t.Fatalf("format = %#v, err %v", formatResult.edits, formatResult.err)
	}
}

func TestCodeActionsAndResolve(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))
	rng := Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 4}}
	diags := []Diagnostic{{
		Range: rng, Severity: 1, Message: "bad", Source: "fake",
		Code: json.RawMessage(`"E1"`), Data: json.RawMessage(`{"fix":1}`),
	}}

	type actionResult struct {
		actions []CodeAction
		err     error
	}
	done := make(chan actionResult, 1)
	go func() {
		actions, err := client.CodeActions(context.Background(), uri, rng, diags)
		done <- actionResult{actions, err}
	}()
	req := server.next(t)
	if req.Method != "textDocument/codeAction" {
		t.Fatalf("method = %q", req.Method)
	}
	var params struct {
		Range   Range `json:"range"`
		Context struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if len(params.Context.Diagnostics) != 1 || string(params.Context.Diagnostics[0].Data) != `{"fix":1}` {
		t.Fatalf("codeAction diagnostics = %#v", params.Context.Diagnostics)
	}

	server.respond(req.ID, []any{
		map[string]any{
			"title": "Fix bad thing",
			"kind":  "quickfix",
			"diagnostics": []any{map[string]any{
				"range":    map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 4}},
				"severity": 1, "message": "bad", "data": map[string]int{"fix": 1},
			}},
			"edit": map[string]any{
				"documentChanges": []any{map[string]any{
					"textDocument": map[string]string{"uri": uri, "version": "1"},
					"edits": []any{map[string]any{
						"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 4}},
						"newText": "good",
					}},
				}},
			},
			"data": map[string]int{"fix": 1},
		},
		map[string]any{
			"title":     "Run legacy command",
			"command":   "fake.fix",
			"arguments": []any{map[string]int{"fix": 1}},
		},
	})
	result := <-done
	if result.err != nil {
		t.Fatalf("CodeActions: %v", result.err)
	}
	if len(result.actions) != 2 {
		t.Fatalf("actions = %#v", result.actions)
	}
	first := result.actions[0]
	if first.Edit == nil || len(first.Edit.Changes[uri]) != 1 || first.Edit.Changes[uri][0].NewText != "good" {
		t.Fatalf("workspace edit = %#v", first.Edit)
	}
	if len(first.Diagnostics) != 1 || string(first.Diagnostics[0].Data) != `{"fix":1}` {
		t.Fatalf("action diagnostics = %#v", first.Diagnostics)
	}
	legacy := result.actions[1]
	if legacy.Command == nil || legacy.Command.Command != "fake.fix" || legacy.Command.Arguments == nil {
		t.Fatalf("legacy command action = %#v", legacy)
	}

	resolveDone := make(chan actionResult, 1)
	go func() {
		action, err := client.ResolveCodeAction(context.Background(), first)
		resolveDone <- actionResult{[]CodeAction{action}, err}
	}()
	req = server.next(t)
	if req.Method != "codeAction/resolve" {
		t.Fatalf("method = %q", req.Method)
	}
	if !bytes.Contains(req.Params, []byte(`"data":{"fix":1}`)) {
		t.Fatalf("resolve params lost data: %s", req.Params)
	}
	server.respond(req.ID, map[string]any{
		"title": "Fix bad thing",
		"edit": map[string]any{"changes": map[string]any{
			uri: []any{map[string]any{
				"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 4}},
				"newText": "resolved",
			}},
		}},
	})
	resolved := <-resolveDone
	if resolved.err != nil || len(resolved.actions) != 1 ||
		resolved.actions[0].Edit == nil || resolved.actions[0].Edit.Changes[uri][0].NewText != "resolved" {
		t.Fatalf("resolved = %#v, err %v", resolved.actions, resolved.err)
	}
}

func TestConcurrentRequestsWithDistinctIDs(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	type completionResult struct {
		items []CompletionItem
		err   error
	}
	type hoverResult struct {
		hover *Hover
		err   error
	}
	completionDone := make(chan completionResult, 1)
	hoverDone := make(chan hoverResult, 1)
	go func() {
		items, err := client.Completion(context.Background(), uri, Position{Line: 1, Character: 1})
		completionDone <- completionResult{items, err}
	}()
	go func() {
		hover, err := client.Hover(context.Background(), uri, Position{Line: 2, Character: 2})
		hoverDone <- hoverResult{hover, err}
	}()

	byMethod := map[string]rpcMessage{}
	for len(byMethod) < 2 {
		msg := server.next(t)
		byMethod[msg.Method] = msg
	}
	completionReq := byMethod["textDocument/completion"]
	hoverReq := byMethod["textDocument/hover"]
	if len(completionReq.ID) == 0 || len(hoverReq.ID) == 0 || string(completionReq.ID) == string(hoverReq.ID) {
		t.Fatalf("ids are not distinct: %s / %s", completionReq.ID, hoverReq.ID)
	}

	// Answer in reverse order to prove id multiplexing.
	server.respond(hoverReq.ID, map[string]any{"contents": "hover answer"})
	server.respond(completionReq.ID, []any{map[string]any{"label": "first"}})

	cr := <-completionDone
	hr := <-hoverDone
	if cr.err != nil || len(cr.items) != 1 || cr.items[0].Label != "first" {
		t.Fatalf("completion = %#v, err %v", cr.items, cr.err)
	}
	if hr.err != nil || hr.hover == nil || hr.hover.Contents != "hover answer" {
		t.Fatalf("hover = %#v, err %v", hr.hover, hr.err)
	}
}

func TestBrokenJSONDoesNotKillClient(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	type result struct {
		items []CompletionItem
		err   error
	}
	done := make(chan result, 1)
	go func() {
		items, err := client.Completion(context.Background(), uri, Position{})
		done <- result{items, err}
	}()
	req := server.next(t)
	if req.Method != "textDocument/completion" {
		t.Fatalf("method = %q", req.Method)
	}
	server.sendRaw([]byte(`{"jsonrpc":"2.0", this is not json`))
	server.respond(req.ID, []any{map[string]any{"label": "still alive"}})

	got := <-done
	if got.err != nil || len(got.items) != 1 || got.items[0].Label != "still alive" {
		t.Fatalf("after broken JSON = %#v, err %v", got.items, got.err)
	}
	if client.Diagnostics(uri) != nil {
		t.Fatal("unexpected diagnostics")
	}
}

func TestEOFClosesPendingCallsAndClient(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Completion(context.Background(), uri, Position{})
		errCh <- err
	}()
	if req := server.next(t); req.Method != "textDocument/completion" {
		t.Fatalf("method = %q", req.Method)
	}
	server.close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("pending request succeeded after EOF")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending request did not fail on EOF")
	}
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("Start after EOF returned nil")
	}
	if _, err := client.Completion(context.Background(), uri, Position{}); err == nil {
		t.Fatal("request after EOF returned nil error")
	}
}

func TestCancelRemovesPending(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := client.Completion(ctx, uri, Position{})
		errCh <- err
	}()
	req := server.next(t)
	if req.Method != "textDocument/completion" {
		t.Fatalf("method = %q", req.Method)
	}
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}

	client.mu.Lock()
	pending := len(client.pending)
	client.mu.Unlock()
	if pending != 0 {
		t.Fatalf("pending map has %d entries after cancel", pending)
	}

	// A late response must be ignored, and the client must stay usable.
	server.respond(req.ID, []any{map[string]any{"label": "late"}})

	hoverCh := make(chan error, 1)
	go func() {
		_, err := client.Hover(context.Background(), uri, Position{})
		hoverCh <- err
	}()
	hoverReq := server.next(t)
	if hoverReq.Method != "textDocument/hover" {
		t.Fatalf("method = %q", hoverReq.Method)
	}
	server.respond(hoverReq.ID, map[string]any{"contents": "ok"})
	if err := <-hoverCh; err != nil {
		t.Fatalf("hover after cancel: %v", err)
	}
}

func TestMultipleFramesInOneWrite(t *testing.T) {
	client, server := newTestClient(t)
	uriA := PathToURI(filepath.Join(client.root, "a.md"))
	uriB := PathToURI(filepath.Join(client.root, "b.md"))

	got := make(chan string, 2)
	client.SetDiagnosticsHandler(func(uri string, diags []Diagnostic) {
		if len(diags) > 0 {
			got <- uri + ":" + diags[0].Message
		}
	})

	frameA, _ := json.Marshal(rpcNotification{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: map[string]any{
			"uri": uriA, "version": 1,
			"diagnostics": []any{map[string]any{
				"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
				"message": "first",
			}},
		},
	})
	frameB, _ := json.Marshal(rpcNotification{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: map[string]any{
			"uri": uriB, "version": 1,
			"diagnostics": []any{map[string]any{
				"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
				"message": "second",
			}},
		},
	})
	server.sendBatch(frameA, frameB)

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case value := <-got:
			seen[value] = true
		case <-time.After(3 * time.Second):
			t.Fatalf("diagnostics batch delivered %d/2 messages", i)
		}
	}
	if !seen[uriA+":first"] || !seen[uriB+":second"] {
		t.Fatalf("batch results = %#v", seen)
	}
}

func TestApplyEditFromServerDoesNotBlockReader(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	entered := make(chan struct{})
	release := make(chan struct{})
	applied := make(chan WorkspaceEdit, 1)
	client.SetApplyEditHandler(func(edit WorkspaceEdit) {
		close(entered)
		<-release
		applied <- edit
	})

	server.request(99, "workspace/applyEdit", map[string]any{
		"edit": map[string]any{"changes": map[string]any{
			uri: []any{map[string]any{
				"range":   map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
				"newText": "x",
			}},
		}},
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("applyEdit handler was not called")
	}

	// While the handler is blocked, ordinary requests must still work.
	hoverCh := make(chan error, 1)
	go func() {
		hover, err := client.Hover(context.Background(), uri, Position{})
		if err == nil && (hover == nil || hover.Contents != "alive") {
			err = errors.New("unexpected hover")
		}
		hoverCh <- err
	}()
	hoverReq := server.next(t)
	if hoverReq.Method != "textDocument/hover" {
		t.Fatalf("method = %q", hoverReq.Method)
	}
	server.respond(hoverReq.ID, map[string]any{"contents": "alive"})
	if err := <-hoverCh; err != nil {
		t.Fatalf("hover while applyEdit blocked: %v", err)
	}

	close(release)
	select {
	case edit := <-applied:
		if len(edit.Changes[uri]) != 1 || edit.Changes[uri][0].NewText != "x" {
			t.Fatalf("applied edit = %#v", edit)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("applyEdit handler did not finish")
	}
	response := server.next(t)
	if id, ok := messageID(response.ID); !ok || id != 99 {
		t.Fatalf("applyEdit response id = %s", response.ID)
	}
	var result struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil || !result.Applied {
		t.Fatalf("applyEdit result = %s (%v)", response.Result, err)
	}
}

func TestApplyWorkspaceEditRequest(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))
	edit := WorkspaceEdit{Changes: map[string][]TextEdit{
		uri: {{Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}}, NewText: "x"}},
	}}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.ApplyWorkspaceEdit(context.Background(), edit)
	}()
	req := server.next(t)
	if req.Method != "workspace/applyEdit" {
		t.Fatalf("method = %q", req.Method)
	}
	if !bytes.Contains(req.Params, []byte(`"newText":"x"`)) {
		t.Fatalf("params = %s", req.Params)
	}
	server.respond(req.ID, map[string]any{"applied": true})
	if err := <-errCh; err != nil {
		t.Fatalf("ApplyWorkspaceEdit: %v", err)
	}
}

func TestInitializeRespectsContext(t *testing.T) {
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	server := newFakeServer(serverIn, serverOut)
	defer server.close()
	client := NewClientWithStreams(ServerConfig{Command: "fake-lsp"}, "", clientIn, clientOut)
	defer client.Close()
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := client.Initialize(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Initialize error = %v, want deadline exceeded", err)
	}
	if req := server.next(t); req.Method != "initialize" {
		t.Fatalf("method = %q", req.Method)
	}
	client.mu.Lock()
	pending := len(client.pending)
	client.mu.Unlock()
	if pending != 0 {
		t.Fatalf("pending map has %d entries after timeout", pending)
	}
}

func TestCloseFailsPendingCalls(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Completion(context.Background(), uri, Position{})
		errCh <- err
	}()
	if req := server.next(t); req.Method != "textDocument/completion" {
		t.Fatalf("method = %q", req.Method)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := <-errCh; !errors.Is(err, ErrClosed) {
		t.Fatalf("pending request error = %v, want ErrClosed", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := client.Completion(context.Background(), uri, Position{}); err == nil {
		t.Fatal("request after Close returned nil error")
	}
}

func TestServerCapabilities(t *testing.T) {
	client, _ := newTestClientWithResult(t, map[string]any{
		"capabilities": map[string]any{
			"hoverProvider":              true,
			"completionProvider":         map[string]any{"triggerCharacters": []string{"."}},
			"definitionProvider":         false,
			"referencesProvider":         map[string]any{},
			"documentFormattingProvider": nil,
			"codeActionProvider":         map[string]any{"codeActionKinds": []string{"quickfix"}},
			"renameProvider":             true,
		},
	})

	want := ServerCapabilities{
		HoverProvider:      true,
		CompletionProvider: true,
		ReferencesProvider: true,
		CodeActionProvider: true,
	}
	if got := client.Capabilities(); got != want {
		t.Fatalf("Capabilities() = %#v, want %#v", got, want)
	}

	features := map[string]bool{
		"hover":      true,
		"completion": true,
		"definition": false,
		"references": true,
		"format":     false,
		"codeAction": true,
		"rename":     false,
	}
	for feature, want := range features {
		if got := client.Supports(feature); got != want {
			t.Errorf("Supports(%q) = %v, want %v", feature, got, want)
		}
	}
}

func TestServerCapabilitiesAbsent(t *testing.T) {
	client, _ := newTestClientWithResult(t, map[string]any{})
	if got := client.Capabilities(); got != (ServerCapabilities{}) {
		t.Fatalf("Capabilities() = %#v, want zero value", got)
	}
	for _, feature := range []string{"hover", "completion", "definition", "references", "format", "codeAction"} {
		if client.Supports(feature) {
			t.Errorf("Supports(%q) = true without declared capabilities", feature)
		}
	}
}

func TestHoverRetriesOnInternalError(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	type hoverResult struct {
		hover *Hover
		err   error
	}
	done := make(chan hoverResult, 1)
	go func() {
		hover, err := client.Hover(context.Background(), uri, Position{Line: 1, Character: 2})
		done <- hoverResult{hover, err}
	}()

	first := server.next(t)
	if first.Method != "textDocument/hover" {
		t.Fatalf("first method = %q", first.Method)
	}
	server.sendValue(rpcErrorResponse{
		JSONRPC: "2.0",
		ID:      first.ID,
		Error:   &rpcError{Code: -32603, Message: "Internal error"},
	})

	retry := server.next(t)
	if retry.Method != "textDocument/hover" {
		t.Fatalf("retry method = %q", retry.Method)
	}
	if string(retry.ID) == string(first.ID) {
		t.Fatalf("retry reused request id %s", retry.ID)
	}
	server.respond(retry.ID, map[string]any{"contents": "after retry"})

	got := <-done
	if got.err != nil {
		t.Fatalf("Hover: %v", got.err)
	}
	if got.hover == nil || got.hover.Contents != "after retry" {
		t.Fatalf("hover = %#v", got.hover)
	}
}

func TestCompletionRetriesOnInternalError(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	type completionResult struct {
		items []CompletionItem
		err   error
	}
	done := make(chan completionResult, 1)
	go func() {
		items, err := client.Completion(context.Background(), uri, Position{Line: 2, Character: 4})
		done <- completionResult{items, err}
	}()

	first := server.next(t)
	if first.Method != "textDocument/completion" {
		t.Fatalf("first method = %q", first.Method)
	}
	server.sendValue(rpcErrorResponse{
		JSONRPC: "2.0",
		ID:      first.ID,
		Error:   &rpcError{Code: -32603, Message: "Internal error"},
	})

	retry := server.next(t)
	if retry.Method != "textDocument/completion" {
		t.Fatalf("retry method = %q", retry.Method)
	}
	if string(retry.ID) == string(first.ID) {
		t.Fatalf("retry reused request id %s", retry.ID)
	}
	server.respond(retry.ID, []any{map[string]any{"label": "after retry"}})

	got := <-done
	if got.err != nil {
		t.Fatalf("Completion: %v", got.err)
	}
	if len(got.items) != 1 || got.items[0].Label != "after retry" {
		t.Fatalf("items = %#v", got.items)
	}
}

func TestInternalErrorRetryIsBounded(t *testing.T) {
	client, server := newTestClient(t)
	uri := PathToURI(filepath.Join(client.root, "main.go"))

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Completion(context.Background(), uri, Position{})
		errCh <- err
	}()

	first := server.next(t)
	if first.Method != "textDocument/completion" {
		t.Fatalf("first method = %q", first.Method)
	}
	server.sendValue(rpcErrorResponse{
		JSONRPC: "2.0",
		ID:      first.ID,
		Error:   &rpcError{Code: -32603, Message: "Internal error"},
	})

	retry := server.next(t)
	if retry.Method != "textDocument/completion" {
		t.Fatalf("retry method = %q", retry.Method)
	}
	server.sendValue(rpcErrorResponse{
		JSONRPC: "2.0",
		ID:      retry.ID,
		Error:   &rpcError{Code: -32603, Message: "Internal error"},
	})

	if err := <-errCh; err == nil {
		t.Fatal("Completion returned nil after two internal errors")
	}

	// A third request would mean the retry loop is not bounded.
	select {
	case msg := <-server.msgs:
		t.Fatalf("unexpected request after the single retry: %q", msg.Method)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestIsInternalError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"code", &rpcError{Code: -32603, Message: "request failed"}, true},
		{"message", &rpcError{Code: -32000, Message: "Internal error"}, true},
		{"method not found", &rpcError{Code: -32601, Message: "method not found"}, false},
		{"context canceled", context.Canceled, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := isInternalError(tc.err); got != tc.want {
			t.Errorf("isInternalError(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStartErrors(t *testing.T) {
	client := NewClient(ServerConfig{Command: filepath.Join(t.TempDir(), "missing-lsp")}, t.TempDir())
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("Start with missing command returned nil")
	}
	_ = client.Close()
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("Start after Close returned nil")
	}
}
