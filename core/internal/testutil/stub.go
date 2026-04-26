// Package testutil provides an in-process HTTPS stub that mimics the
// reference/apps_script/Code.gs contract. Tests use it to exercise the
// relay layer without hitting real Google.
package testutil

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// StubResponse describes what the stub should answer for a given target request.
// Body is ALREADY in its final form — if ContentEncoding is non-empty, the
// caller is responsible for pre-compressing Body to match.
type StubResponse struct {
	Status          int
	Header          map[string]string
	Body            []byte
	ContentEncoding string
}

// RequestLog records one relay-to-stub hit, with the INNER request fields
// (after the envelope has been parsed).
type RequestLog struct {
	Path        string // URL path hit on the stub, e.g. /macros/s/STUB1/exec
	Method      string // inner "m"
	URL         string // inner "u"
	Body        []byte // base64-decoded "b"
	ContentType string // inner "ct"
}

// AppsScriptStub mimics Code.gs. Single mode only (batch can be added later).
type AppsScriptStub struct {
	AuthKey string
	Routes  map[string]StubResponse // key: "METHOD u"

	mu     sync.Mutex
	Log    []RequestLog
	server *httptest.Server
}

// NewStub starts a TLS httptest.Server. Caller must Close it.
func NewStub(authKey string) *AppsScriptStub {
	s := NewHandlerStub(authKey)
	s.server = httptest.NewTLSServer(s.Handler())
	return s
}

// NewHandlerStub returns a stub without an attached server. Use Handler()
// to bind to a listener of your choice — useful for the e2e harness
// where we need a fixed port (httptest auto-picks random ones).
func NewHandlerStub(authKey string) *AppsScriptStub {
	return &AppsScriptStub{AuthKey: authKey, Routes: map[string]StubResponse{}}
}

// Handler exposes the request-handling logic so callers can serve on
// their own net/http.Server. Concurrency-safe.
func (s *AppsScriptStub) Handler() http.Handler { return http.HandlerFunc(s.handle) }

// BaseURL returns the stub's scheme+host base (https://127.0.0.1:PORT).
// Use this to construct multiple "script IDs" via path suffixes.
func (s *AppsScriptStub) BaseURL() string { return s.server.URL }

// ListenerAddr is the real address the fronter Dialer should target.
func (s *AppsScriptStub) ListenerAddr() net.Addr { return s.server.Listener.Addr() }

// Close shuts down the stub.
func (s *AppsScriptStub) Close() { s.server.Close() }

// batchItem mirrors the q-list item shape from Code.gs.
type batchItem struct {
	M  string `json:"m"`
	U  string `json:"u"`
	B  string `json:"b"`
	CT string `json:"ct"`
}

func (s *AppsScriptStub) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, map[string]any{"e": err.Error()})
		return
	}
	// Single-decode of the union envelope: presence of q:[...] selects
	// batch mode, matching Code.gs:35 (Array.isArray(req.q)). The
	// single-mode fields stay zero for batch envelopes and vice versa.
	var env struct {
		K  string            `json:"k"`
		Q  []json.RawMessage `json:"q"`
		M  string            `json:"m"`
		U  string            `json:"u"`
		B  string            `json:"b"`
		CT string            `json:"ct"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		writeJSON(w, map[string]any{"e": "parse: " + err.Error()})
		return
	}
	if env.K != s.AuthKey {
		writeJSON(w, map[string]any{"e": "unauthorized"})
		return
	}
	if env.Q != nil {
		s.handleBatch(w, r.URL.Path, env.Q)
		return
	}
	s.handleSingle(w, r.URL.Path, env.M, env.U, env.B, env.CT)
}

func (s *AppsScriptStub) handleSingle(w http.ResponseWriter, path, method, url, b64Body, ct string) {
	body, _ := base64.StdEncoding.DecodeString(b64Body)
	s.mu.Lock()
	s.Log = append(s.Log, RequestLog{
		Path: path, Method: method, URL: url,
		Body: body, ContentType: ct,
	})
	s.mu.Unlock()

	resp, ok := s.Routes[method+" "+url]
	if !ok {
		writeJSON(w, map[string]any{"s": 404, "h": map[string]string{}, "b": ""})
		return
	}
	writeJSON(w, toEnvelope(resp))
}

// handleBatch logs ONE entry for the whole envelope (so tests can
// assert the coalescing ratio) and returns one q-shaped result per
// item, mirroring Code.gs _doBatch including per-item bad-url errors.
func (s *AppsScriptStub) handleBatch(w http.ResponseWriter, path string, items []json.RawMessage) {
	s.mu.Lock()
	s.Log = append(s.Log, RequestLog{Path: path, Method: "BATCH", URL: ""})
	s.mu.Unlock()

	results := make([]map[string]any, len(items))
	for i, raw := range items {
		var item batchItem
		if err := json.Unmarshal(raw, &item); err != nil {
			results[i] = map[string]any{"e": "parse: " + err.Error()}
			continue
		}
		if !strings.HasPrefix(strings.ToLower(item.U), "http://") &&
			!strings.HasPrefix(strings.ToLower(item.U), "https://") {
			results[i] = map[string]any{"e": "bad url"}
			continue
		}
		resp, ok := s.Routes[item.M+" "+item.U]
		if !ok {
			results[i] = map[string]any{"s": 404, "h": map[string]string{}, "b": ""}
			continue
		}
		results[i] = toEnvelope(resp)
	}
	writeJSON(w, map[string]any{"q": results})
}

func toEnvelope(r StubResponse) map[string]any {
	h := map[string]string{}
	for k, v := range r.Header {
		h[k] = v
	}
	if r.ContentEncoding != "" {
		h["Content-Encoding"] = r.ContentEncoding
	}
	status := r.Status
	if status == 0 {
		status = 200
	}
	return map[string]any{
		"s": status,
		"h": h,
		"b": base64.StdEncoding.EncodeToString(r.Body),
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
