package googlews

import (
	"context"
	"testing"

	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"

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
