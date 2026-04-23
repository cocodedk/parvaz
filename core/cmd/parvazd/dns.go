package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cocodedk/parvaz/core/doh"
	"github.com/cocodedk/parvaz/core/protocol"
)

// dnsHandler satisfies both socks5.DatagramHandler (UDP/53) and the
// dispatcher's TCP-DNS interface (TCP/53 fallback). One backing DoH
// client either way — so the two transports give identical answers.
//
// Wire: DoH → dns.google/dns-query, but transported via the Apps Script
// relay rather than a direct fronter dial. dns.google is NOT served on
// Google's Apps-front edge (216.239.38.120 returns 404), and dialing
// 8.8.8.8 directly is often DPI-blocked exactly because it's the well-
// known DNS IP. Routing through the relay keeps perfect cover — the
// network observer sees the same Google-edge + SNI=www.google.com
// pattern as every other tunneled request.
//
// Cost: one Apps Script quota unit per DNS query. Acceptable for alpha;
// future work can cache or bulk-batch.
//
// Policy summary:
//   - Only the synthetic VPN DNS host is served (codex P2). Queries to
//     specific resolvers like 1.1.1.1 are dropped, not silently
//     rewritten — prevents split-horizon / private-zone surprises.
//   - AAAA queries are answered NOERROR/empty locally (codex P2). The
//     TUN is IPv4-only; handing out AAAA records would let Chrome
//     Happy-Eyeballs onto paths the VPN can't carry.
//   - DoH failures become SERVFAIL synchronously (codex P2) instead of
//     silent drops that force Android's resolver to wait out its
//     multi-second retry budget.
type dnsHandler struct {
	syntheticIP string
	client      *doh.Client
	logger      *slog.Logger
}

// Relayer is what newDNSHandler needs from *relay.Relay. Declared here
// so the dns test can stub it without depending on the full relay
// package (and its fronter/HTTP plumbing).
type Relayer interface {
	Do(ctx context.Context, req protocol.Request) (*protocol.Response, error)
}

func newDNSHandler(rel Relayer, cfg Config, logger *slog.Logger) *dnsHandler {
	return &dnsHandler{
		syntheticIP: cfg.dnsHost(),
		client: &doh.Client{
			HTTP:     newDoHHTTPClient(rel),
			Endpoint: doh.DefaultEndpoint,
		},
		logger: logger,
	}
}

// Handle — socks5.DatagramHandler for UDP/53.
func (h *dnsHandler) Handle(ctx context.Context, dstHost string, dstPort uint16, payload []byte) ([]byte, error) {
	if !h.isSyntheticTarget(dstHost, dstPort) {
		h.logger.Debug("dns: dropping non-synthetic UDP",
			"dst", net.JoinHostPort(dstHost, fmt.Sprint(dstPort)))
		return nil, nil
	}
	return h.answer(ctx, payload), nil
}

// ServeTCP is invoked by the dispatcher when a CONNECT targets the
// synthetic DNS host:53. Implements RFC 1035 §4.2.2 TCP framing:
// 2-byte length prefix followed by the DNS message.
func (h *dnsHandler) ServeTCP(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		msg, err := readDNSTCPMessage(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !isTimeout(err) {
				h.logger.Debug("dns tcp: read", "err", err)
			}
			return
		}
		resp := h.answer(ctx, msg)
		if resp == nil {
			return
		}
		if err := writeDNSTCPMessage(conn, resp); err != nil {
			h.logger.Debug("dns tcp: write", "err", err)
			return
		}
	}
}

// answer applies the full DNS policy: AAAA-suppress → DoH → SERVFAIL.
func (h *dnsHandler) answer(ctx context.Context, query []byte) []byte {
	if qtype, ok := parseQueryQType(query); ok && qtype == dnsTypeAAAA {
		return synthesizeEmpty(query)
	}
	resp, err := h.client.Query(ctx, query)
	if err != nil {
		h.logger.Debug("dns: doh query failed, returning SERVFAIL", "err", err)
		return synthesizeSERVFAIL(query)
	}
	return resp
}

func (h *dnsHandler) isSyntheticTarget(host string, port uint16) bool {
	return port == dnsListenPort && strings.EqualFold(host, h.syntheticIP)
}

func readDNSTCPMessage(r io.Reader) ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint16(lenBuf[:])
	if n == 0 {
		return nil, errors.New("dns tcp: zero-length message")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeDNSTCPMessage(w io.Writer, msg []byte) error {
	if len(msg) > 0xFFFF {
		return fmt.Errorf("dns tcp: message too long (%d)", len(msg))
	}
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(msg)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(msg)
	return err
}

func isTimeout(err error) bool {
	ne, ok := err.(net.Error)
	return ok && ne.Timeout()
}

// newDoHHTTPClient builds an http.Client whose RoundTripper (see
// relay_rt.go) wraps each HTTP request into an Apps Script envelope
// via the shared relay. Code.gs fetches dns.google/dns-query and
// returns the wire response. Short 3s timeout so SERVFAIL fires fast
// rather than stalling page loads.
func newDoHHTTPClient(rel Relayer) *http.Client {
	return &http.Client{
		Transport: &relayRoundTripper{relay: rel},
		Timeout:   3 * time.Second,
	}
}
