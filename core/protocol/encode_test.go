package protocol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestEncodeSingle_MinimalGET(t *testing.T) {
	data, err := EncodeSingle(
		Request{Method: "GET", URL: "https://x", FollowRedirects: true},
		"secret",
	)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{
		"k": "secret", "m": "GET", "u": "https://x",
		"h": map[string]any{}, "r": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("envelope = %v, want %v (must be exactly k,m,u,h,r — no b, no ct)", got, want)
	}
}

func TestEncodeSingle_POSTWithBody(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	data, err := EncodeSingle(Request{
		Method: "POST", URL: "https://x",
		Body: body, ContentType: "application/json", FollowRedirects: true,
	}, "secret")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var raw struct {
		B  string `json:"b"`
		CT string `json:"ct"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.CT != "application/json" {
		t.Errorf("ct = %q, want application/json", raw.CT)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw.B)
	if err != nil {
		t.Fatalf("decode b: %v", err)
	}
	if !bytes.Equal(decoded, body) {
		t.Errorf("body mismatch: got %q want %q", decoded, body)
	}
}

func TestEncodeSingle_HeaderFiltering(t *testing.T) {
	h := http.Header{}
	h.Set("User-Agent", "ParvazTest")
	h.Set("Accept", "*/*")
	for _, k := range []string{
		"Host", "Connection", "Content-Length", "Transfer-Encoding",
		"Proxy-Connection", "Proxy-Authorization", "Priority", "TE",
		// Accept-Encoding is stripped client-side: UrlFetchApp auto-decodes
		// regardless, so shipping it upstream just inflates the envelope.
		"Accept-Encoding",
	} {
		h.Set(k, "x")
	}

	data, err := EncodeSingle(Request{
		Method: "GET", URL: "https://x", Header: h, FollowRedirects: true,
	}, "k")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var env struct {
		H map[string]string `json:"h"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]string{"User-Agent": "ParvazTest", "Accept": "*/*"}
	if !reflect.DeepEqual(env.H, want) {
		t.Errorf("headers = %v, want %v", env.H, want)
	}
}

func TestEncodeBatch(t *testing.T) {
	batch := BatchRequest{Items: []Request{
		{Method: "GET", URL: "https://a", FollowRedirects: true},
		{Method: "GET", URL: "https://b", FollowRedirects: true},
	}}
	data, err := EncodeBatch(batch, "secret")
	if err != nil {
		t.Fatalf("encode batch: %v", err)
	}
	var env struct {
		K string           `json:"k"`
		Q []map[string]any `json:"q"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.K != "secret" {
		t.Errorf("k = %q, want secret", env.K)
	}
	if len(env.Q) != 2 {
		t.Fatalf("q len = %d, want 2", len(env.Q))
	}
	for i, item := range env.Q {
		if _, has := item["k"]; has {
			t.Errorf("item[%d] leaks k at batch-item level", i)
		}
	}
	if env.Q[0]["u"] != "https://a" || env.Q[1]["u"] != "https://b" {
		t.Errorf("URLs out of order: %v, %v", env.Q[0]["u"], env.Q[1]["u"])
	}
}
