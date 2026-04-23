package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"time"

	"github.com/cocodedk/parvaz/core/dispatcher"
	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/mitm"
	"github.com/cocodedk/parvaz/core/relay"
	"github.com/cocodedk/parvaz/core/socks5"
)

// buildPipeline wires the full request path:
//
//	socks5.Server → dispatcher.Dispatcher
//	                  ├─ direct TCP       (AllowList: accounts/mail/gmail/etc.)
//	                  ├─ SNI-rewrite      (SNIRewriteList: YouTube / ytimg / ggpht)
//	                  └─ MITM + relay     (everything else → Apps Script)
//
// Returns a socks5.Server ready to Serve. The only persistent state is
// the CA under cfg.DataDir.
func buildPipeline(cfg Config, logger *slog.Logger) (*socks5.Server, error) {
	client := buildHTTPClient(cfg)
	rel, err := relay.New(relay.Config{
		HTTPClient: client, ScriptURLs: cfg.ScriptURLs, AuthKey: cfg.AuthKey,
	})
	if err != nil {
		return nil, fmt.Errorf("relay: %w", err)
	}
	// Resolve DataDir to absolute so running parvazd from a different CWD
	// can't silently generate a second CA — Android's installed user-root
	// would no longer chain to it and every MITM would fail with
	// untrusted-cert errors.
	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data_dir: %w", err)
	}
	ca, err := mitm.LoadOrCreate(dataDir)
	if err != nil {
		return nil, fmt.Errorf("mitm ca: %w", err)
	}

	interceptor := &mitm.Interceptor{CA: ca, Relay: rel, Logger: logger}

	// A dedicated fronter for the SNI-rewrite path — same FrontDomain as
	// the Apps Script client (so DPI sees the same SNI in either leg) but
	// independent so transport tuning can diverge later (e.g. no h2 ALPN
	// flags for the relay leg vs. h1-only here).
	//
	// InsecureTLS applies here too so local e2e with a self-signed upstream
	// still handshakes. FrontPort does NOT apply — the SNI-rewrite path
	// dials the browser's original port (usually 443), not the relay port.
	sniFronter := &fronter.Dialer{
		FrontDomain:        cfg.FrontDomain,
		InsecureSkipVerify: cfg.InsecureTLSEnabled(),
		DialTimeout:        10 * time.Second,
		HandshakeTimeout:   10 * time.Second,
	}
	sniTunnel := &mitm.SNITunnel{
		CA: ca,
		UpstreamDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sniFronter.Dial(ctx, network, addr)
		},
		UpstreamIP: cfg.GoogleIP,
		Logger:     logger,
	}

	disp := &dispatcher.Dispatcher{
		AllowList:      dispatcher.DefaultAllowList,
		SNIRewriteList: dispatcher.DefaultSNIRewriteList,
		Interceptor:    interceptor,
		SNITunnel:      sniTunnel,
		Logger:         logger,
	}
	return &socks5.Server{Dialer: disp, Logger: logger}, nil
}
