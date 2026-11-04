package googlews

import (
	"context"
	"fmt"
	"strings"
	"testing"

	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"

	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/set"
)

func user(accountsJSON string) *admin.User {
	u := &admin.User{CustomSchemas: map[string]googleapi.RawMessage{}}
	if accountsJSON != "" {
		u.CustomSchemas["Accounts"] = googleapi.RawMessage(accountsJSON)
	}
	return u
}

func TestFormatUserList(t *testing.T) {
	users := []*admin.User{
		user(`{"github":[{"value":"chrisns"}]}`),
		user(`{"github":[{"value":"Foo"},{"value":"tar"}]}`),
		user(`{"github":[{"value":"foo"},{"value":"bar"}]}`),
		user(`{"github":[{"value":"userspace "}]}`),
		user(`{"github":[{}]}`),
		user(`{"github":[]}`),
		user(`{}`),
		user(""),
		nil,
	}
	got := set.New()
	for _, u := range formatUserList(users) {
		got.Add(u)
	}
	want := set.New("chrisns", "foo", "tar", "bar", "userspace")
	if got.Len() != want.Len() {
		t.Fatalf("got %v want %v", got.Sorted(), want.Sorted())
	}
	for _, u := range want.Sorted() {
		if !got.Has(u) {
			t.Errorf("missing %s", u)
		}
	}
}

// fakeUsers implements usersListCaller with in-memory paging.
type fakeUsers struct{ pages []*admin.Users }

func (f *fakeUsers) List(_ context.Context, token string) (*admin.Users, error) {
	idx := 0
	if token != "" {
		for i, p := range f.pages {
			if p.NextPageToken == token {
				idx = i + 1
			}
		}
	}
	return f.pages[idx], nil
}

type errUsers struct{}

func (errUsers) List(context.Context, string) (*admin.Users, error) {
	return nil, fmt.Errorf("boom")
}

func TestListErrorNamesSubject(t *testing.T) {
	s := &Service{cfg: &config.Config{GoogleEmailAddress: "admin@example.com"}, users: errUsers{}}
	_, err := s.GithubUsersFromGoogle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "admin@example.com") {
		t.Fatalf("err = %v, want subject", err)
	}
}

func TestScopesForTeamMap(t *testing.T) {
	got := scopesFor(&config.Config{})
	if len(got) != 1 || got[0] != userScope {
		t.Fatalf("no team map: %v", got)
	}
	got = scopesFor(&config.Config{TeamMap: map[string]string{"eng@x": "eng"}})
	if len(got) != 2 || got[0] != userScope || got[1] != memberScope {
		t.Fatalf("team map: %v", got)
	}
}

func TestGithubUsersFromGooglePaginates(t *testing.T) {
	s := &Service{users: &fakeUsers{pages: []*admin.Users{
		{Users: []*admin.User{user(`{"github":[{"value":"a"}]}`)}, NextPageToken: "p2"},
		{Users: []*admin.User{user(`{"github":[{"value":"b"},{"value":"a"}]}`)}},
	}}}
	got, err := s.GithubUsersFromGoogle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 2 || !got.Has("a") || !got.Has("b") {
		t.Errorf("got %v, want [a b]", got.Sorted())
	}
}

func userWithEmail(email, accountsJSON string) *admin.User {
	u := user(accountsJSON)
	u.PrimaryEmail = email
	return u
}

// fakeMembers implements membersListCaller with in-memory group membership.
type fakeMembers struct{ groups map[string][]*admin.Member }

func (f *fakeMembers) ListMembers(_ context.Context, groupKey, _ string) (*admin.Members, error) {
	return &admin.Members{Members: f.groups[groupKey]}, nil
}

func TestGithubUsersForGroupsResolvesMembers(t *testing.T) {
	users := &fakeUsers{pages: []*admin.Users{{Users: []*admin.User{
		userWithEmail("alice@remarkable.no", `{"github":[{"value":"Alice"}]}`),
		userWithEmail("bob@remarkable.no", `{"github":[{"value":"bob"}]}`),
		userWithEmail("carol@remarkable.no", `{"github":[{"value":"carol"}]}`),
		userWithEmail("nolink@remarkable.no", ""),
	}}}}
	members := &fakeMembers{groups: map[string][]*admin.Member{
		"eng@remarkable.no": {
			{Email: "alice@remarkable.no", Type: "USER"},
			{Email: "nested@remarkable.no", Type: "GROUP"},
			{Email: "nolink@remarkable.no", Type: "USER"},
		},
		"nested@remarkable.no": {
			{Email: "carol@remarkable.no", Type: "USER"},
			// cycle back into the parent group; must not recurse forever.
			{Email: "eng@remarkable.no", Type: "GROUP"},
		},
	}}
	s := &Service{cfg: &config.Config{}, users: users, members: members}

	got, err := s.GithubUsersForGroups(context.Background(), []string{"eng@remarkable.no"})
	if err != nil {
		t.Fatal(err)
	}
	eng := got["eng@remarkable.no"]
	want := set.New("alice", "carol")
	if eng.Len() != want.Len() || !eng.Has("alice") || !eng.Has("carol") {
		t.Errorf("got %v, want %v", eng.Sorted(), want.Sorted())
	}
}
