// Package app wires the Google and GitHub providers together and implements the
// reconciliation flow, mirroring the original index.ts run() logic.
package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

// GoogleProvider supplies the desired set of GitHub users from Google Workspace.
type GoogleProvider interface {
	GithubUsersFromGoogle(ctx context.Context) (set.Set, error)
	GithubUsersForGroups(ctx context.Context, groups []string) (map[string]set.Set, error)
}

// GitHubProvider reads and reconciles GitHub organisation membership.
type GitHubProvider interface {
	UsersFromGitHub(ctx context.Context) (set.Set, error)
	AddUsersToOrg(ctx context.Context, users set.Set) error
	RemoveUsersFromOrg(ctx context.Context, users set.Set) error
	SyncTeamMembers(ctx context.Context) error
	SyncTeamMap(ctx context.Context, desired map[string]set.Set) (bool, error)
}

// Run reconciles GitHub org membership against Google Workspace and returns the
// process exit code (ExitCodeOnMismatch when the two sides differ, else 0).
func Run(ctx context.Context, cfg *config.Config, google GoogleProvider, gh GitHubProvider) (int, error) {
	if cfg.DryRun {
		slog.Info("dry run enabled: no changes will be applied")
	}
	googleUsers, err := google.GithubUsersFromGoogle(ctx)
	if err != nil {
		return 0, err
	}
	slog.Info("users from google", "users", strings.Join(googleUsers.Sorted(), ", "))

	gitHubUsers, err := gh.UsersFromGitHub(ctx)
	if err != nil {
		return 0, err
	}
	slog.Info("users from github", "users", strings.Join(gitHubUsers.Sorted(), ", "))

	usersNotInGitHub := googleUsers.Diff(gitHubUsers)
	usersNotInGoogle := gitHubUsers.Diff(googleUsers)

	if usersNotInGoogle.Len() > 0 {
		slog.Info("users not in google", "users", strings.Join(usersNotInGoogle.Sorted(), ", "))
		if cfg.RemoveUsers {
			// Preserve original behaviour: guard on the number of users we'd
			// be adding, not removing.
			if usersNotInGitHub.Len() <= cfg.MaxRemoveUsers {
				if err := gh.RemoveUsersFromOrg(ctx, usersNotInGoogle); err != nil {
					return 0, err
				}
			} else {
				slog.Warn("not removing users because there are too many")
			}
		}
	}

	if usersNotInGitHub.Len() > 0 {
		slog.Info("users not in github", "users", strings.Join(usersNotInGitHub.Sorted(), ", "))
		if cfg.AddUsers {
			if err := gh.AddUsersToOrg(ctx, usersNotInGitHub); err != nil {
				return 0, err
			}
		}
	}

	if cfg.SyncTeam {
		if err := gh.SyncTeamMembers(ctx); err != nil {
			return 0, err
		}
	}

	teamMapDrift := false
	if len(cfg.TeamMap) > 0 {
		drift, err := syncTeamMap(ctx, cfg, google, gh)
		if err != nil {
			return 0, err
		}
		teamMapDrift = drift
	}

	if usersNotInGoogle.Len() > 0 || usersNotInGitHub.Len() > 0 || teamMapDrift {
		return cfg.ExitCodeOnMismatch, nil
	}
	return 0, nil
}

// syncTeamMap resolves each configured Google Workspace group to its members'
// GitHub usernames and reconciles the mapped GitHub team membership. Multiple
// groups mapping to the same team slug are unioned. It returns true when any
// mapped team differed from its group so callers can flag a mismatch.
func syncTeamMap(ctx context.Context, cfg *config.Config, google GoogleProvider, gh GitHubProvider) (bool, error) {
	groups := make([]string, 0, len(cfg.TeamMap))
	for group := range cfg.TeamMap {
		groups = append(groups, group)
	}

	byGroup, err := google.GithubUsersForGroups(ctx, groups)
	if err != nil {
		return false, err
	}

	desired := make(map[string]set.Set, len(cfg.TeamMap))
	for group, slug := range cfg.TeamMap {
		members := byGroup[group]
		slog.Info("group members resolved to github", "group", group, "team", slug,
			"users", strings.Join(members.Sorted(), ", "))
		team, ok := desired[slug]
		if !ok {
			team = set.New()
			desired[slug] = team
		}
		for _, user := range members.Sorted() {
			team.Add(user)
		}
	}

	return gh.SyncTeamMap(ctx, desired)
}
