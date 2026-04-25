package dispatcher

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

type fakeDNSTCP struct {
	served chan net.Conn
}

func (f *fakeDNSTCP) ServeTCP(_ context.Context, conn net.Conn) {
	f.served <- conn
	// Minimal echo: read one byte, write two, close. Lets the test
	// confirm the pipe flows both ways.
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		_, _ = conn.Write([]byte{buf[0], buf[0]})
	}
	_ = conn.Close()
}

func TestDispatcher_DNSTarget_RoutesToDNSTCPHandler(t *testing.T) {
	f := &fakeDNSTCP{served: make(chan net.Conn, 1)}
	rec := newRecordingInterceptor()
	d := &Dispatcher{
		Interceptor: rec,
		DNSTCP:      f,
		DNSHost:     "10.0.0.2",
		DNSPort:     53,
	}

	conn, err := d.Dial(context.Background(), "10.0.0.2", 53)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	select {
	case <-f.served:
		// expected
	case <-time.After(time.Second):
		t.Fatal("DNSTCPHandler not invoked within 1s")
	}

	// Interceptor must NOT have been called.
	select {
	case call := <-rec.called:
		t.Errorf("interceptor called for DNS target: %+v", call)
	case <-time.After(50 * time.Millisecond):
		// expected silence
	}

	// Confirm pipe carries bytes end-to-end (dispatcher's caller writes,
	// DNSTCPHandler reads; handler writes, caller reads).
	go conn.Write([]byte{0x42})
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 || got[0] != 0x42 || got[1] != 0x42 {
		t.Errorf("pipe round-trip got %v, want [42 42]", got)
	}
}

func TestDispatcher_DNSPortOnDifferentHost_FallsThroughToMITM(t *testing.T) {
	f := &fakeDNSTCP{served: make(chan net.Conn, 1)}
	rec := newRecordingInterceptor()
	d := &Dispatcher{
		Interceptor: rec,
		DNSTCP:      f,
		DNSHost:     "10.0.0.2",
		DNSPort:     53,
	}

	conn, err := d.Dial(context.Background(), "1.1.1.1", 53)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// DNSTCP must NOT have been invoked; MITM interceptor must have.
	select {
	case <-f.served:
		t.Error("DNSTCP invoked for non-synthetic DNS IP")
	case call := <-rec.called:
		if call.host != "1.1.1.1" || call.port != 53 {
			t.Errorf("interceptor got %s:%d, want 1.1.1.1:53", call.host, call.port)
		}
	case <-time.After(time.Second):
		t.Fatal("neither path invoked within 1s")
	}
}

func TestDispatcher_DNSTCPNilHandler_BypassesRoute(t *testing.T) {
	rec := newRecordingInterceptor()
	d := &Dispatcher{
		Interceptor: rec,
		DNSHost:     "10.0.0.2",
		DNSPort:     53,
		// DNSTCP intentionally nil
	}
	conn, err := d.Dial(context.Background(), "10.0.0.2", 53)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	select {
	case <-rec.called:
		// expected — falls through to MITM when DNSTCP is unset
	case <-time.After(time.Second):
		t.Fatal("interceptor not called within 1s")
	}
}
