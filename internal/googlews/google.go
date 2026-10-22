// Package googlews reads the GitHub usernames stored against Google Workspace
// users via a multi-valued custom schema attribute (Accounts.github).
package googlews

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

const scope = admin.AdminDirectoryUserReadonlyScope

// Service fetches GitHub usernames from Google Workspace.
type Service struct {
	cfg   *config.Config
	users usersListCaller
}

// usersListCaller is the slice of the Admin SDK we depend on, extracted as an
// interface so the pagination logic can be unit tested without the network.
type usersListCaller interface {
	List(ctx context.Context, pageToken string) (*admin.Users, error)
}

// New builds a Service backed by the real Admin Directory API using JWT
// domain-wide delegation (impersonating the configured admin email address).
func New(ctx context.Context, cfg *config.Config) (*Service, error) {
	jwtCfg, err := google.JWTConfigFromJSON(cfg.GoogleCredentialsJSON, scope)
	if err != nil {
		return nil, fmt.Errorf("parsing google credentials: %w", err)
	}
	jwtCfg.Subject = cfg.GoogleEmailAddress

	svc, err := admin.NewService(ctx, option.WithHTTPClient(jwtCfg.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("creating admin directory service: %w", err)
	}

	return &Service{cfg: cfg, users: apiCaller{svc: svc, cfg: cfg}}, nil
}

// GithubUsersFromGoogle returns the set of GitHub usernames linked across all
// Google Workspace users, paging through the directory.
func (s *Service) GithubUsersFromGoogle(ctx context.Context) (set.Set, error) {
	accounts := set.New()
	pageToken := ""
	for {
		resp, err := s.users.List(ctx, pageToken)
		if err != nil {
			return nil, fmt.Errorf("listing google users: %w", err)
		}
		for _, u := range formatUserList(resp.Users) {
			accounts.Add(u)
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return accounts, nil
}

// formatUserList extracts the lower-cased, trimmed github account values from
// the users' Accounts custom schema. CustomSchemas values arrive as raw JSON.
func formatUserList(users []*admin.User) []string {
	var out []string
	for _, u := range users {
		if u == nil {
			continue
		}
		raw, ok := u.CustomSchemas["Accounts"]
		if !ok {
			continue
		}
		var parsed struct {
			Github []struct {
				Value string `json:"value"`
			} `json:"github"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		for _, acc := range parsed.Github {
			v := strings.TrimSpace(strings.ToLower(acc.Value))
			if v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

type apiCaller struct {
	svc *admin.Service
	cfg *config.Config
}

func (a apiCaller) List(ctx context.Context, pageToken string) (*admin.Users, error) {
	call := a.svc.Users.List().
		Customer("my_customer").
		CustomFieldMask("Accounts").
		Projection("custom").
		Fields("users(customSchemas/Accounts/github(value)),nextPageToken").
		MaxResults(250).
		Context(ctx)
	if a.cfg.RemoveSuspendedUsers {
		call = call.Query("isSuspended=false")
	}
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}
