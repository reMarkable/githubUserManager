# Manage github organisation membership with Google Workspace user accounts

[![GitHub license](https://img.shields.io/github/license/reMarkable/githubUserManager)](https://github.com/reMarkable/githubUserManager/blob/main/LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/reMarkable/githubusermanager)](https://github.com/reMarkable/githubusermanager/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/reMarkable/githubusermanager)](https://github.com/reMarkable/githubusermanager/network)
[![GitHub issues](https://img.shields.io/github/issues/reMarkable/githubusermanager)](https://github.com/reMarkable/githubusermanager/issues)
[![ci](https://github.com/reMarkable/githubUserManager/actions/workflows/ci.yml/badge.svg)](https://github.com/reMarkable/githubUserManager/actions/workflows/ci.yml)

Manages who is in your GitHub organization based on a custom property in their Google Workspace profile, allowing for seamless JML (Joiner mover leaver) process, if allowed removing/disabling an account in the Google Workspace will remove the user from the GitHub; similarly adding a user also works the same way. If you don't want to run it in destructive mode it can be configured to exit with a non-zero exit code so that you know to go and manually make the changes.

Right now this only handles the organization membership, it **does not** touch team membership, or level of membership; the main focus is to draw alert when the configuration isn't as expected, these features could be added in future.

## Deployment

### Collect the secrets

1.  [Add a custom attribute on the users](https://support.google.com/a/answer/6208725?hl=en#zippy=%2Cadd-a-new-custom-attribute)
    1. Go to https://admin.google.com/ac/customschema
    1. Enter
       - Category: `Accounts`
       - Description: `Accounts held elsewhere to link in`
       - Custom fields:
         - name: `github`
         - info type: `text`
         - visibility: `Visible to the organisation`
         - no. of values: `multi-value`

1.  [Add values to custom attributes for the users](https://support.google.com/a/answer/6208725?hl=en#add_value)
    1.  Go to https://admin.google.com/ac/users
    1.  Click a user to edit them
    1.  Click 'user information'
    1.  Under Accounts, click `github`
    1.  Add all the github accounts for that user
    1.  Click Save

1.  [Make a gcp project](https://console.cloud.google.com/projectcreate)
    1. Enable the [Admin SDK API](https://console.cloud.google.com/apis/library/admin.googleapis.com?q=workspace%20admin&id=d0a160dd-c410-4fd0-a951-c47e05309cb9)
    1. [Create credentials](https://console.cloud.google.com/apis/credentials/wizard?project=githubusermanager)
    - Which API are you using?: `Admin SDK API`
    - Are you planning to use this API with App Engine or Compute Engine: `no`
    - Service account name: `githubusermanager`
    - Role: `[none]`
    - Key type: `JSON`
    - Click `Continue`, then confirm `CREATE WITHOUT ROLE`
    - Edit the user, Click `Enable G Suite domain-wide delegation`
    - Product name for the consent screen: `githubusermanager`
    1. [Delegate domain-wide authority to your service account](https://developers.google.com/admin-sdk/directory/v1/guides/delegation)
    - https://admin.google.com/ac/owl/domainwidedelegation
    - Client ID: `client id from user`
    - OAuth scopes:
      - `https://www.googleapis.com/auth/admin.directory.user.readonly`
      - `https://www.googleapis.com/auth/admin.directory.group.member.readonly` (only needed for `team-map`/`team-map-file` group-to-team syncing)

1.  Register new GitHub App
    1. https://github.com/settings/organizations
    - Click `Settings` on your organization
    - Click `Developer settings`
    - Click `GitHub Apps`
    - Click `New GitHub App`
    1. Enter:
    - GitHub App name: `Google workspace github users`
    - Homepage URL: github.com
    - Webhook
      - Active `uncheck`
    - Organization permissions
      - Members: `Read-only`, or `Read-Write` if you want it to
    - Where can this GitHub App be installed? `Only on this account`
    1. Click `Generate a private key` (should download a .pem)
    1. Click `Install App`
    1. Click `Install`
    1. Click `Install`
    - take note of the url, it'll look something like: `github.com/organizations/myorg/settings/installations/15627551`, the installationId is the last number `15627551`

### Run

#### Github Action:

```yaml
# ./.github/workflows/org-membership.yml
name: Github Org Membership

on:
  schedule:
    - cron: '*/5 * * * *'
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - name: Github Org Membership Manager
        uses:
        docker://ghcr.io/remarkable/githubusermanager/githubusermanager:v2.0.0
        with:
          google-email-address: hello@example.com
          google-credentials: ${{ secrets.GOOGLE_CREDENTIALS }}
          add-users: 'false'
          remove-users: 'false'
          exit-code-on-mismatch: '1'
          github-org: 'myorg'
          github-app-id: 1234
          github-installation-id: 12345
          github-private-key: ${{ secrets.GH_APP_PRIVATE_KEY }}
          ignored-users: user1,user2
          sync-team: 'true'
          team-slug: 'remarkable'
```

#### Syncing Google Workspace groups to GitHub teams

Beyond `sync-team` (which adds *every* org member to a single team), you can
mirror the membership of specific Google Workspace groups into matching GitHub
teams. Provide a JSON object that maps a group's email address to a GitHub team
slug. The members of each group are resolved to their linked GitHub accounts
(via the same `Accounts.github` custom attribute) and reconciled against the
mapped team. Nested groups are expanded automatically.

Keep the mapping in a JSON file in your repo for maintainability:

```json
// .github/github-teams.json
{
  "engineering@remarkable.no": "engineering",
  "design@remarkable.no": "design",
  "platform@remarkable.no": "platform-team"
}
```

```yaml
- name: Github Org Membership Manager
  uses: docker://ghcr.io/remarkable/githubusermanager/githubusermanager:v2.0.0
  with:
    # ...the usual inputs...
    github-org: 'reMarkable'
    remove-users: 'true' # also prunes team members no longer in the group
    team-map-file: .github/github-teams.json
```

Notes:

- Members missing from a team are always added. Members present in a team but
  not in the mapped group are only removed when `remove-users` is `true`, so you
  can run in a safe, add-only mode first. Team removals also respect
  `max-remove-users`, and a group that resolves to zero linked GitHub accounts
  will never empty a team (guards against a mistyped group email).
- Any difference between a mapped team and its group sets the
  `exit-code-on-mismatch` exit code, so `dry-run` + `exit-code-on-mismatch`
  reports team drift in CI without applying changes.
- `ignored-users` are never added to or removed from mapped teams.
- Requires the
  `https://www.googleapis.com/auth/admin.directory.group.member.readonly` scope
  on the service account's domain-wide delegation.
- For a single inline mapping you can use `team-map` instead of a file, e.g.
  `team-map: '{"engineering@remarkable.no":"engineering"}'`. `team-map-file`
  takes precedence when both are set.

Add a dependabot configuration to always get updates!

```yaml
# ./.github/dependabot.yml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: daily
```

#### Docker

This action runs as a [Docker container action](https://docs.github.com/en/actions/sharing-automations/creating-actions/creating-a-docker-container-action).
CI publishes a prebuilt image to the GitHub Container Registry at
`ghcr.io/remarkable/githubusermanager/githubusermanager`, tagged per build
(`main`, `pr-<number>`, the release tag, and `sha-<shortsha>`). Reference it
directly and pin to an immutable `sha-` tag, e.g.:

```yaml
- name: Github Org Membership Manager
  uses: docker://ghcr.io/remarkable/githubusermanager/githubusermanager:sha-d84fa48
  with:
    # ...inputs from the table below...
```

Alternatively, `uses: reMarkable/githubUserManager@<ref>` makes GitHub build the
image from the `Dockerfile` at the ref you pin to.

Inputs are read directly from the GitHub Action inputs (exposed by the runner
as `INPUT_*` environment variables). To run the published image by hand, set the
same inputs with the `INPUT_` prefix and the input name upper-cased, e.g.
`INPUT_GITHUB-ORG`, `INPUT_ADD-USERS`.

The easiest way is to copy `.env.example` to `.env`, fill in the
[inputs below](#action-inputs), and pass it with `--env-file`:

```sh
docker run --rm --env-file .env \
  ghcr.io/remarkable/githubusermanager/githubusermanager:main
```

Or set the inputs individually with `-e`:

```sh
docker run --rm \
  -e INPUT_GOOGLE-EMAIL-ADDRESS=foo@example.com \
  -e INPUT_GITHUB-ORG=myorg \
  -e INPUT_ADD-USERS=false \
  ghcr.io/remarkable/githubusermanager/githubusermanager:main
```

#### Go / lambda / cloud run / something else

1.  Clone this repo
1.  `go build -o githubusermanager .`
1.  `./githubusermanager` (with the `INPUT_*` variables from the [table below](#action-inputs) set)

### Action inputs

Inputs use the action input names below; when running the container directly set
them as `INPUT_<NAME>` environment variables (upper-cased input name).

| Input                    | Description                                                                                                                     | Example           | Default |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------- | ----------------- | ------- |
| `google-email-address`   | Email address to assume to, needs to be an workspace admin                                                                      | `foo@example.com` | `null`  |
| `google-credentials`     | Base64'd json as downloaded from the google service account creation step                                                       | `Zm9vCg==`        | `null`  |
| `add-users`              | Set to TRUE to add users to the github organisation                                                                             | `TRUE`            | `false` |
| `remove-users`           | Set to TRUE to remove users from the github organisation                                                                        | `TRUE`            | `false` |
| `remove-suspended_users` | Set to TRUE to remove users from the github organisation that are suspended in Google                                           | `TRUE`            | `false` |
| `dry-run`                | Set to TRUE to log all intended org/team membership changes without applying them (useful with `exit-code-on-mismatch`)         | `TRUE`            | `false` |
| `exit-code-on-mismatch`  | Exit code to use when there's a mismatch, useful when combined with `add-users` and `remove-users` to be used in a dry-run mode | `1`               | `0`     |
| `max-remove-users`       | Safety limit on how many users may be removed in a single run                                                                   | `100`             | `100`   |
| `github-org`             | GitHub Organization                                                                                                             | `chrisnstest`     | `null`  |
| `github-app-id`          | GitHub App ID                                                                                                                   | `106341`          | `null`  |
| `github-installation-id` | Github App Installation ID                                                                                                      | `15627551`        | `null`  |
| `github-private-key`     | Base64'd private key as downloaded from github application registration step                                                    | `Zm9vCg==`        | `null`  |
| `ignored-users`          | Comma separated list of user ids to totally ignore always, useful for owners of an org you don't want accidentally removed      | `owner1,owner2`   | `null`  |
| `sync-team`              | Set to TRUE to ensure every org member is added to the team named in `team-slug`                                                | `TRUE`            | `false` |
| `team-slug`              | Slug of the team every org member should be added to, e.g. to grant PR rights to shared repos                                   | `remarkable`      | `null`  |
| `team-map`               | Inline JSON mapping a Google Workspace group email to a GitHub team slug; members of each group are synced into the mapped team | `{"eng@x":"eng"}` | `null`  |
| `team-map-file`          | Path to a JSON file with the same group-to-team mapping as `team-map` (preferred for maintainability; wins if both are set)      | `.github/teams.json` | `null` |
| `debug`                  | Set to TRUE to enable debug level logging                                                                                       | `TRUE`            | `false` |
