package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDNSHandler_UDP_DropsNonSyntheticTarget(t *testing.T) {
	srv := stubDoHServer(t, fakeDNSResponse(0x00))
	defer srv.Close()
	h := newTestHandler(srv.URL)

	// Apps hitting 1.1.1.1 or any other DNS IP directly must NOT be
	// silently rewritten to Google. Codex-review P2.
	resp, err := h.Handle(context.Background(), "1.1.1.1", 53, fakeDNSQuery(0xAB))
	if err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if resp != nil {
		t.Errorf("non-synthetic target should drop, got %d bytes", len(resp))
	}
}

func TestDNSHandler_UDP_DropsNonDNSPort(t *testing.T) {
	srv := stubDoHServer(t, fakeDNSResponse(0x00))
	defer srv.Close()
	h := newTestHandler(srv.URL)

	resp, err := h.Handle(context.Background(), "10.0.0.2", 80, []byte("not dns"))
	if err != nil {
		t.Fatalf("Handle err: %v", err)
	}
	if resp != nil {
		t.Errorf("non-53 port should drop, got %d bytes", len(resp))
	}
}

func TestDNSHandler_UDP_SyntheticTargetForwardsToDoH(t *testing.T) {
	srv := stubDoHServer(t, fakeDNSResponse(0x00))
	defer srv.Close()
	h := newTestHandler(srv.URL)

	resp, err := h.Handle(context.Background(), "10.0.0.2", 53, fakeDNSQuery(0xAB))
	if err != nil {
		t.Fatalf("Handle err: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("synthetic target produced empty response")
	}
	// doh.Client restores the query's ID; our stub returned 0, so
	// the response's first two bytes should be the query's.
	if resp[0] != 0xAB {
		t.Errorf("response ID not restored: got %02x%02x", resp[0], resp[1])
	}
}

func TestDNSHandler_TCP_FramesAndDoHRoundTrip(t *testing.T) {
	canned := fakeDNSResponse(0x00)
	srv := stubDoHServer(t, canned)
	defer srv.Close()
	h := newTestHandler(srv.URL)

	clientSide, serverSide := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeTCP(ctx, serverSide)
	}()

	query := fakeDNSQuery(0xCD)
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(query)))
	if _, err := clientSide.Write(lenBuf[:]); err != nil {
		t.Fatalf("write len: %v", err)
	}
	if _, err := clientSide.Write(query); err != nil {
		t.Fatalf("write body: %v", err)
	}

	_ = clientSide.SetReadDeadline(time.Now().Add(3 * time.Second))
	var respLenBuf [2]byte
	if _, err := io.ReadFull(clientSide, respLenBuf[:]); err != nil {
		t.Fatalf("read len: %v", err)
	}
	n := binary.BigEndian.Uint16(respLenBuf[:])
	body := make([]byte, n)
	if _, err := io.ReadFull(clientSide, body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	// doh.Client zeroes both ID bytes on the way in and restores
	// both on the way out, so expected[0..2] == query[0..2].
	expected := append([]byte(nil), canned...)
	expected[0] = query[0]
	expected[1] = query[1]
	if !bytes.Equal(body, expected) {
		t.Errorf("tcp response = %x, want %x", body, expected)
	}

	_ = clientSide.Close()
	wg.Wait()
}

func TestDNSHandler_AAAAQuery_SuppressedLocally(t *testing.T) {
	var dohHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dohHit = true
		_, _ = w.Write(fakeDNSResponse(0x00))
	}))
	defer srv.Close()
	h := newTestHandler(srv.URL)

	query := fakeAAAAQuery(0xAA)
	resp, err := h.Handle(context.Background(), "10.0.0.2", 53, query)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if dohHit {
		t.Error("AAAA query reached DoH — should be answered locally with empty NOERROR")
	}
	if len(resp) < 12 {
		t.Fatalf("response too short: %d bytes", len(resp))
	}
	if resp[0] != query[0] || resp[1] != query[1] {
		t.Errorf("response ID %02x%02x != query ID %02x%02x", resp[0], resp[1], query[0], query[1])
	}
	if resp[2]&0x80 == 0 {
		t.Error("QR bit not set in synthesised response")
	}
	if resp[3]&0x0F != 0 {
		t.Errorf("RCODE should be 0 (NOERROR) for AAAA-suppress, got %d", resp[3]&0x0F)
	}
	if binary.BigEndian.Uint16(resp[6:8]) != 0 {
		t.Errorf("ANCOUNT should be 0, got %d", binary.BigEndian.Uint16(resp[6:8]))
	}
}

func TestDNSHandler_DoHFailure_SynthesizesSERVFAIL(t *testing.T) {
	// Upstream unavailable → every POST fails. Our handler should answer
	// SERVFAIL synchronously so the resolver bails instead of waiting out
	// its multi-second retry budget.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	h := newTestHandler(srv.URL)

	query := fakeDNSQuery(0xAB)
	resp, err := h.Handle(context.Background(), "10.0.0.2", 53, query)
	if err != nil {
		t.Fatalf("Handle returned unexpected err: %v", err)
	}
	if len(resp) < 12 {
		t.Fatalf("response too short: %d bytes", len(resp))
	}
	if rcode := resp[3] & 0x0F; rcode != 2 {
		t.Errorf("RCODE = %d, want 2 (SERVFAIL)", rcode)
	}
	if resp[0] != query[0] || resp[1] != query[1] {
		t.Errorf("response ID not mirrored: got %02x%02x, want %02x%02x",
			resp[0], resp[1], query[0], query[1])
	}
}
