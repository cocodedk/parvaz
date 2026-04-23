package main

import "errors"

// Config mirrors reference/src/config.example.json. Fields can be supplied
// via flags or via a JSON document on stdin; stdin wins on overlap.
type Config struct {
	ScriptURLs  []string `json:"script_urls"`
	AuthKey     string   `json:"auth_key"`
	GoogleIP    string   `json:"google_ip"`
	FrontDomain string   `json:"front_domain"`
	FrontPort   int      `json:"front_port"`
	ListenHost  string   `json:"listen_host"`
	ListenPort  int      `json:"listen_port"`
	DataDir     string   `json:"data_dir"`
	// TunFD is the Android TUN file descriptor inherited from the Kotlin
	// VpnService via ProcessBuilder. When > 0 the sidecar runs tun2socks
	// on it; the loopback SOCKS5 listener is skipped because traffic
	// arrives through the TUN instead. 0/absent means "no TUN, legacy
	// SOCKS5 mode" — still used by the e2e harness and integration tests.
	TunFD int `json:"tun_fd"`
	// TunMTU mirrors what VpnService.Builder.setMtu() was given. Must
	// match or tun2socks will fragment / misread packets.
	TunMTU int `json:"tun_mtu"`
	// DNSListenHost is the IPv4 literal Android advertises via
	// VpnService.addDnsServer — the "server" parvazd's DoH shim answers
	// for. Default "10.0.0.2" (see ParvazVpnService.DNS_SERVER). Any
	// query NOT addressed to this host+53 is dropped: we won't
	// silently rewrite apps that hit their own resolver (1.1.1.1,
	// split-horizon corporate DNS, etc.) with Google's answer.
	DNSListenHost string `json:"dns_listen_host,omitempty"`
	// InsecureTLS disables certificate verification on every fronter
	// (relay path + SNI-rewrite path). Strictly for local e2e against
	// a self-signed Apps Script stub — never flip this in production.
	//
	// Pointer so JSON can distinguish "field absent" (nil) from
	// "field explicitly false" (*false). Stdin wins on overlap as
	// documented — stdin {"insecure_tls": false} turns off a
	// flag-supplied -insecure-tls=true.
	InsecureTLS *bool `json:"insecure_tls,omitempty"`
}

// InsecureTLSEnabled reports whether InsecureTLS is set and true.
// Nil-safe so call sites don't need to guard.
func (c Config) InsecureTLSEnabled() bool {
	return c.InsecureTLS != nil && *c.InsecureTLS
}

const (
	defaultGoogleIP    = "216.239.38.120"
	defaultFrontDomain = "www.google.com"
	defaultFrontPort   = 443
	defaultListenHost  = "127.0.0.1"
	defaultListenPort    = 1080
	defaultDataDir       = "./parvaz-data"
	defaultDNSListenHost = "10.0.0.2"
	dnsListenPort        = 53
)

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
	if over.FrontPort != 0 {
		base.FrontPort = over.FrontPort
	}
	if over.InsecureTLS != nil {
		// Pointer distinguishes "stdin silent" (nil) from "stdin false"
		// (*false); we honour either direction — stdin always wins on
		// overlap.
		base.InsecureTLS = over.InsecureTLS
	}
	if over.ListenHost != "" {
		base.ListenHost = over.ListenHost
	}
	if over.ListenPort != 0 {
		base.ListenPort = over.ListenPort
	}
	if over.TunFD != 0 {
		base.TunFD = over.TunFD
	}
	if over.TunMTU != 0 {
		base.TunMTU = over.TunMTU
	}
	if over.DataDir != "" {
		base.DataDir = over.DataDir
	}
	if over.DNSListenHost != "" {
		base.DNSListenHost = over.DNSListenHost
	}
	return base
}

// dnsHost returns the configured synthetic DNS host, defaulting to
// defaultDNSListenHost when unset. Centralised so every call site agrees.
func (c Config) dnsHost() string {
	if c.DNSListenHost != "" {
		return c.DNSListenHost
	}
	return defaultDNSListenHost
}

func (c Config) validate() error {
	if c.AuthKey == "" {
		return errors.New("auth_key required")
	}
	if len(c.ScriptURLs) == 0 {
		return errors.New("at least one script_url required")
	}
	if c.DataDir == "" {
		return errors.New("data_dir required")
	}
	return nil
}
