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

// enterpriseAPI is the subset of the enterprise teams REST API used to sync
// enterprise team membership. It must be backed by a GitHub App installation on
// the enterprise (not the org), with the "Enterprise teams" permission.
type enterpriseAPI interface {
	ListEnterpriseTeamMembers(ctx context.Context, enterprise, team string) ([]string, error)
	AddEnterpriseTeamMember(ctx context.Context, enterprise, team, username string) error
	RemoveEnterpriseTeamMember(ctx context.Context, enterprise, team, username string) error
}

// Service applies membership changes against a GitHub organisation.
type Service struct {
	cfg *config.Config
	api api
	// ent is nil unless the team map references enterprise teams.
	ent enterpriseAPI
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
	s := &Service{cfg: cfg, api: restAPI{client: client}}

	if cfg.HasEnterpriseTeams() {
		// Enterprise team endpoints need a token for the app's enterprise
		// installation; the org installation token is rejected.
		entTransport, err := newAppTransport(
			http.DefaultTransport,
			cfg.GitHubAppID,
			cfg.GitHubEnterpriseInstallationID,
			[]byte(cfg.GitHubPrivateKey),
		)
		if err != nil {
			return nil, fmt.Errorf("creating github enterprise app transport: %w", err)
		}
		entClient, err := github.NewClient(github.WithHTTPClient(&http.Client{Transport: entTransport}))
		if err != nil {
			return nil, fmt.Errorf("creating github enterprise client: %w", err)
		}
		s.ent = restAPI{client: entClient}
	}
	return s, nil
}

// teamOps abstracts list/add/remove for a single team so SyncTeamMap can treat
// org teams and enterprise teams uniformly.
type teamOps struct {
	list   func(ctx context.Context) ([]string, error)
	add    func(ctx context.Context, username string) error
	remove func(ctx context.Context, username string) error
}

// teamOpsFor resolves a team map value (see config.ParseTeamRef) to the API
// calls that manage that team.
func (s *Service) teamOpsFor(team string) (teamOps, error) {
	ref := config.ParseTeamRef(team)
	if !ref.Enterprise {
		org := s.cfg.GitHubOrg
		return teamOps{
			list:   func(ctx context.Context) ([]string, error) { return s.api.ListTeamMembers(ctx, org, ref.Slug) },
			add:    func(ctx context.Context, u string) error { return s.api.AddTeamMembership(ctx, org, ref.Slug, u) },
			remove: func(ctx context.Context, u string) error { return s.api.RemoveTeamMembership(ctx, org, ref.Slug, u) },
		}, nil
	}
	if s.ent == nil || s.cfg.GitHubEnterprise == "" {
		return teamOps{}, fmt.Errorf("team %s is an enterprise team but github-enterprise / github-enterprise-installation-id are not configured", team)
	}
	ent := s.cfg.GitHubEnterprise
	return teamOps{
		list: func(ctx context.Context) ([]string, error) {
			return s.ent.ListEnterpriseTeamMembers(ctx, ent, ref.Slug)
		},
		add: func(ctx context.Context, u string) error { return s.ent.AddEnterpriseTeamMember(ctx, ent, ref.Slug, u) },
		remove: func(ctx context.Context, u string) error {
			return s.ent.RemoveEnterpriseTeamMember(ctx, ent, ref.Slug, u)
		},
	}, nil
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
// resolved from its mapped Google Workspace group. Missing members are added
// (subject to the add-users gate below); members present in the team but not in
// the group are only removed when remove-users is enabled, keeping the dry-run
// behaviour consistent with org membership reconciliation. When add-users is
// disabled, members who aren't already in the org are skipped so a team add
// can't invite people to the org as a side effect. It returns true when any
// team differed from its group (regardless of whether changes were applied) so
// callers can surface a mismatch exit code.
//
// Keys of desired are team map values: an org team slug, or an "ent:"-prefixed
// enterprise team slug in cfg.GitHubEnterprise. Both
// kinds go through the same add/remove gates and safety guards.
func (s *Service) SyncTeamMap(ctx context.Context, desired map[string]set.Set) (bool, error) {
	slugs := make([]string, 0, len(desired))
	for slug := range desired {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	// Adding a non-member to a team via AddTeamMembership creates a pending org
	// invitation as a side effect. When add-users is disabled the operator has
	// opted out of inviting people, so gate team adds for users who aren't yet
	// org members (members or pending invites). We only pay for this lookup when
	// add-users is off, since otherwise those users are invited upstream anyway.
	var orgMembers set.Set
	if !s.cfg.AddUsers {
		current, err := s.UsersFromGitHub(ctx)
		if err != nil {
			return false, fmt.Errorf("listing org members for team sync: %w", err)
		}
		orgMembers = current
	}

	drift := false
	for _, slug := range slugs {
		want := desired[slug]
		ops, err := s.teamOpsFor(slug)
		if err != nil {
			return drift, err
		}
		currentRaw, err := ops.list(ctx)
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
			if orgMembers != nil && !orgMembers.Has(user) {
				slog.Info("not adding user to team: not an org member and add-users disabled",
					"user", user, "team", slug)
				continue
			}
			slog.Info("adding user to team", "user", user, "team", slug)
			if s.cfg.DryRun {
				slog.Info("dry run: skipping team add", "user", user, "team", slug)
				continue
			}
			// Mirror SyncTeamMembers: log and continue past per-user failures
			// (e.g. SAML SSO validation errors) rather than aborting.
			if err := ops.add(ctx, user); err != nil {
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
			if err := ops.remove(ctx, user); err != nil {
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
