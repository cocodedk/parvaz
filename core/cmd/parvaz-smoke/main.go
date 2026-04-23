// parvaz-smoke — validate the Cloudflare Worker + fronter end-to-end.
//
// Two modes, exercised independently:
//
//	-direct    : dial the Worker URL as-is. Proves cloudflare:sockets
//	             + worker.js work at all.
//	-fronted   : dial a Cloudflare edge IP with SNI=www.cloudflare.com
//	             and the Worker URL's Host header. Proves Cloudflare
//	             routes by Host under SNI mismatch (domain fronting).
//
// Either mode: open a tunnel to <target>, send a raw HTTP/1.1 GET, print
// the response status line + first 256 body bytes. Non-zero exit on any
// failure.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/relay"
)

func main() {
	var (
		workerURL   = flag.String("worker-url", "", "wss://<your-worker>.workers.dev/tunnel")
		authKey     = flag.String("auth-key", "", "worker ACCESS_KEY")
		target      = flag.String("target", "example.com:80", "target host:port to tunnel")
		httpPath    = flag.String("path", "/", "HTTP path to GET once tunneled")
		fronted     = flag.Bool("fronted", false, "use fronter (dial CF edge IP with SNI=www.cloudflare.com)")
		frontIP     = flag.String("front-ip", "104.16.132.229", "Cloudflare edge IP when fronted")
		frontDomain = flag.String("front-domain", "www.cloudflare.com", "TLS SNI when fronted")
		timeout     = flag.Duration("timeout", 15*time.Second, "overall timeout")
	)
	flag.Parse()

	if *workerURL == "" || *authKey == "" {
		log.Fatal("usage: -worker-url=wss://... -auth-key=... [-fronted]")
	}

	var httpClient *http.Client
	if *fronted {
		d := &fronter.Dialer{FrontDomain: *frontDomain}
		edge := net.JoinHostPort(*frontIP, "443")
		httpClient = fronter.NewHTTPClient(d, edge)
		fmt.Printf("[mode] FRONTED · dial %s · SNI=%s · Host from worker URL\n", edge, *frontDomain)
	} else {
		httpClient = &http.Client{}
		fmt.Printf("[mode] DIRECT · dial worker URL as-is (no fronting)\n")
	}

	rel, err := relay.New(relay.Config{
		HTTPClient: httpClient,
		WorkerURL:  *workerURL,
		AuthKey:    *authKey,
	})
	if err != nil {
		log.Fatalf("relay.New: %v", err)
	}

	host, port, err := splitHostPort(*target)
	if err != nil {
		log.Fatalf("target: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Printf("[dial] opening tunnel to %s:%d via worker...\n", host, port)
	conn, err := rel.Dial(ctx, host, port)
	if err != nil {
		log.Fatalf("relay.Dial: %v", err)
	}
	defer conn.Close()
	fmt.Println("[dial] ✓ tunnel open")

	_ = conn.SetDeadline(time.Now().Add(*timeout))

	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: parvaz-smoke/0.1\r\nConnection: close\r\n\r\n",
		*httpPath, host,
	)
	if _, err := conn.Write([]byte(req)); err != nil {
		log.Fatalf("write request: %v", err)
	}
	fmt.Printf("[send] GET %s HTTP/1.1 · Host: %s · (%d bytes)\n", *httpPath, host, len(req))

	r := bufio.NewReader(conn)
	statusLine, err := r.ReadString('\n')
	if err != nil {
		log.Fatalf("read status: %v", err)
	}
	fmt.Printf("[resp] %s", statusLine)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if line == "\r\n" || line == "\n" {
			break
		}
		fmt.Printf("[hdr] %s", strings.TrimRight(line, "\r\n")+"\n")
	}

	head := make([]byte, 256)
	n, _ := io.ReadFull(r, head)
	fmt.Printf("[body] first %d bytes: %q\n", n, head[:n])
	fmt.Println("[done] smoke OK")
}

func splitHostPort(s string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, err
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("parse port: %w", err)
	}
	return host, uint16(p), nil
}
