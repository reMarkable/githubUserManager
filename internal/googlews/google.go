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

const (
	userScope   = admin.AdminDirectoryUserReadonlyScope
	memberScope = admin.AdminDirectoryGroupMemberReadonlyScope
)

// Service fetches GitHub usernames from Google Workspace.
type Service struct {
	cfg     *config.Config
	users   usersListCaller
	members membersListCaller
}

// usersListCaller is the slice of the Admin SDK we depend on, extracted as an
// interface so the pagination logic can be unit tested without the network.
type usersListCaller interface {
	List(ctx context.Context, pageToken string) (*admin.Users, error)
}

// membersListCaller lists the members of a Google Workspace group, extracted as
// an interface so the group resolution logic can be unit tested.
type membersListCaller interface {
	ListMembers(ctx context.Context, groupKey, pageToken string) (*admin.Members, error)
}

// New builds a Service backed by the real Admin Directory API using JWT
// domain-wide delegation (impersonating the configured admin email address).
func New(ctx context.Context, cfg *config.Config) (*Service, error) {
	jwtCfg, err := google.JWTConfigFromJSON(cfg.GoogleCredentialsJSON, userScope, memberScope)
	if err != nil {
		return nil, fmt.Errorf("parsing google credentials: %w", err)
	}
	jwtCfg.Subject = cfg.GoogleEmailAddress

	svc, err := admin.NewService(ctx, option.WithHTTPClient(jwtCfg.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("creating admin directory service: %w", err)
	}

	caller := apiCaller{svc: svc, cfg: cfg}
	return &Service{cfg: cfg, users: caller, members: caller}, nil
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

// GithubUsersForGroups resolves each Google Workspace group (by email) to the
// set of GitHub usernames linked to its members. Membership is expanded
// recursively so nested groups are included. Members without a linked GitHub
// account are simply skipped.
func (s *Service) GithubUsersForGroups(ctx context.Context, groups []string) (map[string]set.Set, error) {
	emailToGithub, err := s.emailToGithub(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]set.Set, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(strings.ToLower(group))
		if group == "" {
			continue
		}
		emails, err := s.groupMemberEmails(ctx, group, map[string]bool{})
		if err != nil {
			return nil, fmt.Errorf("listing members of group %s: %w", group, err)
		}
		accounts := set.New()
		for _, email := range emails {
			for _, gh := range emailToGithub[email].Sorted() {
				accounts.Add(gh)
			}
		}
		out[group] = accounts
	}
	return out, nil
}

// emailToGithub builds a map of lower-cased primary email to the set of GitHub
// usernames linked on that Workspace user, paging through the directory.
func (s *Service) emailToGithub(ctx context.Context) (map[string]set.Set, error) {
	out := map[string]set.Set{}
	pageToken := ""
	for {
		resp, err := s.users.List(ctx, pageToken)
		if err != nil {
			return nil, fmt.Errorf("listing google users: %w", err)
		}
		for _, u := range resp.Users {
			if u == nil || u.PrimaryEmail == "" {
				continue
			}
			linked := githubValues(u)
			if len(linked) == 0 {
				continue
			}
			email := strings.TrimSpace(strings.ToLower(u.PrimaryEmail))
			accounts, ok := out[email]
			if !ok {
				accounts = set.New()
				out[email] = accounts
			}
			for _, v := range linked {
				accounts.Add(v)
			}
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return out, nil
}

// groupMemberEmails returns the lower-cased emails of all USER members of the
// group, recursing into nested GROUP members while guarding against cycles.
func (s *Service) groupMemberEmails(ctx context.Context, group string, visited map[string]bool) ([]string, error) {
	if visited[group] {
		return nil, nil
	}
	visited[group] = true

	var emails []string
	pageToken := ""
	for {
		resp, err := s.members.ListMembers(ctx, group, pageToken)
		if err != nil {
			return nil, err
		}
		for _, m := range resp.Members {
			if m == nil || m.Email == "" {
				continue
			}
			if strings.EqualFold(m.Type, "GROUP") {
				nested, err := s.groupMemberEmails(ctx, strings.ToLower(m.Email), visited)
				if err != nil {
					return nil, err
				}
				emails = append(emails, nested...)
				continue
			}
			emails = append(emails, strings.TrimSpace(strings.ToLower(m.Email)))
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return emails, nil
}

// githubValues extracts the lower-cased, trimmed github account values from a
// single user's Accounts custom schema. CustomSchemas values arrive as raw JSON.
func githubValues(u *admin.User) []string {
	if u == nil {
		return nil
	}
	raw, ok := u.CustomSchemas["Accounts"]
	if !ok {
		return nil
	}
	var parsed struct {
		Github []struct {
			Value string `json:"value"`
		} `json:"github"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	var out []string
	for _, acc := range parsed.Github {
		v := strings.TrimSpace(strings.ToLower(acc.Value))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// formatUserList extracts the lower-cased, trimmed github account values from
// the users' Accounts custom schema. CustomSchemas values arrive as raw JSON.
func formatUserList(users []*admin.User) []string {
	var out []string
	for _, u := range users {
		out = append(out, githubValues(u)...)
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
		Fields("users(primaryEmail,customSchemas/Accounts/github(value)),nextPageToken").
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

func (a apiCaller) ListMembers(ctx context.Context, groupKey, pageToken string) (*admin.Members, error) {
	call := a.svc.Members.List(groupKey).
		MaxResults(200).
		Context(ctx)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}
