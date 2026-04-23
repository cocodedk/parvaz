package protocol

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func TestDecodeResponse_Success(t *testing.T) {
	body := []byte("hello world")
	raw := `{"s":200,"h":{"Content-Type":"text/plain"},"b":"` +
		base64.StdEncoding.EncodeToString(body) + `"}`
	resp, err := DecodeResponse([]byte(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("ct = %q, want text/plain", resp.Header.Get("Content-Type"))
	}
	if !bytes.Equal(resp.Body, body) {
		t.Errorf("body = %q, want %q", resp.Body, body)
	}
}

func TestDecodeResponse_Error(t *testing.T) {
	_, err := DecodeResponse([]byte(`{"e":"unauthorized"}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var srv *ServerError
	if !errors.As(err, &srv) {
		t.Fatalf("expected *ServerError, got %T: %v", err, err)
	}
	if srv.Message != "unauthorized" {
		t.Errorf("message = %q, want unauthorized", srv.Message)
	}
}

func TestDecodeBatchResponse_MixedErrors(t *testing.T) {
	body := []byte("ok")
	raw := `{"q":[` +
		`{"s":200,"h":{"Content-Type":"text/plain"},"b":"` +
		base64.StdEncoding.EncodeToString(body) + `"},` +
		`{"e":"target unreachable"},` +
		`{"s":404,"h":{},"b":""}` +
		`]}`
	bresp, err := DecodeBatchResponse([]byte(raw))
	if err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if len(bresp.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(bresp.Items))
	}

	// Item 0 — success
	if bresp.Items[0].Err != nil || bresp.Items[0].Response == nil {
		t.Fatalf("item[0] = %+v / err=%v", bresp.Items[0].Response, bresp.Items[0].Err)
	}
	if bresp.Items[0].Response.Status != 200 ||
		!bytes.Equal(bresp.Items[0].Response.Body, body) {
		t.Errorf("item[0] wrong: %+v", bresp.Items[0].Response)
	}

	// Item 1 — per-item error (order preserved)
	if bresp.Items[1].Response != nil {
		t.Errorf("item[1].response not nil: %+v", bresp.Items[1].Response)
	}
	var srv *ServerError
	if !errors.As(bresp.Items[1].Err, &srv) || srv.Message != "target unreachable" {
		t.Errorf("item[1].err = %v, want ServerError{target unreachable}", bresp.Items[1].Err)
	}

	// Item 2 — success w/ 404
	if bresp.Items[2].Err != nil || bresp.Items[2].Response == nil ||
		bresp.Items[2].Response.Status != 404 {
		t.Errorf("item[2] = %+v / err=%v", bresp.Items[2].Response, bresp.Items[2].Err)
	}
}
