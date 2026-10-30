package config

import (
	"encoding/base64"
	"os"
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

func TestTeamMapInlineJSON(t *testing.T) {
	inputs := validInputs()
	inputs["team-map"] = `{"Eng@remarkable.no":"engineering"," design@remarkable.no ":"design","":"skip","empty-slug@x":""}`
	cfg, err := Load(inputs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"eng@remarkable.no":    "engineering",
		"design@remarkable.no": "design",
	}
	if !reflect.DeepEqual(cfg.TeamMap, want) {
		t.Errorf("TeamMap = %v, want %v", cfg.TeamMap, want)
	}
}

func TestTeamMapFileTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/teams.json"
	if err := os.WriteFile(path, []byte(`{"platform@remarkable.no":"platform"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inputs := validInputs()
	inputs["team-map"] = `{"ignored@x":"ignored"}`
	inputs["team-map-file"] = path
	cfg, err := Load(inputs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"platform@remarkable.no": "platform"}
	if !reflect.DeepEqual(cfg.TeamMap, want) {
		t.Errorf("TeamMap = %v, want %v", cfg.TeamMap, want)
	}
}

func TestTeamMapUnsetIsNil(t *testing.T) {
	cfg, err := Load(validInputs())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TeamMap != nil {
		t.Errorf("TeamMap = %v, want nil", cfg.TeamMap)
	}
}

func TestTeamMapRejectsBadJSON(t *testing.T) {
	inputs := validInputs()
	inputs["team-map"] = `not json`
	if _, err := Load(inputs); err == nil {
		t.Error("expected error for invalid team-map JSON")
	}
}

func TestDryRun(t *testing.T) {
	cfg, _ := Load(validInputs())
	if cfg.DryRun {
		t.Error("DryRun should default to false")
	}
	inputs := validInputs()
	inputs["dry-run"] = "TRUE"
	cfg, _ = Load(inputs)
	if !cfg.DryRun {
		t.Error("DryRun should be true when dry-run=TRUE")
	}
}

func TestTeamMapFileMissing(t *testing.T) {
	inputs := validInputs()
	inputs["team-map-file"] = "/no/such/teams.json"
	if _, err := Load(inputs); err == nil {
		t.Error("expected error when team-map-file does not exist")
	}
}

func TestTeamMapFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/teams.json"
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	inputs := validInputs()
	inputs["team-map-file"] = path
	if _, err := Load(inputs); err == nil {
		t.Error("expected error for invalid JSON in team-map-file")
	}
}
