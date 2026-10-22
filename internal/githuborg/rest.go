package githuborg

import (
	"context"
	"log/slog"

	"github.com/google/go-github/v88/github"
)

// restAPI adapts the go-github client to the api interface, handling pagination.
type restAPI struct {
	client *github.Client
}

const perPage = 100

func (r restAPI) ListOrgMembers(ctx context.Context, org string) ([]string, error) {
	opts := &github.ListMembersOptions{ListOptions: github.ListOptions{PerPage: perPage}}
	var logins []string
	for {
		members, resp, err := r.client.Organizations.ListMembers(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			logins = append(logins, m.GetLogin())
		}
		if resp.NextPage == 0 {
			return logins, nil
		}
		opts.Page = resp.NextPage
	}
}

func (r restAPI) ListPendingInvites(ctx context.Context, org string) ([]string, error) {
	opts := &github.ListOptions{PerPage: perPage}
	var logins []string
	for {
		invites, resp, err := r.client.Organizations.ListPendingOrgInvitations(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		for _, inv := range invites {
			logins = append(logins, inv.GetLogin())
		}
		if resp.NextPage == 0 {
			return logins, nil
		}
		opts.Page = resp.NextPage
	}
}

func (r restAPI) GetUserID(ctx context.Context, username string) (int64, error) {
	slog.Debug("looking up user", "user", username)
	user, _, err := r.client.Users.Get(ctx, username)
	if err != nil {
		return 0, err
	}
	slog.Debug("resolved user id", "user", username, "userID", user.GetID())
	return user.GetID(), nil
}

func (r restAPI) CreateInvitation(ctx context.Context, org string, inviteeID int64) error {
	_, _, err := r.client.Organizations.CreateOrgInvitation(ctx, org, &github.CreateOrgInvitationOptions{
		InviteeID: new(inviteeID),
	})
	return err
}

func (r restAPI) RemoveMembership(ctx context.Context, org, username string) error {
	_, err := r.client.Organizations.RemoveOrgMembership(ctx, username, org)
	return err
}

func (r restAPI) ListTeamMembers(ctx context.Context, org, slug string) ([]string, error) {
	opts := &github.TeamListTeamMembersOptions{ListOptions: github.ListOptions{PerPage: perPage}}
	var logins []string
	for {
		members, resp, err := r.client.Teams.ListTeamMembersBySlug(ctx, org, slug, opts)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			logins = append(logins, m.GetLogin())
		}
		if resp.NextPage == 0 {
			return logins, nil
		}
		opts.Page = resp.NextPage
	}
}

func (r restAPI) AddTeamMembership(ctx context.Context, org, slug, username string) error {
	_, _, err := r.client.Teams.AddTeamMembershipBySlug(ctx, org, slug, username, nil)
	return err
}
