package main

import "testing"

func TestSplitHostPort(t *testing.T) {
	host, port, err := splitHostPort("example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if host != "example.com" || port != 443 {
		t.Errorf("got %s:%d", host, port)
	}
}

func TestSplitHostPort_BadPort(t *testing.T) {
	if _, _, err := splitHostPort("example.com:abc"); err == nil {
		t.Error("expected error")
	}
}
