package main

import "testing"

func TestConfig_Validate(t *testing.T) {
	ok := Config{
		AuthKey:    "k",
		ScriptURLs: []string{"https://x/exec"},
		DataDir:    "/tmp/parvaz-data",
	}
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty", Config{}, true},
		{"auth only", Config{AuthKey: "k"}, true},
		{"scripts only", Config{ScriptURLs: []string{"https://x/exec"}}, true},
		{"no data_dir", Config{AuthKey: "k", ScriptURLs: []string{"https://x/exec"}}, true},
		{"complete", ok, false},
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
		GoogleIP: defaultGoogleIP, FrontDomain: defaultFrontDomain,
		ListenHost: defaultListenHost, ListenPort: defaultListenPort,
	}
	stdin := Config{
		ScriptURLs: []string{"https://script.google.com/macros/s/ABC/exec"},
		AuthKey:    "secret-from-stdin",
		GoogleIP:   "64.233.160.0",
		DataDir:    "/var/lib/parvaz",
	}
	got := merge(base, stdin)
	if got.DataDir != "/var/lib/parvaz" {
		t.Errorf("DataDir not merged: %q", got.DataDir)
	}
	if len(got.ScriptURLs) != 1 || got.ScriptURLs[0] != stdin.ScriptURLs[0] {
		t.Errorf("ScriptURLs = %v, want %v", got.ScriptURLs, stdin.ScriptURLs)
	}
	if got.AuthKey != stdin.AuthKey {
		t.Errorf("AuthKey not merged")
	}
	if got.GoogleIP != "64.233.160.0" {
		t.Errorf("GoogleIP = %q, want 64.233.160.0", got.GoogleIP)
	}
	// Flag defaults preserved when stdin is silent
	if got.FrontDomain != defaultFrontDomain {
		t.Errorf("FrontDomain lost: %q", got.FrontDomain)
	}
	if got.ListenPort != defaultListenPort {
		t.Errorf("ListenPort lost: %d", got.ListenPort)
	}
}
