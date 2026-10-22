// Package config loads the manager configuration from environment variables.
// The variable names and semantics match the original Node implementation so
// the container contract is unchanged.
package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

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

// Load reads and validates the configuration from the environment.
func Load() (*Config, error) {
	c := &Config{
		AddUsers:             boolEnv("ADD_USERS"),
		RemoveUsers:          boolEnv("REMOVE_USERS"),
		RemoveSuspendedUsers: boolEnv("REMOVE_SUSPENDED_USERS"),
		ExitCodeOnMismatch:   intEnvOr("EXIT_CODE_ON_MISMATCH", 0),
		MaxRemoveUsers:       intEnvOr("MAX_REMOVE_USERS", 100),
		Debug:                boolEnv("DEBUG"),
		GitHubOrg:            os.Getenv("GITHUB_ORG"),
		GitHubTeamSlug:       os.Getenv("GITHUB_TEAM_SLUG"),
		SyncTeam:             boolEnv("SYNC_TEAM"),
		IgnoredUsers:         ignoredUsers(os.Getenv("IGNORED_USERS")),
		GoogleEmailAddress:   os.Getenv("GOOGLE_EMAIL_ADDRESS"),
	}

	appID, err := strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_APP_ID must be a number: %w", err)
	}
	c.GitHubAppID = appID

	installationID, err := strconv.ParseInt(os.Getenv("GITHUB_INSTALLATION_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_INSTALLATION_ID must be a number: %w", err)
	}
	c.GitHubInstallationID = installationID

	key, err := decodeBase64("GITHUB_PRIVATE_KEY")
	if err != nil {
		return nil, err
	}
	c.GitHubPrivateKey = string(key)

	creds, err := decodeBase64("GOOGLE_CREDENTIALS")
	if err != nil {
		return nil, err
	}
	c.GoogleCredentialsJSON = creds
	if err := json.Unmarshal(creds, &c.GoogleCredentials); err != nil {
		return nil, fmt.Errorf("GOOGLE_CREDENTIALS is not valid JSON: %w", err)
	}

	return c, nil
}

func boolEnv(name string) bool {
	return strings.EqualFold(os.Getenv(name), "true")
}

// intEnvOr matches the Node `parseInt(...) || fallback` behaviour: an unset,
// invalid, or zero value falls back to the default.
func intEnvOr(name string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || v == 0 {
		return fallback
	}
	return v
}

func ignoredUsers(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(strings.ToLower(raw), ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func decodeBase64(name string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64 encoded: %w", name, err)
	}
	return decoded, nil
}
