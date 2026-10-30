// Package githuborg manages GitHub organisation (and optional team) membership
// using a GitHub App installation for authentication.
package githuborg

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/google/go-github/v88/github"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

// api is the subset of the GitHub REST API the manager depends on, extracted so
// the membership logic can be unit tested without the network.
type api interface {
	ListOrgMembers(ctx context.Context, org string) ([]string, error)
	ListPendingInvites(ctx context.Context, org string) ([]string, error)
	GetUserID(ctx context.Context, username string) (int64, error)
	CreateInvitation(ctx context.Context, org string, inviteeID int64) error
	RemoveMembership(ctx context.Context, org, username string) error
	ListTeamMembers(ctx context.Context, org, slug string) ([]string, error)
	AddTeamMembership(ctx context.Context, org, slug, username string) error
	RemoveTeamMembership(ctx context.Context, org, slug, username string) error
}

// Service applies membership changes against a GitHub organisation.
type Service struct {
	cfg *config.Config
	api api
}

// New builds a Service authenticated as the configured GitHub App installation.
func New(cfg *config.Config) (*Service, error) {
	transport, err := newAppTransport(
		http.DefaultTransport,
		cfg.GitHubAppID,
		cfg.GitHubInstallationID,
		[]byte(cfg.GitHubPrivateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("creating github app transport: %w", err)
	}
	client, err := github.NewClient(github.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}
	return &Service{cfg: cfg, api: restAPI{client: client}}, nil
}

func (s *Service) isIgnored(user string) bool {
	for _, ignored := range s.cfg.IgnoredUsers {
		if ignored == strings.ToLower(user) {
			return true
		}
	}
	return false
}

// UsersFromGitHub returns the union of current org members and pending invitees.
func (s *Service) UsersFromGitHub(ctx context.Context) (set.Set, error) {
	members, err := s.api.ListOrgMembers(ctx, s.cfg.GitHubOrg)
	if err != nil {
		return nil, fmt.Errorf("listing org members: %w", err)
	}
	pending, err := s.api.ListPendingInvites(ctx, s.cfg.GitHubOrg)
	if err != nil {
		return nil, fmt.Errorf("listing pending invitations: %w", err)
	}

	pendingSet := formatUserList(pending)
	if pendingSet.Len() > 0 {
		slog.Info("outstanding github invites", "org", strings.Join(pendingSet.Sorted(), ", "))
	}
	return formatUserList(members).Union(pendingSet), nil
}

// AddUsersToOrg invites each user to the organisation.
func (s *Service) AddUsersToOrg(ctx context.Context, users set.Set) error {
	for _, user := range users.Sorted() {
		if s.isIgnored(user) {
			slog.Info("ignoring add", "user", user)
			continue
		}
		if s.cfg.DryRun {
			slog.Info("dry run: skipping org invite", "user", user, "org", s.cfg.GitHubOrg)
			continue
		}
		userID, err := s.api.GetUserID(ctx, user)
		if err != nil {
			return fmt.Errorf("unable to find user id for %s: %w", user, err)
		}
		slog.Info("inviting user", "user", user, "userID", userID, "org", s.cfg.GitHubOrg)
		if err := s.api.CreateInvitation(ctx, s.cfg.GitHubOrg, userID); err != nil {
			return fmt.Errorf("inviting %s: %w", user, err)
		}
	}
	return nil
}

// RemoveUsersFromOrg removes each user (or pending invitation) from the org.
func (s *Service) RemoveUsersFromOrg(ctx context.Context, users set.Set) error {
	for _, user := range users.Sorted() {
		if s.isIgnored(user) {
			slog.Info("ignoring remove", "user", user)
			continue
		}
		slog.Info("removing user/invitation", "user", user, "org", s.cfg.GitHubOrg)
		if s.cfg.DryRun {
			slog.Info("dry run: skipping org removal", "user", user, "org", s.cfg.GitHubOrg)
			continue
		}
		if err := s.api.RemoveMembership(ctx, s.cfg.GitHubOrg, user); err != nil {
			return fmt.Errorf("removing %s: %w", user, err)
		}
	}
	return nil
}

// SyncTeamMembers ensures every org member is also a member of the configured team.
func (s *Service) SyncTeamMembers(ctx context.Context) error {
	members, err := s.api.ListOrgMembers(ctx, s.cfg.GitHubOrg)
	if err != nil {
		return fmt.Errorf("listing org members: %w", err)
	}
	orgMembers := formatUserList(members)

	teamMembersRaw, err := s.api.ListTeamMembers(ctx, s.cfg.GitHubOrg, s.cfg.GitHubTeamSlug)
	if err != nil {
		return fmt.Errorf("listing team members: %w", err)
	}
	teamMembers := formatUserList(teamMembersRaw)

	missing := orgMembers.Diff(teamMembers)
	if missing.Len() == 0 {
		slog.Info("all org members already in team", "team", s.cfg.GitHubTeamSlug)
		return nil
	}

	slog.Info("users not in team", "team", s.cfg.GitHubTeamSlug, "users", strings.Join(missing.Sorted(), ", "))
	for _, user := range missing.Sorted() {
		if s.isIgnored(user) {
			slog.Info("ignoring team add", "user", user)
			continue
		}
		slog.Info("adding user to team", "user", user, "team", s.cfg.GitHubTeamSlug)
		if s.cfg.DryRun {
			slog.Info("dry run: skipping team add", "user", user, "team", s.cfg.GitHubTeamSlug)
			continue
		}
		// Mirror the Node behaviour: log and continue past per-user failures
		// (e.g. SAML SSO validation errors) rather than aborting the sync.
		if err := s.api.AddTeamMembership(ctx, s.cfg.GitHubOrg, s.cfg.GitHubTeamSlug, user); err != nil {
			slog.Warn("skipping team add", "user", user, "err", err)
		}
	}
	return nil
}

// SyncTeamMap reconciles each GitHub team against the desired set of members
// resolved from its mapped Google Workspace group. Missing members are always
// added; members present in the team but not in the group are only removed when
// remove-users is enabled, keeping the dry-run behaviour consistent with org
// membership reconciliation. It returns true when any team differed from its
// group (regardless of whether changes were applied) so callers can surface a
// mismatch exit code.
func (s *Service) SyncTeamMap(ctx context.Context, desired map[string]set.Set) (bool, error) {
	slugs := make([]string, 0, len(desired))
	for slug := range desired {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	drift := false
	for _, slug := range slugs {
		want := desired[slug]
		currentRaw, err := s.api.ListTeamMembers(ctx, s.cfg.GitHubOrg, slug)
		if err != nil {
			return drift, fmt.Errorf("listing members of team %s: %w", slug, err)
		}
		current := formatUserList(currentRaw)

		missing := want.Diff(current)
		extra := current.Diff(want)

		if missing.Len() == 0 && extra.Len() == 0 {
			slog.Info("team already in sync with group", "team", slug)
			continue
		}
		drift = true

		for _, user := range missing.Sorted() {
			if s.isIgnored(user) {
				slog.Info("ignoring team add", "user", user, "team", slug)
				continue
			}
			slog.Info("adding user to team", "user", user, "team", slug)
			if s.cfg.DryRun {
				slog.Info("dry run: skipping team add", "user", user, "team", slug)
				continue
			}
			// Mirror SyncTeamMembers: log and continue past per-user failures
			// (e.g. SAML SSO validation errors) rather than aborting.
			if err := s.api.AddTeamMembership(ctx, s.cfg.GitHubOrg, slug, user); err != nil {
				slog.Warn("skipping team add", "user", user, "team", slug, "err", err)
			}
		}

		if extra.Len() == 0 {
			continue
		}
		if !s.cfg.RemoveUsers {
			slog.Info("team members not in group (not removing, remove-users disabled)",
				"team", slug, "users", strings.Join(extra.Sorted(), ", "))
			continue
		}
		// Safety: a group that resolves to no linked GitHub accounts (e.g. a
		// typo'd group email or members who haven't linked GitHub) must not be
		// treated as "remove everyone".
		if want.Len() == 0 {
			slog.Warn("group resolved to zero github users; refusing to empty team",
				"team", slug, "current", strings.Join(extra.Sorted(), ", "))
			continue
		}
		// Safety: mirror the org MaxRemoveUsers guard so a mapping mistake can't
		// mass-remove a whole team in one run.
		if extra.Len() > s.cfg.MaxRemoveUsers {
			slog.Warn("not removing team members because there are too many",
				"team", slug, "count", extra.Len(), "max", s.cfg.MaxRemoveUsers)
			continue
		}
		for _, user := range extra.Sorted() {
			if s.isIgnored(user) {
				slog.Info("ignoring team remove", "user", user, "team", slug)
				continue
			}
			slog.Info("removing user from team", "user", user, "team", slug)
			if s.cfg.DryRun {
				slog.Info("dry run: skipping team remove", "user", user, "team", slug)
				continue
			}
			if err := s.api.RemoveTeamMembership(ctx, s.cfg.GitHubOrg, slug, user); err != nil {
				slog.Warn("skipping team remove", "user", user, "team", slug, "err", err)
			}
		}
	}
	return drift, nil
}

// formatUserList lower-cases and de-duplicates a list of GitHub logins.
func formatUserList(logins []string) set.Set {
	s := set.New()
	for _, login := range logins {
		s.Add(strings.ToLower(login))
	}
	return s
}
