package githuborg

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

func init() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) }

// fakeAPI records calls and returns canned data.
type fakeAPI struct {
	orgMembers   []string
	pending      []string
	teamMembers  []string
	userIDs      map[string]int64
	invited      []int64
	removed      []string
	teamAdded    []string
	addTeamErr   map[string]error
	getUserIDErr error
}

func (f *fakeAPI) ListOrgMembers(context.Context, string) ([]string, error) { return f.orgMembers, nil }
func (f *fakeAPI) ListPendingInvites(context.Context, string) ([]string, error) {
	return f.pending, nil
}
func (f *fakeAPI) GetUserID(_ context.Context, u string) (int64, error) {
	if f.getUserIDErr != nil {
		return 0, f.getUserIDErr
	}
	return f.userIDs[u], nil
}
func (f *fakeAPI) CreateInvitation(_ context.Context, _ string, id int64) error {
	f.invited = append(f.invited, id)
	return nil
}
func (f *fakeAPI) RemoveMembership(_ context.Context, _, u string) error {
	f.removed = append(f.removed, u)
	return nil
}
func (f *fakeAPI) ListTeamMembers(context.Context, string, string) ([]string, error) {
	return f.teamMembers, nil
}
func (f *fakeAPI) AddTeamMembership(_ context.Context, _, _, u string) error {
	if err := f.addTeamErr[u]; err != nil {
		return err
	}
	f.teamAdded = append(f.teamAdded, u)
	return nil
}

func svc(api *fakeAPI, cfg *config.Config) *Service {
	if cfg == nil {
		cfg = &config.Config{GitHubOrg: "myorg", GitHubTeamSlug: "remarkable"}
	}
	return &Service{cfg: cfg, api: api}
}

func TestUsersFromGitHubUnionsMembersAndInvites(t *testing.T) {
	s := svc(&fakeAPI{
		orgMembers: []string{"ChrisNS", "bar", "foo"},
		pending:    []string{"pending", "chrisns"},
	}, nil)
	got, err := s.UsersFromGitHub(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := set.New("chrisns", "bar", "foo", "pending")
	if got.Len() != want.Len() {
		t.Fatalf("got %v want %v", got.Sorted(), want.Sorted())
	}
	for _, u := range want.Sorted() {
		if !got.Has(u) {
			t.Errorf("missing %s", u)
		}
	}
}

func TestAddUsersSkipsIgnored(t *testing.T) {
	api := &fakeAPI{userIDs: map[string]int64{"foo": 42}}
	s := svc(api, &config.Config{GitHubOrg: "myorg", IgnoredUsers: []string{"bar"}})
	if err := s.AddUsersToOrg(context.Background(), set.New("foo", "bar")); err != nil {
		t.Fatal(err)
	}
	if len(api.invited) != 1 || api.invited[0] != 42 {
		t.Errorf("invited = %v, want [42]", api.invited)
	}
}

func TestAddUsersPropagatesLookupError(t *testing.T) {
	api := &fakeAPI{getUserIDErr: errors.New("not found")}
	s := svc(api, &config.Config{GitHubOrg: "myorg"})
	if err := s.AddUsersToOrg(context.Background(), set.New("foo")); err == nil {
		t.Error("expected error when user lookup fails")
	}
}

func TestRemoveUsersSkipsIgnored(t *testing.T) {
	api := &fakeAPI{}
	s := svc(api, &config.Config{GitHubOrg: "myorg", IgnoredUsers: []string{"foo"}})
	if err := s.RemoveUsersFromOrg(context.Background(), set.New("foo", "bar")); err != nil {
		t.Fatal(err)
	}
	if len(api.removed) != 1 || api.removed[0] != "bar" {
		t.Errorf("removed = %v, want [bar]", api.removed)
	}
}

func TestSyncTeamAddsMissing(t *testing.T) {
	api := &fakeAPI{
		orgMembers:  []string{"chrisns", "foo", "bar"},
		teamMembers: []string{"chrisns"},
	}
	s := svc(api, nil)
	if err := s.SyncTeamMembers(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := set.New("foo", "bar")
	if len(api.teamAdded) != 2 {
		t.Fatalf("teamAdded = %v, want foo,bar", api.teamAdded)
	}
	for _, u := range api.teamAdded {
		if !want.Has(u) {
			t.Errorf("unexpected team add %s", u)
		}
	}
}

func TestSyncTeamNoopWhenComplete(t *testing.T) {
	api := &fakeAPI{orgMembers: []string{"chrisns"}, teamMembers: []string{"chrisns"}}
	s := svc(api, nil)
	if err := s.SyncTeamMembers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.teamAdded) != 0 {
		t.Errorf("teamAdded = %v, want none", api.teamAdded)
	}
}

func TestSyncTeamContinuesPastErrors(t *testing.T) {
	api := &fakeAPI{
		orgMembers:  []string{"foo", "bar"},
		teamMembers: []string{},
		addTeamErr:  map[string]error{"bar": errors.New("Validation Failed: no_saml_sso")},
	}
	s := svc(api, nil)
	if err := s.SyncTeamMembers(context.Background()); err != nil {
		t.Fatalf("sync should not abort on per-user error: %v", err)
	}
	if len(api.teamAdded) != 1 || api.teamAdded[0] != "foo" {
		t.Errorf("teamAdded = %v, want [foo]", api.teamAdded)
	}
}
