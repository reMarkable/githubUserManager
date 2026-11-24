// Package config loads the manager configuration from GitHub Action inputs
// (exposed as INPUT_* environment variables) via go-githubactions.
package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
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
	DryRun               bool
	ExitCodeOnMismatch   int
	MaxRemoveUsers       int
	Debug                bool

	GitHubOrg            string
	GitHubAppID          int64
	GitHubInstallationID int64
	GitHubPrivateKey     string
	GitHubTeamSlug       string
	SyncTeam             bool
	// TeamMap maps a Google Workspace group email (lower-cased) to the GitHub
	// team slug whose membership should mirror that group. Empty when unset.
	// Values starting with "ent:" (GitHub's enterprise team slug prefix) refer
	// to enterprise teams; see ParseTeamRef.
	TeamMap      map[string]string
	IgnoredUsers []string

	// GitHubEnterprise is the enterprise slug owning any enterprise teams in
	// TeamMap. GitHubEnterpriseInstallationID is the installation of the same
	// GitHub App on that enterprise, which is distinct from the org
	// installation. Both are required only when TeamMap references an
	// enterprise team.
	GitHubEnterprise               string
	GitHubEnterpriseInstallationID int64

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
		DryRun:               boolVal(src.GetInput("dry-run")),
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

	teamMap, err := loadTeamMap(src.GetInput("team-map-file"), src.GetInput("team-map"))
	if err != nil {
		return nil, err
	}
	c.TeamMap = teamMap

	c.GitHubEnterprise = strings.TrimSpace(src.GetInput("github-enterprise"))
	if raw := strings.TrimSpace(src.GetInput("github-enterprise-installation-id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("github-enterprise-installation-id must be a number: %w", err)
		}
		c.GitHubEnterpriseInstallationID = id
	}
	if c.HasEnterpriseTeams() {
		if c.GitHubEnterprise == "" {
			return nil, fmt.Errorf("team map references enterprise teams but github-enterprise is not set")
		}
		if c.GitHubEnterpriseInstallationID == 0 {
			return nil, fmt.Errorf("team map references enterprise teams but github-enterprise-installation-id is not set")
		}
	}

	return c, nil
}

// EnterpriseTeamPrefix is the prefix GitHub gives every enterprise team slug.
// Team map values starting with it (e.g. "ent:platform") are enterprise teams;
// org team slugs can't contain a colon, so the two never collide.
const EnterpriseTeamPrefix = "ent:"

// TeamRef identifies a team targeted by the team map.
type TeamRef struct {
	// Enterprise is true for enterprise teams, false for org teams.
	Enterprise bool
	// Slug is the team slug as GitHub knows it, including the "ent:" prefix
	// for enterprise teams.
	Slug string
}

// ParseTeamRef classifies a team map value. Values starting with "ent:" are
// enterprise team slugs; anything else is an org team slug.
func ParseTeamRef(value string) TeamRef {
	return TeamRef{Enterprise: strings.HasPrefix(value, EnterpriseTeamPrefix), Slug: value}
}

// HasEnterpriseTeams reports whether any team map entry targets an enterprise team.
func (c *Config) HasEnterpriseTeams() bool {
	for _, team := range c.TeamMap {
		if ParseTeamRef(team).Enterprise {
			return true
		}
	}
	return false
}

// loadTeamMap resolves the group-to-team mapping from either a JSON file
// (team-map-file, preferred for maintainability) or inline JSON (team-map).
// The file takes precedence when both are set. Keys (group emails) are
// lower-cased and trimmed; empty entries are dropped.
func loadTeamMap(path, inline string) (map[string]string, error) {
	raw := strings.TrimSpace(inline)
	if p := strings.TrimSpace(path); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading team-map-file %q: %w", p, err)
		}
		raw = strings.TrimSpace(string(b))
	}
	if raw == "" {
		return nil, nil
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("team map is not valid JSON (expected {\"group@domain\":\"team-slug\"} or {\"group@domain\":\"ent:enterprise-team-slug\"}): %w", err)
	}

	out := make(map[string]string, len(parsed))
	for group, slug := range parsed {
		g := strings.TrimSpace(strings.ToLower(group))
		s := strings.TrimSpace(slug)
		if g == "" || s == "" || s == EnterpriseTeamPrefix {
			continue
		}
		out[g] = s
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
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
