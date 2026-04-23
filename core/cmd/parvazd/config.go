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
	// InsecureTLS disables certificate verification on the fronter
	// dialer. Strictly for local e2e against a self-signed Apps Script
	// stub — never flip this in production builds.
	InsecureTLS bool `json:"insecure_tls"`
}

const (
	defaultGoogleIP    = "216.239.38.120"
	defaultFrontDomain = "www.google.com"
	defaultFrontPort   = 443
	defaultListenHost  = "127.0.0.1"
	defaultListenPort  = 1080
	defaultDataDir     = "./parvaz-data"
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
	if over.InsecureTLS {
		// bool can't distinguish "unset" from "false"; only propagate
		// when stdin explicitly opts in to avoid silently unsetting a
		// flag-supplied true.
		base.InsecureTLS = true
	}
	if over.ListenHost != "" {
		base.ListenHost = over.ListenHost
	}
	if over.ListenPort != 0 {
		base.ListenPort = over.ListenPort
	}
	if over.DataDir != "" {
		base.DataDir = over.DataDir
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
	if c.DataDir == "" {
		return errors.New("data_dir required")
	}
	return nil
}
