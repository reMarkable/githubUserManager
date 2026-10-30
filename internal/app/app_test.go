package app

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

type fakeGoogle struct{ users set.Set }

func (f fakeGoogle) GithubUsersFromGoogle(context.Context) (set.Set, error) { return f.users, nil }
func (f fakeGoogle) GithubUsersForGroups(context.Context, []string) (map[string]set.Set, error) {
	return nil, nil
}

type fakeGitHub struct {
	users          set.Set
	added, removed set.Set
	syncCalled     bool
	teamMapDesired map[string]set.Set
	teamMapDrift   bool
}

func (f *fakeGitHub) UsersFromGitHub(context.Context) (set.Set, error) { return f.users, nil }
func (f *fakeGitHub) AddUsersToOrg(_ context.Context, u set.Set) error { f.added = u; return nil }
func (f *fakeGitHub) RemoveUsersFromOrg(_ context.Context, u set.Set) error {
	f.removed = u
	return nil
}
func (f *fakeGitHub) SyncTeamMembers(context.Context) error { f.syncCalled = true; return nil }
func (f *fakeGitHub) SyncTeamMap(_ context.Context, desired map[string]set.Set) (bool, error) {
	f.teamMapDesired = desired
	return f.teamMapDrift, nil
}

func init() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) }

func mismatch() (fakeGoogle, *fakeGitHub) {
	return fakeGoogle{users: set.New("a", "d")}, &fakeGitHub{users: set.New("b", "c", "a")}
}

func TestMismatchExitCodeDefault(t *testing.T) {
	g, gh := mismatch()
	code, err := Run(context.Background(), &config.Config{}, g, gh)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("default exit code = %d, want 0", code)
	}
}

func TestMismatchExitCodeConfigured(t *testing.T) {
	g, gh := mismatch()
	code, _ := Run(context.Background(), &config.Config{ExitCodeOnMismatch: 122}, g, gh)
	if code != 122 {
		t.Errorf("exit code = %d, want 122", code)
	}
}

func TestDoesNotAddOrRemoveWhenDisabled(t *testing.T) {
	g, gh := mismatch()
	_, _ = Run(context.Background(), &config.Config{}, g, gh)
	if gh.added != nil {
		t.Errorf("should not add, got %v", gh.added)
	}
	if gh.removed != nil {
		t.Errorf("should not remove, got %v", gh.removed)
	}
}

func TestAddsUsersWhenEnabled(t *testing.T) {
	g, gh := mismatch()
	_, _ = Run(context.Background(), &config.Config{AddUsers: true}, g, gh)
	// "d" is in google but not github.
	if gh.added == nil || !gh.added.Has("d") || gh.added.Len() != 1 {
		t.Errorf("added = %v, want [d]", gh.added)
	}
}

func TestRemovesUsersWhenEnabled(t *testing.T) {
	g, gh := mismatch()
	_, _ = Run(context.Background(), &config.Config{RemoveUsers: true, MaxRemoveUsers: 100}, g, gh)
	// "b" and "c" are in github but not google.
	if gh.removed == nil || !gh.removed.Has("b") || !gh.removed.Has("c") || gh.removed.Len() != 2 {
		t.Errorf("removed = %v, want [b c]", gh.removed)
	}
}

func TestRemoveGuardedByMaxAdds(t *testing.T) {
	// usersNotInGitHub (adds) exceeds MaxRemoveUsers, so removal is skipped.
	g := fakeGoogle{users: set.New("a", "d", "e", "f")}
	gh := &fakeGitHub{users: set.New("b", "a")}
	_, _ = Run(context.Background(), &config.Config{RemoveUsers: true, MaxRemoveUsers: 2}, g, gh)
	if gh.removed != nil {
		t.Errorf("removal should be skipped, got %v", gh.removed)
	}
}

func TestSyncTeamCalledWhenEnabled(t *testing.T) {
	g, gh := mismatch()
	_, _ = Run(context.Background(), &config.Config{SyncTeam: true}, g, gh)
	if !gh.syncCalled {
		t.Error("SyncTeamMembers should have been called")
	}
}

func TestMatchExitsZero(t *testing.T) {
	g := fakeGoogle{users: set.New("a", "b", "c", "d")}
	gh := &fakeGitHub{users: set.New("a", "b", "c", "d")}
	code, _ := Run(context.Background(), &config.Config{ExitCodeOnMismatch: 122, AddUsers: true, RemoveUsers: true, MaxRemoveUsers: 100}, g, gh)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if gh.added != nil || gh.removed != nil {
		t.Errorf("no changes expected when matched: added=%v removed=%v", gh.added, gh.removed)
	}
}

type fakeGoogleGroups struct {
	users  set.Set
	groups map[string]set.Set
}

func (f fakeGoogleGroups) GithubUsersFromGoogle(context.Context) (set.Set, error) {
	return f.users, nil
}
func (f fakeGoogleGroups) GithubUsersForGroups(_ context.Context, _ []string) (map[string]set.Set, error) {
	return f.groups, nil
}

func TestTeamMapSyncUnionsGroupsPerTeam(t *testing.T) {
	g := fakeGoogleGroups{
		users: set.New("a"),
		groups: map[string]set.Set{
			"eng@x":   set.New("a", "b"),
			"infra@x": set.New("b", "c"),
		},
	}
	gh := &fakeGitHub{users: set.New("a")}
	cfg := &config.Config{TeamMap: map[string]string{
		"eng@x":   "platform",
		"infra@x": "platform",
	}}
	if _, err := Run(context.Background(), cfg, g, gh); err != nil {
		t.Fatal(err)
	}
	got := gh.teamMapDesired["platform"]
	if got == nil || got.Len() != 3 || !got.Has("a") || !got.Has("b") || !got.Has("c") {
		t.Errorf("platform desired = %v, want [a b c]", got.Sorted())
	}
}

func TestTeamMapSyncSkippedWhenUnset(t *testing.T) {
	g := fakeGoogleGroups{users: set.New("a")}
	gh := &fakeGitHub{users: set.New("a")}
	if _, err := Run(context.Background(), &config.Config{}, g, gh); err != nil {
		t.Fatal(err)
	}
	if gh.teamMapDesired != nil {
		t.Errorf("SyncTeamMap should not be called, got %v", gh.teamMapDesired)
	}
}

func TestTeamMapDriftDrivesExitCode(t *testing.T) {
	g := fakeGoogleGroups{users: set.New("a"), groups: map[string]set.Set{"eng@x": set.New("a")}}
	gh := &fakeGitHub{users: set.New("a"), teamMapDrift: true}
	cfg := &config.Config{ExitCodeOnMismatch: 7, TeamMap: map[string]string{"eng@x": "eng"}}
	code, err := Run(context.Background(), cfg, g, gh)
	_ = err
	if code != 7 {
		t.Errorf("exit code = %d, want 7 (team drift should flag mismatch)", code)
	}
}
