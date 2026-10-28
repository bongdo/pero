package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrClosed     = errors.New("lsp: client is closed")
	ErrNotStarted = errors.New("lsp: client is not started")
)

const (
	// ltex-ls loads LanguageTool on initialize and needs well over 10s on a
	// cold machine; a dead server is still bounded by the caller's context.
	startupTimeout = 30 * time.Second
	// closeTimeout bounds shutdown/exit and the final process wait. A wedged
	// server must never block editor quit.
	closeTimeout = 2 * time.Second
	// retryDelay is the pause before the single internal-error retry of hover
	// and completion. A didOpen sent just before the request may still sit in
	// the server's queue; gopls answers -32603 in that window.
	retryDelay = 150 * time.Millisecond
)

type pendingCall struct {
	ch chan rpcMessage
}

type diagnosticsEntry struct {
	diags      []Diagnostic
	version    int
	hasVersion bool
}

type Client struct {
	cfg  ServerConfig
	root string

	stream bool
	in     io.Reader
	out    io.Writer

	writeMu sync.Mutex
	startMu sync.Mutex
	initMu  sync.Mutex

	mu          sync.Mutex
	started     bool
	initialized bool
	closed      bool
	closeErr    error
	nextID      int64
	pending     map[int64]*pendingCall
	diagnostics map[string]diagnosticsEntry
	diagHandler func(uri string, diags []Diagnostic)
	// diagVersionHandler получает версию документа, если сервер её прислал.
	// Нужен UI, чтобы не рисовать диагностики от старой версии.
	diagVersionHandler func(uri string, version int, hasVersion bool, diags []Diagnostic)
	applyHandler       func(edit WorkspaceEdit)
	settings           map[string]any
	settingsFunc       func(section string) any
	capabilities       ServerCapabilities

	cmd    *exec.Cmd
	ownIn  io.Closer
	ownOut io.Closer

	done      chan struct{}
	closeOnce sync.Once
}

func NewClient(cfg ServerConfig, root string) *Client {
	return &Client{
		cfg:         cfg,
		root:        root,
		pending:     make(map[int64]*pendingCall),
		diagnostics: make(map[string]diagnosticsEntry),
		done:        make(chan struct{}),
	}
}

// NewClientWithStreams creates a client over an existing transport. Close
// closes in/out when they implement io.Closer so the reader goroutine can
// always be stopped.
func NewClientWithStreams(cfg ServerConfig, root string, in io.Reader, out io.Writer) *Client {
	c := NewClient(cfg, root)
	c.stream = true
	c.in = in
	c.out = out
	if closer, ok := in.(io.Closer); ok {
		c.ownIn = closer
	}
	if closer, ok := out.(io.Closer); ok {
		c.ownOut = closer
	}
	return c
}

// Start launches the server process (or starts the reader for stream clients).
// It never waits for initialize; call Initialize afterwards. A second Start on
// a live client is a no-op; after Close or a crash it returns an error.
func (c *Client) Start(ctx context.Context) error {
	c.startMu.Lock()
	defer c.startMu.Unlock()

	ctx, cancel := withTimeout(ctx, startupTimeout)
	defer cancel()

	c.mu.Lock()
	if c.closed {
		err := c.closedErrLocked()
		c.mu.Unlock()
		return err
	}
	if c.started {
		c.mu.Unlock()
		return nil
	}
	stream := c.stream
	c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	var reader io.Reader
	if stream {
		c.mu.Lock()
		reader = c.in
		hasOut := c.out != nil
		c.mu.Unlock()
		if reader == nil || !hasOut {
			return errors.New("lsp: no input/output stream")
		}
	} else {
		if c.cfg.Command == "" {
			return errors.New("lsp: empty server command")
		}
		cmd := exec.Command(c.cfg.Command, c.cfg.Args...)
		if c.root != "" {
			cmd.Dir = c.root
		}
		// shortcut: server stderr is discarded, add a ring buffer if startup
		// failures need more diagnostics in the status line.
		cmd.Stderr = io.Discard
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("lsp: stdin pipe: %w", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			stdin.Close()
			return fmt.Errorf("lsp: stdout pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			stdin.Close()
			stdout.Close()
			return fmt.Errorf("lsp: start %s: %w", c.cfg.Command, err)
		}
		reader = stdout

		c.mu.Lock()
		if c.closed {
			err := c.closedErrLocked()
			c.mu.Unlock()
			cmd.Process.Kill()
			cmd.Wait()
			return err
		}
		c.cmd = cmd
		c.in = stdout
		c.out = stdin
		c.ownIn = stdout
		c.ownOut = stdin
		c.started = true
		c.mu.Unlock()

		go c.waitProcess(cmd)
	}

	c.mu.Lock()
	if c.closed {
		err := c.closedErrLocked()
		c.mu.Unlock()
		return err
	}
	if !stream {
		// already marked started above
	} else {
		c.started = true
	}
	c.mu.Unlock()

	go c.readLoop(bufio.NewReader(reader))
	return nil
}

func (c *Client) waitProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err == nil {
		err = errors.New("server exited")
	}
	c.finish(fmt.Errorf("lsp: server exited: %w", err))
}

func (c *Client) readLoop(r *bufio.Reader) {
	for {
		body, err := readFrame(r)
		if err != nil {
			c.finish(fmt.Errorf("lsp: read: %w", err))
			return
		}
		var msg rpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			// A broken message is isolated to its frame; keep the client alive.
			continue
		}
		c.handleMessage(msg)
	}
}

func (c *Client) handleMessage(msg rpcMessage) {
	if msg.Method == "" {
		id, ok := messageID(msg.ID)
		if !ok {
			return
		}
		c.mu.Lock()
		p := c.pending[id]
		if p != nil {
			delete(c.pending, id)
		}
		c.mu.Unlock()
		if p != nil {
			p.ch <- msg
		}
		return
	}
	if len(bytes.TrimSpace(msg.ID)) > 0 && string(bytes.TrimSpace(msg.ID)) != "null" {
		go c.answerServerRequest(msg)
		return
	}
	c.handleNotification(msg)
}

func (c *Client) handleNotification(msg rpcMessage) {
	switch msg.Method {
	case "textDocument/publishDiagnostics":
		c.storeDiagnostics(msg.Params)
	default:
		// window/logMessage, $/progress and friends are intentionally ignored.
	}
}

func (c *Client) storeDiagnostics(raw json.RawMessage) {
	var params struct {
		URI         string       `json:"uri"`
		Version     *int         `json:"version"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.URI == "" {
		return
	}

	c.mu.Lock()
	if old, ok := c.diagnostics[params.URI]; ok && params.Version != nil && old.hasVersion && *params.Version < old.version {
		c.mu.Unlock()
		return
	}
	entry := diagnosticsEntry{diags: append([]Diagnostic(nil), params.Diagnostics...)}
	if params.Version != nil {
		entry.version = *params.Version
		entry.hasVersion = true
	}
	c.diagnostics[params.URI] = entry
	handler := c.diagHandler
	versionHandler := c.diagVersionHandler
	c.mu.Unlock()

	if versionHandler != nil {
		diags := append([]Diagnostic(nil), params.Diagnostics...)
		version, hasVersion := 0, false
		if params.Version != nil {
			version, hasVersion = *params.Version, true
		}
		go versionHandler(params.URI, version, hasVersion, diags)
		return
	}
	if handler != nil {
		diags := append([]Diagnostic(nil), params.Diagnostics...)
		go handler(params.URI, diags)
	}
}

func (c *Client) answerServerRequest(msg rpcMessage) {
	switch msg.Method {
	case "workspace/applyEdit":
		var params struct {
			Edit WorkspaceEdit `json:"edit"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			c.respond(msg.ID, nil, &rpcError{Code: -32602, Message: "invalid workspace/applyEdit params"})
			return
		}
		c.mu.Lock()
		handler := c.applyHandler
		c.mu.Unlock()
		applied := handler != nil
		if handler != nil {
			handler(params.Edit)
		}
		result := map[string]any{"applied": applied}
		if !applied {
			result["failureReason"] = "no applyEdit handler configured"
		}
		c.respond(msg.ID, result, nil)
	case "workspace/configuration":
		var params struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		values := make([]any, len(params.Items))
		for i, raw := range params.Items {
			var item struct {
				Section string `json:"section"`
			}
			if err := json.Unmarshal(raw, &item); err == nil {
				value := c.settingForSection(item.Section)
				// A handler can return anything; never let one bad value make
				// the whole response unserializable and hang the server.
				if _, err := json.Marshal(value); err != nil {
					value = nil
				}
				values[i] = value
			}
		}
		c.respond(msg.ID, values, nil)
	case "workspace/workspaceFolders":
		folders := c.workspaceFolders()
		if folders == nil {
			c.respond(msg.ID, json.RawMessage("null"), nil)
		} else {
			c.respond(msg.ID, folders, nil)
		}
	case "client/registerCapability", "client/unregisterCapability",
		"window/workDoneProgress/create", "window/showMessageRequest":
		c.respond(msg.ID, json.RawMessage("null"), nil)
	default:
		c.respond(msg.ID, nil, &rpcError{Code: -32601, Message: "method not found: " + msg.Method})
	}
}

func (c *Client) workspaceFolders() []map[string]string {
	if c.root == "" {
		return nil
	}
	uri := PathToURI(c.root)
	name := filepath.Base(c.root)
	if name == "." || name == string(filepath.Separator) {
		name = c.root
	}
	return []map[string]string{{"uri": uri, "name": name}}
}

func (c *Client) respond(id json.RawMessage, result any, rpcErr *rpcError) {
	var (
		payload []byte
		err     error
	)
	if rpcErr != nil {
		payload, err = json.Marshal(rpcErrorResponse{JSONRPC: "2.0", ID: id, Error: rpcErr})
	} else {
		if result == nil {
			result = json.RawMessage("null")
		}
		payload, err = json.Marshal(rpcResultResponse{JSONRPC: "2.0", ID: id, Result: result})
	}
	if err != nil {
		return
	}
	c.writePayload(payload)
}

func (c *Client) writePayload(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.Lock()
	out := c.out
	closed := c.closed
	c.mu.Unlock()
	if closed || out == nil {
		return ErrClosed
	}
	return writeFrame(out, payload)
}

func (c *Client) Initialize(ctx context.Context) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()

	ctx, cancel := withTimeout(ctx, startupTimeout)
	defer cancel()

	c.mu.Lock()
	if c.closed {
		err := c.closedErrLocked()
		c.mu.Unlock()
		return err
	}
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	raw, err := c.call(ctx, "initialize", c.initializeParams())
	if err != nil {
		return err
	}
	caps := decodeServerCapabilities(raw)
	if err := c.notify("initialized", struct{}{}); err != nil {
		return err
	}

	c.mu.Lock()
	c.capabilities = caps
	c.initialized = true
	c.mu.Unlock()
	return nil
}

// Capabilities returns the capabilities declared by the server in its
// initialize response. Before a successful Initialize every field is false.
func (c *Client) Capabilities() ServerCapabilities {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.capabilities
}

// Supports reports whether the server explicitly advertised a feature.
// feature is "hover", "completion", "definition", "references", "format" or
// "codeAction"; any other value is false. A server that sends no capabilities
// object advertises nothing, so callers should hide the feature rather than
// send a request. Request methods still work when Supports is false.
func (c *Client) Supports(feature string) bool {
	caps := c.Capabilities()
	switch feature {
	case "hover":
		return caps.HoverProvider
	case "completion":
		return caps.CompletionProvider
	case "definition":
		return caps.DefinitionProvider
	case "references":
		return caps.ReferencesProvider
	case "format":
		return caps.DocumentFormattingProvider
	case "codeAction":
		return caps.CodeActionProvider
	default:
		return false
	}
}

func (c *Client) initializeParams() map[string]any {
	params := map[string]any{
		"processId": os.Getpid(),
		"clientInfo": map[string]string{
			"name":    "pero",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"applyEdit": true,
				"workspaceEdit": map[string]any{
					"documentChanges": false,
				},
				"workspaceFolders": true,
				"configuration":    true,
			},
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": false,
					"willSave":            false,
					"didSave":             true,
				},
				"completion": map[string]any{
					"dynamicRegistration": false,
					"completionItem": map[string]any{
						"snippetSupport":      false,
						"documentationFormat": []string{"markdown", "plaintext"},
					},
				},
				"hover": map[string]any{
					"dynamicRegistration": false,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"definition": map[string]any{
					"dynamicRegistration": false,
					"linkSupport":         true,
				},
				"references": map[string]any{
					"dynamicRegistration": false,
				},
				"formatting": map[string]any{
					"dynamicRegistration": false,
				},
				"codeAction": map[string]any{
					"dynamicRegistration": false,
					"codeActionLiteralSupport": map[string]any{
						"codeActionKind": map[string]any{
							"valueSet": []string{"", "quickfix", "refactor", "source"},
						},
					},
					"resolveSupport": map[string]any{
						"properties": []string{"edit"},
					},
					"dataSupport": true,
				},
				"publishDiagnostics": map[string]any{
					"versionSupport": true,
				},
			},
			"window": map[string]any{
				"workDoneProgress": true,
			},
			"general": map[string]any{
				"positionEncodings": []string{"utf-16"},
			},
		},
	}
	if c.root != "" {
		uri := PathToURI(c.root)
		params["rootUri"] = uri
		params["rootPath"] = c.root
		params["workspaceFolders"] = c.workspaceFolders()
	} else {
		params["rootUri"] = nil
		params["rootPath"] = nil
	}
	if c.cfg.InitOptions != nil {
		params["initializationOptions"] = c.cfg.InitOptions
	}
	return params
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.closed {
		err := c.closedErrLocked()
		c.mu.Unlock()
		return nil, err
	}
	if !c.started {
		c.mu.Unlock()
		return nil, ErrNotStarted
	}
	c.nextID++
	id := c.nextID
	p := &pendingCall{ch: make(chan rpcMessage, 1)}
	c.pending[id] = p
	c.mu.Unlock()

	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		c.removePending(id)
		return nil, err
	}
	if err := c.writePayload(payload); err != nil {
		c.removePending(id)
		c.finish(fmt.Errorf("lsp: write: %w", err))
		return nil, err
	}

	select {
	case msg := <-p.ch:
		if msg.err != nil {
			return nil, msg.err
		}
		if msg.Error != nil {
			return nil, msg.Error
		}
		return msg.Result, nil
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	case <-c.done:
		c.removePending(id)
		return nil, c.closedErr()
	}
}

func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	if c.closed {
		err := c.closedErrLocked()
		c.mu.Unlock()
		return err
	}
	if !c.started {
		c.mu.Unlock()
		return ErrNotStarted
	}
	c.mu.Unlock()

	payload, err := json.Marshal(rpcNotification{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	if err := c.writePayload(payload); err != nil {
		c.finish(fmt.Errorf("lsp: write: %w", err))
		return err
	}
	return nil
}

func (c *Client) removePending(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// callRetryInternal retries a request once after retryDelay when the server
// reported an internal error. The editor may send hover/completion before the
// server has consumed the preceding didOpen, and gopls answers -32603 in that
// window. Context errors, a closed client and unsupported methods are not
// retried, so a real failure still surfaces immediately.
func (c *Client) callRetryInternal(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	raw, err := c.call(ctx, method, params)
	if !isInternalError(err) {
		return raw, err
	}
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.closedErr()
	case <-timer.C:
	}
	return c.call(ctx, method, params)
}

func isInternalError(err error) bool {
	var rpcErr *rpcError
	if !errors.As(err, &rpcErr) {
		return false
	}
	return rpcErr.Code == -32603 || strings.Contains(strings.ToLower(rpcErr.Message), "internal error")
}

func (c *Client) closedErrLocked() error {
	if c.closeErr != nil {
		return c.closeErr
	}
	return ErrClosed
}

func (c *Client) closedErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closedErrLocked()
}

func (c *Client) alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started && !c.closed
}

// finish closes the client exactly once: pending calls receive the reason,
// streams/process are torn down and the reader is expected to unblock.
func (c *Client) finish(err error) {
	c.closeOnce.Do(func() {
		if err == nil {
			err = ErrClosed
		}
		c.mu.Lock()
		c.closed = true
		c.closeErr = err
		pending := c.pending
		c.pending = make(map[int64]*pendingCall)
		cmd := c.cmd
		in := c.ownIn
		out := c.ownOut
		c.mu.Unlock()

		close(c.done)

		if in != nil {
			_ = in.Close()
		}
		if out != nil {
			_ = out.Close()
		}
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}

		for _, p := range pending {
			p.ch <- rpcMessage{err: err, Error: &rpcError{Code: -32803, Message: err.Error()}}
		}
	})
}

func (c *Client) Close() error {
	c.shutdown()
	c.finish(ErrClosed)
	return nil
}

// shutdown asks a live server to stop (LSP shutdown then exit), bounded by
// closeTimeout. finish still closes streams and kills a stuck process.
func (c *Client) shutdown() {
	c.mu.Lock()
	started := c.started
	initialized := c.initialized
	closed := c.closed
	hasProcess := c.cmd != nil && c.cmd.Process != nil
	c.mu.Unlock()

	if closed || !started {
		return
	}
	if initialized {
		ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		_, _ = c.call(ctx, "shutdown", nil)
		cancel()
	}
	_ = c.notify("exit", nil)
	if !hasProcess {
		return
	}
	timer := time.NewTimer(closeTimeout)
	defer timer.Stop()
	select {
	case <-c.done:
	case <-timer.C:
	}
}

func (c *Client) DidOpen(uri, languageID string, version int, text string) {
	_ = c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    version,
			"text":       text,
		},
	})
}

func (c *Client) DidChange(uri string, version int, text string) {
	_ = c.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]string{{"text": text}},
	})
}

func (c *Client) DidSave(uri string) {
	_ = c.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

func (c *Client) DidClose(uri string) {
	_ = c.notify("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

func (c *Client) Diagnostics(uri string) []Diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.diagnostics[uri]
	if !ok {
		return nil
	}
	return append([]Diagnostic(nil), entry.diags...)
}

func (c *Client) SetDiagnosticsHandler(handler func(uri string, diags []Diagnostic)) {
	c.mu.Lock()
	c.diagHandler = handler
	c.mu.Unlock()
}

// SetDiagnosticsHandlerVersioned is like SetDiagnosticsHandler, but also passes
// the document version from publishDiagnostics when the server provides one.
// The versioned handler takes precedence if both are set.
func (c *Client) SetDiagnosticsHandlerVersioned(handler func(uri string, version int, hasVersion bool, diags []Diagnostic)) {
	c.mu.Lock()
	c.diagVersionHandler = handler
	c.mu.Unlock()
}

// SetApplyEditHandler sets the callback for workspace/applyEdit. The client
// replies {"applied": true} only after the handler returns; the handler runs
// outside the reader loop but blocks that one server request.
func (c *Client) SetApplyEditHandler(handler func(edit WorkspaceEdit)) {
	c.mu.Lock()
	c.applyHandler = handler
	c.mu.Unlock()
}

// SetConfiguration sets the values returned to the server's
// workspace/configuration requests. Keys are configuration sections, e.g.
// {"ltex": {"language": "ru-RU"}} or {"harper-ls": {"dialect": "British"}}.
// An empty section returns the whole map; an unknown section returns nil.
// Call it before Initialize so servers that ask during startup see the values.
func (c *Client) SetConfiguration(config map[string]any) {
	copied := cloneSettings(config)
	c.mu.Lock()
	c.settings = copied
	c.mu.Unlock()
}

// cloneSettings deep-copies through JSON so nested maps can be normalized and
// later mutation of the caller's map cannot race with server requests.
func cloneSettings(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil
	}
	var copied map[string]any
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil
	}
	return copied
}

// SetConfigurationHandler overrides the map for workspace/configuration.
// It runs on a per-request goroutine; a nil result falls back to
// SetConfiguration.
func (c *Client) SetConfigurationHandler(handler func(section string) any) {
	c.mu.Lock()
	c.settingsFunc = handler
	c.mu.Unlock()
}

func (c *Client) settingForSection(section string) any {
	c.mu.Lock()
	handler := c.settingsFunc
	settings := c.settings
	c.mu.Unlock()

	if handler != nil {
		if value := handler(section); value != nil {
			return value
		}
	}
	if settings == nil {
		return nil
	}
	section = strings.TrimSpace(section)
	if section == "" {
		return settings
	}
	if value, ok := settings[section]; ok {
		return value
	}
	// Also accept dotted sections like "ltex.language".
	var current any = settings
	for _, part := range strings.Split(section, ".") {
		values, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = values[part]
		if !ok {
			return nil
		}
	}
	return current
}

// ExecuteCommand runs a workspace/executeCommand request. args is the raw
// "arguments" array expected by the server; nil omits the field.
func (c *Client) ExecuteCommand(ctx context.Context, command string, args json.RawMessage) error {
	params := map[string]any{"command": command}
	if trimmed := bytes.TrimSpace(args); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		params["arguments"] = args
	}
	_, err := c.call(ctx, "workspace/executeCommand", params)
	return err
}

func (c *Client) Completion(ctx context.Context, uri string, pos Position) ([]CompletionItem, error) {
	raw, err := c.callRetryInternal(ctx, "textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     pos,
		"context":      map[string]any{"triggerKind": 1},
	})
	if err != nil {
		return nil, err
	}
	return decodeCompletionItems(raw)
}

func (c *Client) Hover(ctx context.Context, uri string, pos Position) (*Hover, error) {
	raw, err := c.callRetryInternal(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     pos,
	})
	if err != nil {
		return nil, err
	}
	if isJSONNull(raw) {
		return nil, nil
	}
	var hover Hover
	if err := json.Unmarshal(raw, &hover); err != nil {
		return nil, fmt.Errorf("lsp: hover response: %w", err)
	}
	return &hover, nil
}

func (c *Client) Definition(ctx context.Context, uri string, pos Position) ([]Location, error) {
	raw, err := c.call(ctx, "textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     pos,
	})
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

func (c *Client) References(ctx context.Context, uri string, pos Position) ([]Location, error) {
	raw, err := c.call(ctx, "textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     pos,
		"context":      map[string]any{"includeDeclaration": true},
	})
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

func (c *Client) Format(ctx context.Context, uri string) ([]TextEdit, error) {
	raw, err := c.call(ctx, "textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"options": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
	})
	if err != nil {
		return nil, err
	}
	if isJSONNull(raw) {
		return nil, nil
	}
	var edits []TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("lsp: formatting response: %w", err)
	}
	return edits, nil
}

func (c *Client) CodeActions(ctx context.Context, uri string, rng Range, diags []Diagnostic) ([]CodeAction, error) {
	if diags == nil {
		diags = []Diagnostic{}
	}
	raw, err := c.call(ctx, "textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range":        rng,
		"context": map[string]any{
			"diagnostics": diags,
		},
	})
	if err != nil {
		return nil, err
	}
	return decodeCodeActions(raw)
}

func (c *Client) ResolveCodeAction(ctx context.Context, action CodeAction) (CodeAction, error) {
	raw, err := c.call(ctx, "codeAction/resolve", action)
	if err != nil {
		return action, err
	}
	if isJSONNull(raw) {
		return action, nil
	}
	var resolved CodeAction
	if err := json.Unmarshal(raw, &resolved); err != nil {
		return action, fmt.Errorf("lsp: codeAction/resolve response: %w", err)
	}
	return resolved, nil
}

func (c *Client) ApplyWorkspaceEdit(ctx context.Context, edit WorkspaceEdit) error {
	raw, err := c.call(ctx, "workspace/applyEdit", map[string]any{"edit": edit})
	if err != nil {
		return err
	}
	if isJSONNull(raw) {
		return nil
	}
	var result struct {
		Applied       bool   `json:"applied"`
		FailureReason string `json:"failureReason"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("lsp: applyEdit response: %w", err)
	}
	if !result.Applied {
		if result.FailureReason != "" {
			return fmt.Errorf("lsp: apply edit failed: %s", result.FailureReason)
		}
		return errors.New("lsp: apply edit failed")
	}
	return nil
}

func decodeCompletionItems(raw json.RawMessage) ([]CompletionItem, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[0] == '{' {
		var list struct {
			Items []CompletionItem `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("lsp: completion response: %w", err)
		}
		return list.Items, nil
	}
	var items []CompletionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("lsp: completion response: %w", err)
	}
	return items, nil
}

func decodeLocations(raw json.RawMessage) ([]Location, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[0] == '[' {
		var locations []Location
		if err := json.Unmarshal(raw, &locations); err != nil {
			return nil, fmt.Errorf("lsp: location response: %w", err)
		}
		return locations, nil
	}
	var location Location
	if err := json.Unmarshal(raw, &location); err != nil {
		return nil, fmt.Errorf("lsp: location response: %w", err)
	}
	if location.URI == "" {
		return nil, nil
	}
	return []Location{location}, nil
}

func decodeCodeActions(raw json.RawMessage) ([]CodeAction, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, fmt.Errorf("lsp: code action response: %w", err)
	}
	actions := make([]CodeAction, 0, len(raws))
	for _, item := range raws {
		var probe struct {
			Command json.RawMessage `json:"command"`
		}
		_ = json.Unmarshal(item, &probe)
		command := bytes.TrimSpace(probe.Command)
		if len(command) > 0 && command[0] == '"' {
			var cmd Command
			if json.Unmarshal(item, &cmd) == nil {
				actions = append(actions, CodeAction{Title: cmd.Title, Command: &cmd})
			}
			continue
		}
		var action CodeAction
		if err := json.Unmarshal(item, &action); err != nil {
			continue
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func isJSONNull(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) == 0 || string(raw) == "null"
}

func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}
