package config

import (
	"encoding/base64"
	"reflect"
	"testing"
)

// fakeInputs implements InputSource backed by a map of input name to value.
type fakeInputs map[string]string

func (f fakeInputs) GetInput(name string) string { return f[name] }

// validInputs returns the minimum required inputs for Load to succeed.
func validInputs() fakeInputs {
	return fakeInputs{
		"github-app-id":          "123",
		"github-installation-id": "456",
		"github-private-key":     base64.StdEncoding.EncodeToString([]byte("helloworld")),
		"google-credentials":     base64.StdEncoding.EncodeToString([]byte(`{"client_email":"foo","private_key":"bar"}`)),
	}
}

func TestBoolVal(t *testing.T) {
	cases := map[string]bool{"": false, "foobar": false, "false": false, "true": true, "tRuE": true}
	for in, want := range cases {
		inputs := validInputs()
		if in != "" {
			inputs["add-users"] = in
		}
		cfg, err := Load(inputs)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.AddUsers != want {
			t.Errorf("add-users=%q => %v, want %v", in, cfg.AddUsers, want)
		}
	}
}

func TestExitCodeOnMismatch(t *testing.T) {
	cases := map[string]int{"": 0, "hello": 0, "123": 123}
	for in, want := range cases {
		inputs := validInputs()
		if in != "" {
			inputs["exit-code-on-mismatch"] = in
		}
		cfg, _ := Load(inputs)
		if cfg.ExitCodeOnMismatch != want {
			t.Errorf("exit-code-on-mismatch=%q => %d, want %d", in, cfg.ExitCodeOnMismatch, want)
		}
	}
}

func TestMaxRemoveUsersDefault(t *testing.T) {
	inputs := validInputs()
	cfg, _ := Load(inputs)
	if cfg.MaxRemoveUsers != 100 {
		t.Errorf("default MaxRemoveUsers = %d, want 100", cfg.MaxRemoveUsers)
	}
	inputs["max-remove-users"] = "5"
	cfg, _ = Load(inputs)
	if cfg.MaxRemoveUsers != 5 {
		t.Errorf("MaxRemoveUsers = %d, want 5", cfg.MaxRemoveUsers)
	}
}

func TestIgnoredUsers(t *testing.T) {
	cases := map[string][]string{
		"":                    nil,
		"user1":               {"user1"},
		"user1,user2,USER4":   {"user1", "user2", "user4"},
		"user1, user2 ,USER3": {"user1", "user2", "user3"},
	}
	for in, want := range cases {
		inputs := validInputs()
		if in != "" {
			inputs["ignored-users"] = in
		}
		cfg, _ := Load(inputs)
		if !reflect.DeepEqual(cfg.IgnoredUsers, want) {
			t.Errorf("ignored-users=%q => %v, want %v", in, cfg.IgnoredUsers, want)
		}
	}
}

func TestPrivateKeyBase64Decode(t *testing.T) {
	cfg, err := Load(validInputs())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHubPrivateKey != "helloworld" {
		t.Errorf("GitHubPrivateKey = %q, want helloworld", cfg.GitHubPrivateKey)
	}
}

func TestGoogleCredentialsDecode(t *testing.T) {
	inputs := validInputs()
	inputs["google-credentials"] = base64.StdEncoding.EncodeToString([]byte(`{"client_email":"hello","private_key":"world"}`))
	cfg, err := Load(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleCredentials.ClientEmail != "hello" || cfg.GoogleCredentials.PrivateKey != "world" {
		t.Errorf("GoogleCredentials = %+v", cfg.GoogleCredentials)
	}
}

func TestLoadRejectsBadAppID(t *testing.T) {
	inputs := validInputs()
	inputs["github-app-id"] = "notanumber"
	if _, err := Load(inputs); err == nil {
		t.Error("expected error for non-numeric github-app-id")
	}
}

func TestLoadRejectsBadBase64(t *testing.T) {
	inputs := validInputs()
	inputs["github-private-key"] = "!!!notbase64!!!"
	if _, err := Load(inputs); err == nil {
		t.Error("expected error for invalid base64 github-private-key")
	}
}
