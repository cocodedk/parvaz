// parvazd — sidecar binary. The Kotlin app launches this via ProcessBuilder,
// pipes a JSON config on stdin, reads the single line "READY" on stdout,
// and connects to 127.0.0.1:<listen_port> as a SOCKS5 client.
//
// HTTP-over-SOCKS5 bridging (turn raw TCP flows into Apps Script fetches) is
// not yet wired — this binary stands up the network shell so Phase B can
// validate the launcher. The actual bridge arrives in a follow-up milestone.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/relay"
	"github.com/cocodedk/parvaz/core/socks5"
)

var version = "dev"

// Config mirrors reference/src/config.example.json. Fields can be supplied
// via flags or via a JSON document on stdin; stdin wins on overlap.
type Config struct {
	ScriptURLs  []string `json:"script_urls"`
	AuthKey     string   `json:"auth_key"`
	GoogleIP    string   `json:"google_ip"`
	FrontDomain string   `json:"front_domain"`
	ListenHost  string   `json:"listen_host"`
	ListenPort  int      `json:"listen_port"`
}

const (
	defaultGoogleIP    = "216.239.38.120"
	defaultFrontDomain = "www.google.com"
	defaultListenHost  = "127.0.0.1"
	defaultListenPort  = 1080
)

// stubDialer rejects every CONNECT until the HTTP-over-SOCKS5 bridge exists.
// Kept here so the process listens and Phase B's launcher can probe it.
type stubDialer struct{}

func (stubDialer) Dial(_ context.Context, host string, port uint16) (net.Conn, error) {
	return nil, fmt.Errorf("parvazd: CONNECT %s:%d — bridge not yet implemented", host, port)
}

func main() {
	if err := run(); err != nil {
		log.SetFlags(0)
		log.Fatalf("parvazd: %v", err)
	}
}

func run() error {
	var (
		useStdin     = flag.Bool("stdin", false, "read JSON config from stdin")
		scriptURLs   = flag.String("script-urls", "", "comma-separated Apps Script deployment URLs")
		authKey      = flag.String("auth-key", "", "shared secret with Code.gs")
		googleIP     = flag.String("google-ip", defaultGoogleIP, "TCP target (Google front IP)")
		frontDomain  = flag.String("front-domain", defaultFrontDomain, "TLS SNI")
		listenHost   = flag.String("listen-host", defaultListenHost, "SOCKS5 listen host")
		listenPort   = flag.Int("listen-port", defaultListenPort, "SOCKS5 listen port")
		printVersion = flag.Bool("version", false, "print version and exit")
		logLevelStr  = flag.String("log-level", "warn", "slog level: debug|info|warn|error")
	)
	flag.Parse()
	if *printVersion {
		fmt.Println("parvazd", version)
		return nil
	}

	logger, err := newLogger(*logLevelStr)
	if err != nil {
		return err
	}

	cfg := Config{
		GoogleIP: *googleIP, FrontDomain: *frontDomain,
		ListenHost: *listenHost, ListenPort: *listenPort,
		AuthKey: *authKey,
	}
	if *scriptURLs != "" {
		for _, u := range strings.Split(*scriptURLs, ",") {
			if u = strings.TrimSpace(u); u != "" {
				cfg.ScriptURLs = append(cfg.ScriptURLs, u)
			}
		}
	}
	if *useStdin {
		var fromStdin Config
		if err := json.NewDecoder(os.Stdin).Decode(&fromStdin); err != nil {
			return fmt.Errorf("parse stdin config: %w", err)
		}
		cfg = merge(cfg, fromStdin)
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	client := buildHTTPClient(cfg)
	rel, err := relay.New(relay.Config{
		HTTPClient: client, ScriptURLs: cfg.ScriptURLs, AuthKey: cfg.AuthKey,
	})
	if err != nil {
		return err
	}
	_ = rel // future HTTP-over-SOCKS5 bridge will consume this

	ln, err := net.Listen("tcp", net.JoinHostPort(cfg.ListenHost, fmt.Sprint(cfg.ListenPort)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	srv := &socks5.Server{Dialer: stubDialer{}, Logger: logger}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintln(os.Stdout, "READY")
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ctx, ln) }()

	select {
	case err := <-errc:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	case <-ctx.Done():
	}
	return nil
}

func buildHTTPClient(cfg Config) *http.Client {
	d := &fronter.Dialer{
		FrontDomain:      cfg.FrontDomain,
		DialTimeout:      10 * time.Second,
		HandshakeTimeout: 10 * time.Second,
	}
	target := net.JoinHostPort(cfg.GoogleIP, "443")
	return fronter.NewHTTPClient(d, target)
}

func merge(base, over Config) Config {
	if len(over.ScriptURLs) > 0 {
		base.ScriptURLs = over.ScriptURLs
	}
	if over.AuthKey != "" {
		base.AuthKey = over.AuthKey
	}
	if over.GoogleIP != "" {
		base.GoogleIP = over.GoogleIP
	}
	if over.FrontDomain != "" {
		base.FrontDomain = over.FrontDomain
	}
	if over.ListenHost != "" {
		base.ListenHost = over.ListenHost
	}
	if over.ListenPort != 0 {
		base.ListenPort = over.ListenPort
	}
	return base
}

func (c Config) validate() error {
	if c.AuthKey == "" {
		return errors.New("auth_key required")
	}
	if len(c.ScriptURLs) == 0 {
		return errors.New("at least one script_url required")
	}
	return nil
}
