package config

import (
	"encoding/base64"
	"os"
	"reflect"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ADD_USERS", "REMOVE_USERS", "REMOVE_SUSPENDED_USERS", "EXIT_CODE_ON_MISMATCH",
		"MAX_REMOVE_USERS", "GITHUB_ORG", "GITHUB_APP_ID", "GITHUB_INSTALLATION_ID",
		"GITHUB_PRIVATE_KEY", "GITHUB_TEAM_SLUG", "SYNC_TEAM", "IGNORED_USERS",
		"GOOGLE_EMAIL_ADDRESS", "GOOGLE_CREDENTIALS",
	} {
		_ = os.Unsetenv(k)
	}
}

// validEnv sets the minimum required variables for Load to succeed.
func validEnv(t *testing.T) {
	t.Helper()
	_ = os.Setenv("GITHUB_APP_ID", "123")
	_ = os.Setenv("GITHUB_INSTALLATION_ID", "456")
	_ = os.Setenv("GITHUB_PRIVATE_KEY", base64.StdEncoding.EncodeToString([]byte("helloworld")))
	_ = os.Setenv("GOOGLE_CREDENTIALS", base64.StdEncoding.EncodeToString([]byte(`{"client_email":"foo","private_key":"bar"}`)))
}

func TestBoolEnv(t *testing.T) {
	cases := map[string]bool{"": false, "foobar": false, "false": false, "true": true, "tRuE": true}
	for in, want := range cases {
		clearEnv(t)
		validEnv(t)
		if in != "" {
			_ = os.Setenv("ADD_USERS", in)
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.AddUsers != want {
			t.Errorf("ADD_USERS=%q => %v, want %v", in, cfg.AddUsers, want)
		}
	}
}

func TestExitCodeOnMismatch(t *testing.T) {
	cases := map[string]int{"": 0, "hello": 0, "123": 123}
	for in, want := range cases {
		clearEnv(t)
		validEnv(t)
		if in != "" {
			_ = os.Setenv("EXIT_CODE_ON_MISMATCH", in)
		}
		cfg, _ := Load()
		if cfg.ExitCodeOnMismatch != want {
			t.Errorf("EXIT_CODE_ON_MISMATCH=%q => %d, want %d", in, cfg.ExitCodeOnMismatch, want)
		}
	}
}

func TestMaxRemoveUsersDefault(t *testing.T) {
	clearEnv(t)
	validEnv(t)
	cfg, _ := Load()
	if cfg.MaxRemoveUsers != 100 {
		t.Errorf("default MaxRemoveUsers = %d, want 100", cfg.MaxRemoveUsers)
	}
	_ = os.Setenv("MAX_REMOVE_USERS", "5")
	cfg, _ = Load()
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
		clearEnv(t)
		validEnv(t)
		if in != "" {
			_ = os.Setenv("IGNORED_USERS", in)
		}
		cfg, _ := Load()
		if !reflect.DeepEqual(cfg.IgnoredUsers, want) {
			t.Errorf("IGNORED_USERS=%q => %v, want %v", in, cfg.IgnoredUsers, want)
		}
	}
}

func TestPrivateKeyBase64Decode(t *testing.T) {
	clearEnv(t)
	validEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHubPrivateKey != "helloworld" {
		t.Errorf("GitHubPrivateKey = %q, want helloworld", cfg.GitHubPrivateKey)
	}
}

func TestGoogleCredentialsDecode(t *testing.T) {
	clearEnv(t)
	validEnv(t)
	_ = os.Setenv("GOOGLE_CREDENTIALS", base64.StdEncoding.EncodeToString([]byte(`{"client_email":"hello","private_key":"world"}`)))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleCredentials.ClientEmail != "hello" || cfg.GoogleCredentials.PrivateKey != "world" {
		t.Errorf("GoogleCredentials = %+v", cfg.GoogleCredentials)
	}
}

func TestLoadRejectsBadAppID(t *testing.T) {
	clearEnv(t)
	validEnv(t)
	_ = os.Setenv("GITHUB_APP_ID", "notanumber")
	if _, err := Load(); err == nil {
		t.Error("expected error for non-numeric GITHUB_APP_ID")
	}
}

func TestLoadRejectsBadBase64(t *testing.T) {
	clearEnv(t)
	validEnv(t)
	_ = os.Setenv("GITHUB_PRIVATE_KEY", "!!!notbase64!!!")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid base64 GITHUB_PRIVATE_KEY")
	}
}
