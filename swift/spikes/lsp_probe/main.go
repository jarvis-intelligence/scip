// Command lsp_probe is the Phase 1 SourceKit-LSP spike driver (plan 01-03).
//
// CAUTION: throwaway spike evidence tooling. Nothing under swift/spikes/ may be
// promoted into internal/ or wired into the CLI; the Phase 3 LSP client is
// built fresh against the findings recorded by this driver.
//
// Modes:
//
//	-mode capability  harvest the hand-written hard-case fixture
//	-mode perf        harvest the generated ~500-file perf fixture
//
// Readiness discipline (guards the cold-index trap): before any harvest
// request the driver polls the sourcekit/isIndexing extension and then requires
// a non-empty USR from a textDocument/symbolInfo probe at a known declaration.
// An empty result can therefore never be recorded as "LSP cannot do X" — the
// gate either passes first or the run fails loudly.
//
// Harvest discipline (so perf numbers are predictive, not adversarial): one
// documentSymbol pass per file, then one references pass per unique
// definition, bounded concurrency for the references pass only.
//
// Usage (from swift/):
//
//	GOWORK=off go run ./spikes/lsp_probe \
//	  -fixture spikes/fixture -out spikes/evidence-capability.jsonl \
//	  -mode capability
//	GOWORK=off go run ./spikes/lsp_probe \
//	  -fixture spikes/perf/gen -out spikes/evidence-perf-cold.jsonl \
//	  -mode perf
//
// macOS + Xcode required (runtime-only): the server is discovered via
// `xcrun -f sourcekit-lsp` with a PATH fallback, never a hardcoded path.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/jsonrpc2"
)

// --- Evidence records ---

// rec is one JSONL evidence line: one RPC request/response pair (or a
// meta/gate/summary marker) with its round-trip latency in milliseconds.
type rec struct {
	TS        string          `json:"ts"`
	Phase     string          `json:"phase"` // meta | init | ready | harvest | hierarchy | summary
	Method    string          `json:"method,omitempty"`
	File      string          `json:"file,omitempty"`
	LatencyMS float64         `json:"latency_ms"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type evidence struct {
	mu sync.Mutex
	w  *bufio.Writer
	f  *os.File

	// trim replaces large result payloads with compact counts (perf mode)
	// so multi-thousand-record runs stay diffable; capability mode keeps
	// verbatim payloads because the findings doc quotes excerpts from them.
	trim bool

	requests  int
	latencies []float64
	crashes   int
}

func (e *evidence) write(r rec) {
	r.TS = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsp_probe: marshal evidence record: %v\n", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.w.Write(b)
	e.w.WriteByte('\n')
}

// --- LSP wire shapes ---

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type textDocumentID struct {
	URI string `json:"uri"`
}

func docSymbolParams(uri string) map[string]any {
	return map[string]any{"textDocument": textDocumentID{URI: uri}}
}

func positionParams(uri string, pos lspPosition) map[string]any {
	return map[string]any{"textDocument": textDocumentID{URI: uri}, "position": pos}
}

func referencesParams(uri string, pos lspPosition) map[string]any {
	return map[string]any{
		"textDocument": textDocumentID{URI: uri},
		"position":     pos,
		"context":      map[string]any{"includeDeclaration": true},
	}
}

// defInfo is a flattened document-symbol definition site.
type defInfo struct {
	URI       string
	Name      string
	Kind      int
	Line      int
	Character int
	Container string
}

func (d defInfo) key() string {
	return fmt.Sprintf("%s|%d|%d|%s", d.URI, d.Line, d.Character, d.Name)
}

// flattenSymbols accepts either hierarchical DocumentSymbol[] or flat
// SymbolInformation[] (both are legal LSP responses) and flattens to def sites.
func flattenSymbols(uri string, raw json.RawMessage) ([]defInfo, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("documentSymbol payload: %w", err)
	}
	var out []defInfo
	var walk func(entry map[string]json.RawMessage)
	walk = func(entry map[string]json.RawMessage) {
		var name string
		json.Unmarshal(entry["name"], &name)
		var kind int
		json.Unmarshal(entry["kind"], &kind)
		if sel, ok := entry["selectionRange"]; ok {
			// Hierarchical DocumentSymbol.
			var r lspRange
			if err := json.Unmarshal(sel, &r); err == nil {
				out = append(out, defInfo{
					URI: uri, Name: name, Kind: kind,
					Line: r.Start.Line, Character: r.Start.Character,
				})
			}
			var children []map[string]json.RawMessage
			if json.Unmarshal(entry["children"], &children) == nil {
				for _, c := range children {
					walk(c)
				}
			}
			return
		}
		// Flat SymbolInformation.
		var loc struct {
			URI   string   `json:"uri"`
			Range lspRange `json:"range"`
		}
		if err := json.Unmarshal(entry["location"], &loc); err == nil && loc.URI != "" {
			var container string
			json.Unmarshal(entry["containerName"], &container)
			out = append(out, defInfo{
				URI: loc.URI, Name: name, Kind: kind, Container: container,
				Line: loc.Range.Start.Line, Character: loc.Range.Start.Character,
			})
		}
	}
	for _, e := range entries {
		walk(e)
	}
	return out, nil
}

// referenceLocations counts Locations in a references/definition response
// (Location | Location[] | null all handled leniently).
func referenceLocations(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 1 // single Location
	}
	return len(arr)
}

// --- Server process ---

// stderrRing keeps the last tail bytes of server stderr for crash diagnostics.
type stderrRing struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	tail []byte
}

func (s *stderrRing) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Write(p)
	if s.buf.Len() > 64*1024 {
		over := s.buf.Len() - 64*1024
		s.buf.Next(over)
	}
	return len(p), nil
}

func (s *stderrRing) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

type pipeRWC struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (p pipeRWC) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p pipeRWC) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p pipeRWC) Close() error {
	rerr := p.r.Close()
	werr := p.w.Close()
	if rerr != nil {
		return rerr
	}
	return werr
}

// discoverServer finds sourcekit-lsp via xcrun (never a hardcoded absolute
// path) with a PATH fallback. Fixed argv; no shell, no interpolation.
func discoverServer() (string, string, error) {
	out, err := exec.Command("xcrun", "-f", "sourcekit-lsp").Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return p, "xcrun -f sourcekit-lsp", nil
		}
	}
	if p, err := exec.LookPath("sourcekit-lsp"); err == nil {
		return p, "PATH fallback", nil
	}
	return "", "", fmt.Errorf("sourcekit-lsp not found via xcrun or PATH: %w", err)
}

func commandOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return fmt.Sprintf("(error: %v) %s", err, s)
	}
	return s
}

func serverRSSKb(pid int) int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return -1
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return -1
	}
	return kb
}

// --- UTF-16 position math (LSP positions are UTF-16 code units) ---

// utf16Col converts a byte offset within a single line into a UTF-16 column.
func utf16Col(line string, byteOff int) int {
	u := 0
	for i, r := range line {
		if i >= byteOff {
			break
		}
		if r > 0xFFFF {
			u += 2
		} else {
			u++
		}
	}
	return u
}

// findDecl locates `keyword name` (e.g. "struct Shape") and returns the
// UTF-16 position of `name`. Returns ok=false when absent.
func findDecl(text, keyword, name string) (lspPosition, bool) {
	lines := strings.Split(text, "\n")
	for li, ln := range lines {
		ki := strings.Index(ln, keyword)
		if ki < 0 {
			continue
		}
		ni := strings.Index(ln[ki+len(keyword):], name)
		if ni < 0 {
			continue
		}
		byteOff := ki + len(keyword) + ni
		return lspPosition{Line: li, Character: utf16Col(ln, byteOff)}, true
	}
	return lspPosition{}, false
}

// findUse locates the identifier inside the first occurrence of `snippet`
// (e.g. "draw" inside "d.draw()").
func findUse(text, snippet, ident string) (lspPosition, bool) {
	lines := strings.Split(text, "\n")
	for li, ln := range lines {
		si := strings.Index(ln, snippet)
		if si < 0 {
			continue
		}
		ii := strings.Index(snippet, ident)
		byteOff := si + ii
		return lspPosition{Line: li, Character: utf16Col(ln, byteOff)}, true
	}
	return lspPosition{}, false
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file://" + abs
}

// --- Driver ---

type driver struct {
	ev       *evidence
	conn     *jsonrpc2.Conn
	cmd      *exec.Cmd
	stderr   *stderrRing
	callTOM  time.Duration
	openDocs map[string]bool
}

func (d *driver) call(ctx context.Context, phase, method, file string, params any, result any) error {
	pctx, cancel := context.WithTimeout(ctx, d.callTOM)
	defer cancel()
	start := time.Now()
	callErr := d.conn.Call(pctx, method, params, result)
	lat := float64(time.Since(start).Microseconds()) / 1000.0

	d.ev.mu.Lock()
	d.ev.requests++
	d.ev.latencies = append(d.ev.latencies, lat)
	d.ev.mu.Unlock()

	r := rec{Phase: phase, Method: method, File: file, LatencyMS: lat}
	if params != nil {
		r.Params, _ = json.Marshal(params)
	}
	if callErr != nil {
		r.Error = callErr.Error()
	} else if result != nil {
		switch v := result.(type) {
		case *json.RawMessage:
			if v != nil && len(*v) > 0 {
				if d.ev.trim {
					r.Result = trimmedResult(method, *v)
				} else {
					r.Result = *v
				}
			}
		default:
			b, merr := json.Marshal(result)
			if merr == nil {
				if d.ev.trim {
					r.Result = trimmedResult(method, b)
				} else {
					r.Result = b
				}
			}
		}
	}
	d.ev.write(r)

	if callErr != nil && isConnDeath(callErr) {
		d.ev.mu.Lock()
		d.ev.crashes++
		d.ev.mu.Unlock()
		fmt.Fprintf(os.Stderr, "lsp_probe: server connection died on %s: %v\nstderr tail:\n%s\n",
			method, callErr, d.stderr.snapshot())
	}
	return callErr
}

func (d *driver) notify(ctx context.Context, phase, method, file string, params any) error {
	start := time.Now()
	err := d.conn.Notify(ctx, method, params)
	lat := float64(time.Since(start).Microseconds()) / 1000.0
	r := rec{Phase: phase, Method: method, File: file, LatencyMS: lat}
	if params != nil {
		r.Params, _ = json.Marshal(params)
	}
	if err != nil {
		r.Error = err.Error()
	}
	d.ev.write(r)
	return err
}

// trimmedResult replaces bulky payloads with a compact count in perf mode.
func trimmedResult(method string, raw json.RawMessage) json.RawMessage {
	n := referenceLocations(raw)
	// documentSymbol responses are arrays of symbol objects; count them too.
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		n = len(arr)
	}
	b, _ := json.Marshal(map[string]any{"trimmed": true, "count": n})
	return b
}

func isConnDeath(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "file already closed") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, io.ErrClosedPipe.Error())
}

func (d *driver) start(ctx context.Context, serverPath string) error {
	cmd := exec.Command(serverPath) // fixed argv; no flags, no shell
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sourcekit-lsp: %w", err)
	}
	d.cmd = cmd
	d.stderr = &stderrRing{}
	go io.Copy(d.stderr, stderr)
	noop := jsonrpc2.HandlerWithError(func(context.Context, *jsonrpc2.Conn, *jsonrpc2.Request) (interface{}, error) {
		return nil, nil
	})
	d.conn = jsonrpc2.NewConn(ctx, jsonrpc2.NewBufferedStream(pipeRWC{stdout, stdin}, jsonrpc2.VSCodeObjectCodec{}), noop)
	return nil
}

// initialize performs the LSP handshake with the fixture as rootUri and
// records the full InitializeResult (capabilities verbatim + serverInfo).
func (d *driver) initialize(ctx context.Context, rootPath string) error {
	uri := fileURI(rootPath)
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   uri,
		"clientInfo": map[string]any{
			"name":    "scip-swift-phase1-spike",
			"version": "01-03",
		},
		"workspaceFolders": []any{map[string]any{"uri": uri, "name": filepath.Base(rootPath)}},
		"capabilities": map[string]any{
			"window": map[string]any{"workDoneProgress": true},
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
	}
	var result json.RawMessage
	if err := d.call(ctx, "init", "initialize", "", params, &result); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	return d.notify(ctx, "init", "initialized", "", map[string]any{})
}

// readinessGate implements the cold-index trap guard: poll
// sourcekit/isIndexing, then require a non-empty USR from symbolInfo at a
// known declaration, retrying until the timeout. The probe document is
// didOpen'd first — sourcekit-lsp answers -32001 "No language service" for
// documents it has not been shown.
func (d *driver) readinessGate(ctx context.Context, probePath string, probePos lspPosition) (time.Duration, bool, error) {
	gateStart := time.Now()
	probeURI := fileURI(probePath)
	if err := d.didOpen(ctx, "ready", probePath); err != nil {
		return 0, true, fmt.Errorf("gate didOpen: %w", err)
	}
	indexingSupported := true
	deadline := gateStart.Add(*readyTimeout)
	for {
		var raw json.RawMessage
		err := d.call(ctx, "ready", "sourcekit/isIndexing", "", map[string]any{}, &raw)
		if err != nil {
			if strings.Contains(err.Error(), "method not found") || strings.Contains(err.Error(), "MethodNotFound") {
				indexingSupported = false
			}
			break // probe below is the authoritative gate
		}
		if !isIndexing(raw) {
			break
		}
		if time.Now().After(deadline) {
			return time.Since(gateStart), indexingSupported, fmt.Errorf("isIndexing still true after %v", *readyTimeout)
		}
		select {
		case <-ctx.Done():
			return time.Since(gateStart), indexingSupported, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	// Authoritative gate: known-symbol probe must return a non-empty USR.
	for {
		var details json.RawMessage
		err := d.call(ctx, "ready", "textDocument/symbolInfo", filepath.Base(strings.TrimPrefix(probeURI, "file://")), positionParams(probeURI, probePos), &details)
		if err == nil && hasUSR(details) {
			elapsed := time.Since(gateStart)
			d.ev.write(rec{Phase: "ready", Method: "gate", Result: mustJSON(map[string]any{
				"time_to_ready_ms":     float64(elapsed.Microseconds()) / 1000.0,
				"indexing_supported":   indexingSupported,
				"probe_uri":            probeURI,
				"probe_symbol_details": details,
			})})
			return elapsed, indexingSupported, nil
		}
		if time.Now().After(deadline) {
			reason := fmt.Sprintf("readiness probe never returned a USR (last err: %v)", err)
			d.ev.write(rec{Phase: "ready", Method: "gate", Error: reason})
			return time.Since(gateStart), indexingSupported, fmt.Errorf("%s", reason)
		}
		select {
		case <-ctx.Done():
			return time.Since(gateStart), indexingSupported, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func isIndexing(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return false
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	var obj struct {
		Indexing   *bool `json:"indexing"`
		IsIndexing *bool `json:"isIndexing"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if obj.Indexing != nil {
			return *obj.Indexing
		}
		if obj.IsIndexing != nil {
			return *obj.IsIndexing
		}
	}
	return false
}

func hasUSR(raw json.RawMessage) bool {
	var details []struct {
		USR string `json:"usr"`
	}
	if err := json.Unmarshal(raw, &details); err != nil {
		return false
	}
	for _, d := range details {
		if d.USR != "" {
			return true
		}
	}
	return false
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"marshal_error":true}`)
	}
	return b
}

// didOpen opens a document (text from disk, version 1), skipping documents
// this driver already opened (re-opening is a protocol error).
func (d *driver) didOpen(ctx context.Context, phase, path string) error {
	uri := fileURI(path)
	if d.openDocs[uri] {
		return nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := d.notify(ctx, phase, "textDocument/didOpen", filepath.Base(path), map[string]any{
		"textDocument": map[string]any{
			"uri": uri, "languageId": "swift", "version": 1, "text": string(text),
		},
	}); err != nil {
		return err
	}
	d.openDocs[uri] = true
	return nil
}

// harvestFiles does one didOpen + documentSymbol pass per file and returns
// the unique definition sites across the fixture.
func (d *driver) harvestFiles(ctx context.Context, files []string) ([]defInfo, int, error) {
	seen := map[string]bool{}
	var defs []defInfo
	for _, f := range files {
		if err := d.didOpen(ctx, "harvest", f); err != nil {
			return nil, 0, fmt.Errorf("didOpen %s: %w", f, err)
		}
		uri := fileURI(f)
		var raw json.RawMessage
		if err := d.call(ctx, "harvest", "textDocument/documentSymbol", filepath.Base(f), docSymbolParams(uri), &raw); err != nil {
			return nil, 0, fmt.Errorf("documentSymbol %s: %w", f, err)
		}
		fileDefs, err := flattenSymbols(uri, raw)
		if err != nil {
			return nil, 0, fmt.Errorf("flatten %s: %w", f, err)
		}
		for _, def := range fileDefs {
			if !seen[def.key()] {
				seen[def.key()] = true
				defs = append(defs, def)
			}
		}
	}
	return defs, 0, nil
}

// referencesPass runs one references request per unique definition with a
// bounded worker pool. Returns the total occurrence count harvested.
func (d *driver) referencesPass(ctx context.Context, defs []defInfo, workers int) (int, error) {
	type job struct{ def defInfo }
	jobs := make(chan job)
	var mu sync.Mutex
	occurrences := 0
	var runErr error
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var raw json.RawMessage
				err := d.call(ctx, "harvest", "textDocument/references",
					filepath.Base(strings.TrimPrefix(j.def.URI, "file://")),
					referencesParams(j.def.URI, lspPosition{Line: j.def.Line, Character: j.def.Character}), &raw)
				mu.Lock()
				if err != nil && runErr == nil && isConnDeath(err) {
					runErr = fmt.Errorf("server died during references pass: %w", err)
				}
				if err == nil {
					occurrences += referenceLocations(raw)
				}
				mu.Unlock()
			}
		}()
	}
	for _, def := range defs {
		if ctx.Err() != nil {
			break
		}
		mu.Lock()
		stopped := runErr != nil
		mu.Unlock()
		if stopped {
			break
		}
		jobs <- job{def: def}
	}
	close(jobs)
	wg.Wait()
	return occurrences, runErr
}

func percentile(lats []float64, p float64) float64 {
	if len(lats) == 0 {
		return 0
	}
	s := append([]float64(nil), lats...)
	sort.Float64s(s)
	idx := int(p * float64(len(s)))
	if idx >= len(s) {
		idx = len(s) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return s[idx]
}

func swiftSources(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".build" || strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".swift") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

var (
	flagFixture   = flag.String("fixture", "", "path to the SwiftPM fixture package root (required)")
	flagOut       = flag.String("out", "", "JSONL evidence output path (required)")
	flagMode      = flag.String("mode", "capability", "capability | perf")
	flagProbeSym  = flag.String("probe-symbol", "", "known symbol name for the readiness gate (default: Shape in capability mode, Gen0000Widget in perf mode)")
	flagProbeKind = flag.String("probe-kind", "", "declaration keyword for the readiness probe (default: struct)")
	callTimeout   = flag.Duration("call-timeout", 3*time.Minute, "per-request timeout")
	readyTimeout  = flag.Duration("ready-timeout", 30*time.Minute, "readiness gate timeout")
	flagWorkers   = flag.Int("concurrency", 4, "perf mode: bounded concurrency for the references pass")
)

func main() {
	flag.Parse()
	if *flagFixture == "" || *flagOut == "" {
		fmt.Fprintln(os.Stderr, "lsp_probe: -fixture and -out are required")
		flag.Usage()
		os.Exit(2)
	}
	started := time.Now()
	fixtureAbs, err := filepath.Abs(*flagFixture)
	if err != nil {
		fail(err)
	}
	files, err := swiftSources(fixtureAbs)
	if err != nil {
		fail(err)
	}
	if len(files) == 0 {
		fail(fmt.Errorf("no .swift sources found under %s", fixtureAbs))
	}

	// Readiness probe defaults per mode.
	probeSymbol, probeKind := *flagProbeSym, *flagProbeKind
	if probeSymbol == "" {
		probeSymbol = "Shape"
		if *flagMode == "perf" {
			// Matches the default generate -files 500 padding (Gen000…Gen499).
			probeSymbol = "Gen000Widget"
		}
	}
	if probeKind == "" {
		probeKind = "struct"
	}

	out, err := os.Create(*flagOut)
	if err != nil {
		fail(err)
	}
	ev := &evidence{w: bufio.NewWriter(out), f: out, trim: *flagMode == "perf"}
	defer func() {
		ev.w.Flush()
		out.Close()
	}()

	serverPath, discovery, err := discoverServer()
	if err != nil {
		fail(err)
	}
	ev.write(rec{Phase: "meta", Result: mustJSON(map[string]any{
		"mode":                  *flagMode,
		"fixture":               fixtureAbs,
		"files":                 len(files),
		"concurrency":           *flagWorkers,
		"server_path":           serverPath,
		"server_discovery":      discovery,
		"sourcekit_lsp_flag":    commandOutput(serverPath, "--version"),
		"sourcekit_lsp_version": "recorded via initialize response serverInfo (Xcode 26.3 build has no --version flag)",
		"swift_version":         commandOutput("swift", "--version"),
		"xcodebuild_version":    commandOutput("xcodebuild", "-version"),
		"started_at":            started.UTC().Format(time.RFC3339Nano),
	})})

	ctx := context.Background()
	d := &driver{ev: ev, callTOM: *callTimeout, openDocs: map[string]bool{}}
	if err := d.start(ctx, serverPath); err != nil {
		fail(err)
	}
	defer func() {
		d.conn.Close()
		done := make(chan struct{})
		go func() { d.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			d.cmd.Process.Kill()
		}
	}()

	if err := d.initialize(ctx, fixtureAbs); err != nil {
		fail(err)
	}

	// Readiness gate before any harvest (cold-index trap guard).
	var probePath string
	var probePos lspPosition
	for _, f := range files {
		text, err := os.ReadFile(f)
		if err != nil {
			fail(err)
		}
		if pos, ok := findDecl(string(text), probeKind, probeSymbol); ok {
			probePath, probePos = f, pos
			break
		}
	}
	if probePath == "" {
		fail(fmt.Errorf("probe declaration %s %s not found in fixture", probeKind, probeSymbol))
	}
	timeToReady, indexingSupported, err := d.readinessGate(ctx, probePath, probePos)
	if err != nil {
		fail(err)
	}
	_ = indexingSupported

	// Harvest phase 1: one didOpen + documentSymbol pass per file.
	defs, _, err := d.harvestFiles(ctx, files)
	if err != nil {
		fail(err)
	}

	occurrences := 0
	if *flagMode == "capability" {
		// Phase 2: symbolInfo + definition + references per unique def.
		for _, def := range defs {
			pos := lspPosition{Line: def.Line, Character: def.Character}
			base := filepath.Base(strings.TrimPrefix(def.URI, "file://"))
			var si json.RawMessage
			d.call(ctx, "harvest", "textDocument/symbolInfo", base, positionParams(def.URI, pos), &si)
			var defn json.RawMessage
			d.call(ctx, "harvest", "textDocument/definition", base, positionParams(def.URI, pos), &defn)
			var refs json.RawMessage
			if err := d.call(ctx, "harvest", "textDocument/references", base, referencesParams(def.URI, pos), &refs); err == nil {
				occurrences += referenceLocations(refs)
			}
		}
		// Phase 3: hierarchy requests for exactly one call site and one type.
		d.hierarchyProbes(ctx, files)
	} else {
		// Perf discipline: references pass per unique definition, bounded pool.
		occurrences, err = d.referencesPass(ctx, defs, *flagWorkers)
		if err != nil {
			fail(err)
		}
	}

	// Server RSS while the process is still alive (before shutdown/exit).
	rss := serverRSSKb(d.cmd.Process.Pid)

	// Shutdown.
	var shutdownResult json.RawMessage
	d.call(ctx, "summary", "shutdown", "", nil, &shutdownResult)
	d.notify(ctx, "summary", "exit", "", nil)

	wall := time.Since(started)
	ev.mu.Lock()
	reqCount, crashes, lats := ev.requests, ev.crashes, ev.latencies
	ev.mu.Unlock()
	ev.write(rec{Phase: "summary", Result: mustJSON(map[string]any{
		"mode":                       *flagMode,
		"total_wall_ms":              float64(wall.Microseconds()) / 1000.0,
		"time_to_ready_ms":           float64(timeToReady.Microseconds()) / 1000.0,
		"request_count":              reqCount,
		"requests_per_sec":           float64(reqCount) / wall.Seconds(),
		"latency_p50_ms":             percentile(lats, 0.50),
		"latency_p95_ms":             percentile(lats, 0.95),
		"server_restarts_or_crashes": crashes,
		"symbols_harvested":          len(defs),
		"occurrences":                occurrences,
		"server_rss_kb":              rss,
		"concurrency":                *flagWorkers,
	})})
	fmt.Printf("lsp_probe: mode=%s requests=%d time_to_ready=%v wall=%v symbols=%d occurrences=%d crashes=%d rss_kb=%d\n",
		*flagMode, reqCount, timeToReady.Round(time.Millisecond), wall.Round(time.Millisecond),
		len(defs), occurrences, crashes, rss)
}

// hierarchyProbes exercises the call/type-hierarchy surface for exactly one
// call site and one type (capability mode only).
func (d *driver) hierarchyProbes(ctx context.Context, files []string) {
	var core string
	for _, f := range files {
		if filepath.Base(f) == "Core.swift" {
			core = f
			text, err := os.ReadFile(f)
			if err != nil {
				return
			}
			d.hierarchyAt(ctx, string(text), core, "d.draw()", "draw")
			d.hierarchyAt(ctx, string(text), core, "final class Vec", "Vec")
			return
		}
	}
}

func (d *driver) hierarchyAt(ctx context.Context, text, file, snippet, ident string) {
	pos, ok := findUse(string(text), snippet, ident)
	if !ok {
		d.ev.write(rec{Phase: "hierarchy", Error: fmt.Sprintf("snippet %q not found in %s", snippet, file)})
		return
	}
	base := filepath.Base(file)
	var items json.RawMessage
	if err := d.call(ctx, "hierarchy", "textDocument/prepareCallHierarchy", base, positionParams(fileURI(file), pos), &items); err != nil {
		d.ev.write(rec{Phase: "hierarchy", Error: fmt.Sprintf("textDocument/prepareCallHierarchy: %v", err)})
	} else if item, ok := firstItem(items); ok {
		d.call(ctx, "hierarchy", "callHierarchy/incomingCalls", base, map[string]any{"item": item}, new(json.RawMessage))
		d.call(ctx, "hierarchy", "callHierarchy/outgoingCalls", base, map[string]any{"item": item}, new(json.RawMessage))
	}
	var tItems json.RawMessage
	if err := d.call(ctx, "hierarchy", "textDocument/prepareTypeHierarchy", base, positionParams(fileURI(file), pos), &tItems); err != nil {
		d.ev.write(rec{Phase: "hierarchy", Error: fmt.Sprintf("textDocument/prepareTypeHierarchy: %v", err)})
	} else if item, ok := firstItem(tItems); ok {
		d.call(ctx, "hierarchy", "typeHierarchy/supertypes", base, map[string]any{"item": item}, new(json.RawMessage))
		d.call(ctx, "hierarchy", "typeHierarchy/subtypes", base, map[string]any{"item": item}, new(json.RawMessage))
	}
}

func firstItem(raw json.RawMessage) (json.RawMessage, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return nil, false
	}
	return arr[0], true
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "lsp_probe: %v\n", err)
	os.Exit(1)
}
