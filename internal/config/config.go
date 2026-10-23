// Package config loads the manager configuration from GitHub Action inputs
// (exposed as INPUT_* environment variables) via go-githubactions.
package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// InputSource resolves a GitHub Action input by its kebab-case name. It is
// satisfied by *github.com/sethvargo/go-githubactions.Action.
type InputSource interface {
	GetInput(name string) string
}

// GoogleCredentials mirrors the service-account JSON downloaded from GCP.
type GoogleCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// Config is the fully-resolved runtime configuration.
type Config struct {
	AddUsers             bool
	RemoveUsers          bool
	RemoveSuspendedUsers bool
	ExitCodeOnMismatch   int
	MaxRemoveUsers       int
	Debug                bool

	GitHubOrg            string
	GitHubAppID          int64
	GitHubInstallationID int64
	GitHubPrivateKey     string
	GitHubTeamSlug       string
	SyncTeam             bool
	IgnoredUsers         []string

	GoogleEmailAddress string
	GoogleCredentials  GoogleCredentials
	// GoogleCredentialsJSON retains the decoded credentials bytes for callers
	// that want to hand them straight to the Google client libraries.
	GoogleCredentialsJSON []byte
}

// Load reads and validates the configuration from the action inputs.
func Load(src InputSource) (*Config, error) {
	c := &Config{
		AddUsers:             boolVal(src.GetInput("add-users")),
		RemoveUsers:          boolVal(src.GetInput("remove-users")),
		RemoveSuspendedUsers: boolVal(src.GetInput("remove-suspended_users")),
		ExitCodeOnMismatch:   intOr(src.GetInput("exit-code-on-mismatch"), 0),
		MaxRemoveUsers:       intOr(src.GetInput("max-remove-users"), 100),
		Debug:                boolVal(src.GetInput("debug")),
		GitHubOrg:            src.GetInput("github-org"),
		GitHubTeamSlug:       src.GetInput("team-slug"),
		SyncTeam:             boolVal(src.GetInput("sync-team")),
		IgnoredUsers:         ignoredUsers(src.GetInput("ignored-users")),
		GoogleEmailAddress:   src.GetInput("google-email-address"),
	}

	appID, err := strconv.ParseInt(src.GetInput("github-app-id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github-app-id must be a number: %w", err)
	}
	c.GitHubAppID = appID

	installationID, err := strconv.ParseInt(src.GetInput("github-installation-id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github-installation-id must be a number: %w", err)
	}
	c.GitHubInstallationID = installationID

	key, err := decodeBase64("github-private-key", src.GetInput("github-private-key"))
	if err != nil {
		return nil, err
	}
	c.GitHubPrivateKey = string(key)

	creds, err := decodeBase64("google-credentials", src.GetInput("google-credentials"))
	if err != nil {
		return nil, err
	}
	c.GoogleCredentialsJSON = creds
	if err := json.Unmarshal(creds, &c.GoogleCredentials); err != nil {
		return nil, fmt.Errorf("google-credentials is not valid JSON: %w", err)
	}

	return c, nil
}

func boolVal(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "true")
}

// intOr matches the Node `parseInt(...) || fallback` behaviour: an unset,
// invalid, or zero value falls back to the default.
func intOr(s string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || v == 0 {
		return fallback
	}
	return v
}

func ignoredUsers(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(strings.ToLower(raw), ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func decodeBase64(name, raw string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%s must be base64 encoded: %w", name, err)
	}
	return decoded, nil
}
