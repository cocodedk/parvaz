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
	s := &AppsScriptStub{AuthKey: authKey, Routes: map[string]StubResponse{}}
	s.server = httptest.NewTLSServer(http.HandlerFunc(s.handle))
	return s
}

// BaseURL returns the stub's scheme+host base (https://127.0.0.1:PORT).
// Use this to construct multiple "script IDs" via path suffixes.
func (s *AppsScriptStub) BaseURL() string { return s.server.URL }

// ListenerAddr is the real address the fronter Dialer should target.
func (s *AppsScriptStub) ListenerAddr() net.Addr { return s.server.Listener.Addr() }

// Close shuts down the stub.
func (s *AppsScriptStub) Close() { s.server.Close() }

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
	var env struct {
		K  string `json:"k"`
		M  string `json:"m"`
		U  string `json:"u"`
		B  string `json:"b"`
		CT string `json:"ct"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		writeJSON(w, map[string]any{"e": "parse: " + err.Error()})
		return
	}
	if env.K != s.AuthKey {
		writeJSON(w, map[string]any{"e": "unauthorized"})
		return
	}
	body, _ := base64.StdEncoding.DecodeString(env.B)
	s.mu.Lock()
	s.Log = append(s.Log, RequestLog{
		Path: r.URL.Path, Method: env.M, URL: env.U,
		Body: body, ContentType: env.CT,
	})
	s.mu.Unlock()

	resp, ok := s.Routes[env.M+" "+env.U]
	if !ok {
		writeJSON(w, map[string]any{"s": 404, "h": map[string]string{}, "b": ""})
		return
	}
	writeJSON(w, toEnvelope(resp))
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
