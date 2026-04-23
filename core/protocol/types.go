// Package protocol implements the Apps Script relay JSON envelope.
//
// This package is pure: no network, no goroutines, no I/O beyond what
// encoding/json does. The wire format mirrors reference/apps_script/Code.gs:
//
//	request  (single): { k, m, u, h, b?, ct?, r }
//	request  (batch):  { k, q: [ { m, u, h, b?, ct?, r }, ... ] }
//	response (single): { s, h, b }  or  { e: "..." }
//	response (batch):  { q: [ { s, h, b } | { e: "..." }, ... ] }  or  { e: "..." }
package protocol

import "net/http"

// Request is a single HTTP request to be tunneled through the relay.
type Request struct {
	Method          string
	URL             string
	Header          http.Header
	Body            []byte
	ContentType     string
	FollowRedirects bool
}

// BatchRequest groups multiple requests into a single envelope. The server
// dispatches them in parallel via UrlFetchApp.fetchAll().
type BatchRequest struct {
	Items []Request
}

// Response mirrors what the relay reports for a single item.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// BatchItemResult carries either Response or Err — never both.
type BatchItemResult struct {
	Response *Response
	Err      error
}

// BatchResponse is parallel to BatchRequest.Items — order is preserved.
type BatchResponse struct {
	Items []BatchItemResult
}

// ServerError is the typed form of the relay's { "e": "..." } error envelope.
type ServerError struct {
	Message string
}

func (e *ServerError) Error() string { return "apps script: " + e.Message }

// skipHeaders matches reference/apps_script/Code.gs SKIP_HEADERS. Lowercase
// keys for case-insensitive comparison.
var skipHeaders = map[string]struct{}{
	"host":                {},
	"connection":          {},
	"content-length":      {},
	"transfer-encoding":   {},
	"proxy-connection":    {},
	"proxy-authorization": {},
	"priority":            {},
	"te":                  {},
}

// envelopeSingle is the single-mode request wire format.
type envelopeSingle struct {
	K  string            `json:"k"`
	M  string            `json:"m"`
	U  string            `json:"u"`
	H  map[string]string `json:"h"`
	B  string            `json:"b,omitempty"`
	CT string            `json:"ct,omitempty"`
	R  bool              `json:"r"`
}

// envelopeItem — batch items carry no auth key.
type envelopeItem struct {
	M  string            `json:"m"`
	U  string            `json:"u"`
	H  map[string]string `json:"h"`
	B  string            `json:"b,omitempty"`
	CT string            `json:"ct,omitempty"`
	R  bool              `json:"r"`
}

// envelopeBatch is the batch-mode request wire format.
type envelopeBatch struct {
	K string         `json:"k"`
	Q []envelopeItem `json:"q"`
}

// responseEnvelope is the single-mode response wire format.
type responseEnvelope struct {
	S int               `json:"s,omitempty"`
	H map[string]string `json:"h,omitempty"`
	B string            `json:"b,omitempty"`
	E string            `json:"e,omitempty"`
}

// batchResponseEnvelope is the batch-mode response wire format.
type batchResponseEnvelope struct {
	Q []responseEnvelope `json:"q,omitempty"`
	E string             `json:"e,omitempty"`
}
