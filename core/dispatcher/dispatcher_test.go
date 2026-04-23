package dispatcher

import (
	"context"
	"net"
	"testing"
	"time"
)

// interceptCall captures what the interceptor was called with.
type interceptCall struct {
	host string
	port uint16
}

// recordingInterceptor signals every Intercept call on a channel so tests
// can assert the dispatcher made exactly the routing decision they expect.
type recordingInterceptor struct {
	called chan interceptCall
}

func newRecordingInterceptor() *recordingInterceptor {
	return &recordingInterceptor{called: make(chan interceptCall, 4)}
}

func (r *recordingInterceptor) Intercept(_ context.Context, rawConn net.Conn, host string, port uint16) error {
	r.called <- interceptCall{host: host, port: port}
	rawConn.Close()
	return nil
}

func TestDispatcher_AllowListLookup_MatchesWildcards(t *testing.T) {
	d := &Dispatcher{AllowList: []string{
		"*.google.com",
		"*.googleapis.com",
		"exact.example.com",
	}}
	cases := []struct {
		host string
		want bool
	}{
		{"google.com", true},             // apex matches *.google.com
		{"www.google.com", true},         // single-label subdomain
		{"a.b.c.google.com", true},       // deep subdomain
		{"notgoogle.com", false},         // not a suffix match
		{"google.com.evil.com", false},   // embedded substring must not match
		{"googleapis.com", true},         // apex + second wildcard
		{"exact.example.com", true},      // exact match
		{"sub.exact.example.com", false}, // exact must not match subdomain
		{"foo.bar", false},               // unrelated
		{"GOOGLE.COM", true},             // case-insensitive
	}
	for _, tc := range cases {
		if got := d.matchesAllowList(tc.host); got != tc.want {
			t.Errorf("matchesAllowList(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestDispatcher_GoogleHost_UsesDirect(t *testing.T) {
	rec := newRecordingInterceptor()
	dialed := make(chan string, 1)
	d := &Dispatcher{
		AllowList:   []string{"*.google.com"},
		Interceptor: rec,
		DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialed <- addr
			a, _ := net.Pipe()
			return a, nil
		},
	}

	conn, err := d.Dial(context.Background(), "mail.google.com", 443)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	select {
	case addr := <-dialed:
		if addr != "mail.google.com:443" {
			t.Errorf("dialed %q, want mail.google.com:443", addr)
		}
	case <-time.After(time.Second):
		t.Fatal("DialContext not called within 1s")
	}

	// Interceptor must NOT have been called — Google host should go direct.
	select {
	case call := <-rec.called:
		t.Errorf("interceptor called for Google host: %+v", call)
	case <-time.After(100 * time.Millisecond):
		// expected silence
	}
}

func TestDispatcher_ArbitraryHost_UsesMITM(t *testing.T) {
	rec := newRecordingInterceptor()
	dialed := make(chan string, 1)
	d := &Dispatcher{
		AllowList:   []string{"*.google.com"},
		Interceptor: rec,
		DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialed <- addr
			a, _ := net.Pipe()
			return a, nil
		},
	}

	conn, err := d.Dial(context.Background(), "netflic.com", 443)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	select {
	case call := <-rec.called:
		if call.host != "netflic.com" || call.port != 443 {
			t.Errorf("interceptor got %s:%d, want netflic.com:443", call.host, call.port)
		}
	case <-time.After(time.Second):
		t.Fatal("interceptor not called within 1s for non-Google host")
	}

	// DialContext must NOT have been called — MITM path does not pre-dial.
	select {
	case addr := <-dialed:
		t.Errorf("DialContext called on MITM path: %q", addr)
	case <-time.After(100 * time.Millisecond):
		// expected silence
	}
}

func TestDispatcher_DefaultAllowList_Coverage(t *testing.T) {
	d := &Dispatcher{AllowList: DefaultAllowList}

	// Hosts that should take the direct path (not DPI-blocked).
	directHosts := []string{
		"www.google.com",
		"fonts.googleapis.com",
		"lh3.googleusercontent.com",
		"ssl.gstatic.com",
	}
	for _, h := range directHosts {
		if !d.matchesAllowList(h) {
			t.Errorf("default allow-list missed %q", h)
		}
	}

	// Hosts that must NOT be in DefaultAllowList — they are DPI-blocked
	// and belong on DefaultSNIRewriteList instead.
	dpiBlockedHosts := []string{
		"m.youtube.com",
		"i.ytimg.com",
		"yt3.ggpht.com",
	}
	for _, h := range dpiBlockedHosts {
		if d.matchesAllowList(h) {
			t.Errorf("default allow-list includes DPI-blocked host %q — belongs on SNIRewriteList", h)
		}
	}
}

