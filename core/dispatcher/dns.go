package dispatcher

import (
	"context"
	"net"
	"strings"
)

// DNSTCPHandler is how the dispatcher routes TCP-fallback DNS (RFC 1035
// §4.2.2) to the parvazd DoH shim. A CONNECT to DNSHost:DNSPort returns
// a pipe whose other end is handed to this handler; resolver retries
// after a truncated UDP answer land here. Without this, a TC=1 UDP
// response would trigger TCP/53 that the MITM path can't serve.
type DNSTCPHandler interface {
	ServeTCP(ctx context.Context, conn net.Conn)
}

func (d *Dispatcher) isDNSTCPTarget(host string, port uint16) bool {
	if d.DNSTCP == nil || d.DNSHost == "" {
		return false
	}
	return port == d.DNSPort && strings.EqualFold(host, d.DNSHost)
}

// dialDNSTCP pipes the dispatcher's returned conn straight to the DoH
// shim via DNSTCPHandler.ServeTCP. No MITM, no relay — the handler
// reads length-prefixed DNS wire messages and writes the responses.
func (d *Dispatcher) dialDNSTCP(ctx context.Context) (net.Conn, error) {
	serverSide, clientSide := net.Pipe()
	go d.DNSTCP.ServeTCP(ctx, serverSide)
	return clientSide, nil
}
