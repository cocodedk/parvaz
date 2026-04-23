package main

import "testing"

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty", Config{}, true},
		{"auth only", Config{AuthKey: "k"}, true},
		{"worker only", Config{WorkerURL: "wss://x"}, true},
		{"complete", Config{AuthKey: "k", WorkerURL: "wss://x"}, false},
	}
	for _, tc := range cases {
		err := tc.cfg.validate()
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

func TestMerge_StdinOverridesFlagDefaults(t *testing.T) {
	base := Config{
		FrontIP: defaultFrontIP, FrontDomain: defaultFrontDomain,
		ListenHost: defaultListenHost, ListenPort: defaultListenPort,
	}
	stdin := Config{
		WorkerURL: "wss://relay.example/tunnel",
		AuthKey:   "secret-from-stdin",
		FrontIP:   "1.1.1.1",
	}
	got := merge(base, stdin)
	if got.WorkerURL != stdin.WorkerURL {
		t.Errorf("WorkerURL = %q, want %q", got.WorkerURL, stdin.WorkerURL)
	}
	if got.AuthKey != stdin.AuthKey {
		t.Errorf("AuthKey not merged")
	}
	if got.FrontIP != "1.1.1.1" {
		t.Errorf("FrontIP = %q, want 1.1.1.1", got.FrontIP)
	}
	// Flag defaults preserved when stdin is silent
	if got.FrontDomain != defaultFrontDomain {
		t.Errorf("FrontDomain lost: %q", got.FrontDomain)
	}
	if got.ListenPort != defaultListenPort {
		t.Errorf("ListenPort lost: %d", got.ListenPort)
	}
}
