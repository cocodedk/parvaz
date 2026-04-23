package dispatcher

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// recordingSNITunnel is the SNI equivalent of recordingInterceptor —
// signals each Tunnel call on a channel so tests can assert Path 2 routing.
type recordingSNITunnel struct {
	called chan interceptCall
}

func newRecordingSNITunnel() *recordingSNITunnel {
	return &recordingSNITunnel{called: make(chan interceptCall, 4)}
}

func (r *recordingSNITunnel) Tunnel(_ context.Context, rawConn net.Conn, host string, port uint16) error {
	r.called <- interceptCall{host: host, port: port}
	rawConn.Close()
	return nil
}

func TestDispatcher_DefaultSNIRewriteList_CoversVideoTargets(t *testing.T) {
	if !matchesPatternList("m.youtube.com", DefaultSNIRewriteList) {
		t.Error("m.youtube.com missing from DefaultSNIRewriteList")
	}
	if !matchesPatternList("i.ytimg.com", DefaultSNIRewriteList) {
		t.Error("i.ytimg.com missing from DefaultSNIRewriteList")
	}
	if !matchesPatternList("yt3.ggpht.com", DefaultSNIRewriteList) {
		t.Error("yt3.ggpht.com missing from DefaultSNIRewriteList")
	}
	// Google search / accounts / etc. belong on the allow-list, not here.
	if matchesPatternList("www.google.com", DefaultSNIRewriteList) {
		t.Error("www.google.com in SNIRewriteList — belongs on AllowList")
	}
}

func TestDispatcher_SNIRewriteHost_UsesSNITunnel(t *testing.T) {
	ic := newRecordingInterceptor()
	st := newRecordingSNITunnel()
	dialed := make(chan string, 1)
	d := &Dispatcher{
		AllowList:      []string{"*.google.com"},
		SNIRewriteList: []string{"*.youtube.com"},
		Interceptor:    ic,
		SNITunnel:      st,
		DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialed <- addr
			a, _ := net.Pipe()
			return a, nil
		},
	}

	conn, err := d.Dial(context.Background(), "m.youtube.com", 443)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	select {
	case call := <-st.called:
		if call.host != "m.youtube.com" || call.port != 443 {
			t.Errorf("SNITunnel got %s:%d, want m.youtube.com:443", call.host, call.port)
		}
	case <-time.After(time.Second):
		t.Fatal("SNITunnel not called within 1s for YouTube host")
	}

	select {
	case call := <-ic.called:
		t.Errorf("interceptor called for SNI-rewrite host: %+v", call)
	case addr := <-dialed:
		t.Errorf("direct dial for SNI-rewrite host: %q", addr)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDispatcher_SNIRewriteList_WithoutTunnel_ErrorsLoudly(t *testing.T) {
	// SNIRewriteList set but SNITunnel nil is almost certainly a wiring
	// mistake. A silent fallback to MITM+relay would burn Apps Script
	// quota on every YouTube asset with no signal to the operator —
	// prefer the loud error so the misconfig gets caught at startup.
	d := &Dispatcher{
		AllowList:      []string{"*.google.com"},
		SNIRewriteList: []string{"*.youtube.com"},
		Interceptor:    newRecordingInterceptor(),
		// SNITunnel: nil
	}
	_, err := d.Dial(context.Background(), "m.youtube.com", 443)
	if err == nil {
		t.Fatal("expected error on SNIRewriteList match without SNITunnel, got nil")
	}
	if !strings.Contains(err.Error(), "SNITunnel is nil") {
		t.Errorf("err = %v, want message mentioning nil SNITunnel", err)
	}
}
