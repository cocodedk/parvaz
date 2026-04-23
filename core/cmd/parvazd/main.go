// parvazd — sidecar binary. The Kotlin app launches this via ProcessBuilder,
// pipes a JSON config on stdin, reads the single line "READY" on stdout,
// and connects to 127.0.0.1:<listen_port> as a SOCKS5 client.
//
// Each SOCKS5 CONNECT opens one WebSocket to the configured Cloudflare
// Worker, which pipes raw TCP bytes to the final destination.
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
	"syscall"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/relay"
	"github.com/cocodedk/parvaz/core/socks5"
)

var version = "dev"

// Config is the JSON document piped on stdin (or set via flags).
type Config struct {
	WorkerURL   string `json:"worker_url"`   // wss://<worker>.workers.dev/tunnel
	AuthKey     string `json:"auth_key"`     // shared secret with worker.js
	FrontIP     string `json:"front_ip"`     // Cloudflare edge IP to dial (TCP)
	FrontDomain string `json:"front_domain"` // TLS SNI (any popular CF-hosted site)
	ListenHost  string `json:"listen_host"`
	ListenPort  int    `json:"listen_port"`
}

const (
	// defaultFrontIP is a public Cloudflare anycast IP. Override via flag/stdin
	// when operating in regions where a specific IP is preferred.
	defaultFrontIP     = "104.16.132.229"
	defaultFrontDomain = "www.cloudflare.com"
	defaultListenHost  = "127.0.0.1"
	defaultListenPort  = 1080
)

func main() {
	if err := run(); err != nil {
		log.SetFlags(0)
		log.Fatalf("parvazd: %v", err)
	}
}

func run() error {
	var (
		useStdin     = flag.Bool("stdin", false, "read JSON config from stdin")
		workerURL    = flag.String("worker-url", "", "Cloudflare Worker WebSocket URL (wss://...)")
		authKey      = flag.String("auth-key", "", "shared secret with the worker")
		frontIP      = flag.String("front-ip", defaultFrontIP, "Cloudflare edge IP (TCP target)")
		frontDomain  = flag.String("front-domain", defaultFrontDomain, "TLS SNI (popular CF-hosted site)")
		listenHost   = flag.String("listen-host", defaultListenHost, "SOCKS5 listen host")
		listenPort   = flag.Int("listen-port", defaultListenPort, "SOCKS5 listen port")
		printVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *printVersion {
		fmt.Println("parvazd", version)
		return nil
	}

	cfg := Config{
		WorkerURL: *workerURL, AuthKey: *authKey,
		FrontIP: *frontIP, FrontDomain: *frontDomain,
		ListenHost: *listenHost, ListenPort: *listenPort,
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
		HTTPClient: client, WorkerURL: cfg.WorkerURL, AuthKey: cfg.AuthKey,
	})
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(cfg.ListenHost, fmt.Sprint(cfg.ListenPort)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	srv := &socks5.Server{Dialer: rel}
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
	d := &fronter.Dialer{FrontDomain: cfg.FrontDomain}
	target := net.JoinHostPort(cfg.FrontIP, "443")
	return fronter.NewHTTPClient(d, target)
}

func merge(base, over Config) Config {
	if over.WorkerURL != "" {
		base.WorkerURL = over.WorkerURL
	}
	if over.AuthKey != "" {
		base.AuthKey = over.AuthKey
	}
	if over.FrontIP != "" {
		base.FrontIP = over.FrontIP
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
	if c.WorkerURL == "" {
		return errors.New("worker_url required")
	}
	return nil
}
