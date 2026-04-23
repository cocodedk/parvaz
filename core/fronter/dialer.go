// Package fronter opens TLS connections with a split between the TCP dial
// target and the TLS SNI — the primitive that powers domain fronting.
//
// A network observer (DPI, ISP, firewall) sees a TLS session for whatever
// domain FrontDomain names. Google's edge load balancer, once TLS is
// terminated, routes the underlying HTTP by Host: header — so we can dial a
// well-known Google IP, SNI `www.google.com`, and Host `script.google.com`.
//
// This package never parses JSON and never speaks HTTP; it shuttles bytes.
package fronter

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
)

// Dialer opens TLS connections with a custom SNI, independent of the dial target.
type Dialer struct {
	// FrontDomain is the server name sent in the TLS ClientHello SNI
	// extension. This is what a DPI box sees.
	FrontDomain string

	// InsecureSkipVerify disables certificate-chain validation.
	// Tests set this to true; production must leave it false — Google's
	// own cert chain for `www.google.com` validates normally.
	InsecureSkipVerify bool

	// BaseDialer is the underlying TCP dialer. If nil, net.Dialer{} is used.
	BaseDialer *net.Dialer

	// TLSConfigHook allows callers to observe / modify the tls.Config used
	// for the handshake. Primarily for tests. Applied after ServerName and
	// InsecureSkipVerify are set.
	TLSConfigHook func(*tls.Config)
}

// Dial connects to addr over network and performs a TLS handshake presenting
// d.FrontDomain as the SNI. network must be "tcp".
func (d *Dialer) Dial(ctx context.Context, network, addr string) (*tls.Conn, error) {
	if d.FrontDomain == "" {
		return nil, errors.New("fronter: FrontDomain required")
	}
	base := d.BaseDialer
	if base == nil {
		base = &net.Dialer{}
	}
	rawConn, err := base.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		ServerName:         d.FrontDomain,
		InsecureSkipVerify: d.InsecureSkipVerify, //nolint:gosec // production must set false
	}
	if d.TLSConfigHook != nil {
		d.TLSConfigHook(tlsCfg)
	}
	tlsConn := tls.Client(rawConn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}
