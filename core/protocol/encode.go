package protocol

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

// EncodeSingle marshals a Request into a single-mode envelope.
func EncodeSingle(req Request, authKey string) ([]byte, error) {
	env := envelopeSingle{
		K: authKey,
		M: req.Method,
		U: req.URL,
		H: filterHeaders(req.Header),
		R: req.FollowRedirects,
	}
	if len(req.Body) > 0 {
		env.B = base64.StdEncoding.EncodeToString(req.Body)
	}
	if req.ContentType != "" {
		env.CT = req.ContentType
	}
	return json.Marshal(env)
}

// EncodeBatch marshals a BatchRequest into a batch-mode envelope. The auth
// key appears exactly once at the top level; batch items never carry it.
func EncodeBatch(batch BatchRequest, authKey string) ([]byte, error) {
	env := envelopeBatch{K: authKey, Q: make([]envelopeItem, len(batch.Items))}
	for i, req := range batch.Items {
		item := envelopeItem{
			M: req.Method,
			U: req.URL,
			H: filterHeaders(req.Header),
			R: req.FollowRedirects,
		}
		if len(req.Body) > 0 {
			item.B = base64.StdEncoding.EncodeToString(req.Body)
		}
		if req.ContentType != "" {
			item.CT = req.ContentType
		}
		env.Q[i] = item
	}
	return json.Marshal(env)
}

// filterHeaders flattens http.Header → map[string]string, stripping entries
// listed in skipHeaders. Multi-value headers are joined per RFC 7230.
func filterHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if _, skip := skipHeaders[strings.ToLower(k)]; skip {
			continue
		}
		if len(v) == 0 {
			continue
		}
		out[k] = strings.Join(v, ", ")
	}
	return out
}
