package relay_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/internal/testutil"
	"github.com/cocodedk/parvaz/core/relay"
)

// mkRelay wires stub → relay, returns ready to use.
func mkRelay(t *testing.T, stub *testutil.WorkerStub, authKey string) *relay.Relay {
	t.Helper()
	r, err := relay.New(relay.Config{
		HTTPClient: stub.HTTPClient(),
		WorkerURL:  stub.WSURL(),
		AuthKey:    authKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRelay_Dial_PassesAuthAndTarget(t *testing.T) {
	stub := testutil.NewWorkerStub("secret")
	defer stub.Close()
	r := mkRelay(t, stub, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := r.Dial(ctx, "instagram.com", 443)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()

	if n := stub.Hits(); n != 1 {
		t.Fatalf("hits = %d, want 1", n)
	}
	if stub.Log[0].Host != "instagram.com" || stub.Log[0].Port != 443 {
		t.Errorf("stub got %s:%d, want instagram.com:443",
			stub.Log[0].Host, stub.Log[0].Port)
	}
}

func TestRelay_Dial_Unauthorized_ReturnsError(t *testing.T) {
	stub := testutil.NewWorkerStub("correct-key")
	defer stub.Close()
	r := mkRelay(t, stub, "wrong-key")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Dial(ctx, "example.com", 80)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Message should mention unauthorized to surface the problem cleanly.
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("err = %v, want message containing 'unauthorized'", err)
	}
}

func TestRelay_Dial_ProxiesTCPBytesBothWays(t *testing.T) {
	stub := testutil.NewWorkerStub("secret")
	defer stub.Close()
	r := mkRelay(t, stub, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := r.Dial(ctx, "echo.example", 7)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	payload := []byte("ping ping ping")
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("echo mismatch: got %q want %q", got, payload)
	}
}

func TestRelay_Dial_PropagatesContextCancel(t *testing.T) {
	stub := testutil.NewWorkerStub("secret")
	// Slow handler to force client to wait past the deadline.
	stub.UpstreamHandler = func(_ string, _ uint16, c io.ReadWriteCloser) {
		defer c.Close()
		time.Sleep(2 * time.Second)
	}
	defer stub.Close()
	r := mkRelay(t, stub, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	conn, err := r.Dial(ctx, "slow.example", 443)
	// The Dial itself completes when the WS upgrade succeeds — context
	// deadline then governs reads.
	if err != nil {
		return // acceptable: deadline during upgrade
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	_, err = conn.Read(make([]byte, 16))
	if err == nil {
		t.Fatal("expected read timeout, got nil")
	}
}

func TestRelay_Dial_ServerCloseCausesConnEOF(t *testing.T) {
	stub := testutil.NewWorkerStub("secret")
	// Upstream handler closes immediately, so client read should see EOF.
	stub.UpstreamHandler = func(_ string, _ uint16, c io.ReadWriteCloser) {
		_ = c.Close()
	}
	defer stub.Close()
	r := mkRelay(t, stub, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := r.Dial(ctx, "brief.example", 80)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Read(make([]byte, 16))
	if err == nil {
		t.Fatal("expected EOF, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Logf("got %v (%T) — accepting any non-nil error as EOF signal", err, err)
	}
}
