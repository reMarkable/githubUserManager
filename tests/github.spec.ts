
vi.mock('@octokit/rest')
import { Octokit } from '@octokit/rest'
import { config } from '../src/config'
import * as mod from '../src/github'

describe('github integration', () => {
  beforeEach(() => {
    vi.spyOn(config, 'githubPrivateKey', 'get').mockReturnValue('helloworld')
    vi.spyOn(config, 'githubAppID', 'get').mockReturnValue(123)
    vi.spyOn(config, 'githubInstallationID', 'get').mockReturnValue(123)
    vi.spyOn(global.console, 'log').mockImplementation(() => {})
  })

  it('getAuthenticatedOctokit', () => {
    mod.getAuthenticatedOctokit()
    return expect(Octokit).toMatchSnapshot()
  })
  it('getGithubUsersFromGithub', async () => {
    const fakeOctokit = {
      orgs: { listMembers: vi.fn(), listPendingInvitations: vi.fn() },
      paginate: vi
        .fn<() => Promise<{ login: string }[]>>()
        .mockResolvedValueOnce([{ login: 'chrisns' }, { login: 'bar' }, { login: 'foo' }])
        .mockResolvedValueOnce([{ login: 'pending' }, { login: 'chrisns' }, { login: 'anotherpending' }]),
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    vi.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    await expect(mod.getGithubUsersFromGithub()).resolves.toMatchSnapshot()
  })
  it('getUserIdFromUsername found', () => {
    const fakeOctokit = {
      users: {
        getByUsername: vi.fn<() => Promise<{ data: { id: number } }>>().mockResolvedValue({ data: { id: 123 } }),
      },
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    vi.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    return expect(mod.getUserIdFromUsername('foo')).resolves.toMatchSnapshot()
  })

  it('getUserIdFromUsername notfound', () => {
    const fakeOctokit = {
      users: {
        getByUsername: vi.fn<() => Promise<void>>().mockRejectedValue(new Error('not found')),
      },
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    vi.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    return expect(mod.getUserIdFromUsername('foo')).rejects.toMatchSnapshot()
  })

  it('addUsersToGitHubOrg', async () => {
    const users = new Set(['foo', 'bar'])
    vi.spyOn(mod, 'addUserToGitHubOrg').mockResolvedValue(false)
    await mod.addUsersToGitHubOrg(users)
    return expect(mod.addUserToGitHubOrg).toMatchSnapshot()
  })

  it('removeUsersFromGitHubOrg', async () => {
    const users = new Set(['foo', 'bar'])
    vi.spyOn(mod, 'removeUserFromGitHubOrg').mockResolvedValue(false)
    await mod.removeUsersFromGitHubOrg(users)
    return expect(mod.removeUserFromGitHubOrg).toMatchSnapshot()
  })

  it('removeUserFromGitHubOrg skip ignore', async () => {
    vi.spyOn(config, 'ignoredUsers', 'get').mockReturnValue(['foo'])
    await expect(mod.removeUserFromGitHubOrg('foo')).resolves.toBe(false)
  })

  it('addUserToGitHubOrg skip ignore', async () => {
    vi.spyOn(config, 'ignoredUsers', 'get').mockReturnValue(['foo'])
    await expect(mod.addUserToGitHubOrg('foo')).resolves.toBe(false)
  })

  it('addUserToGitHubOrg', async () => {
    const fakeOctokit = {
      orgs: {
        createInvitation: vi.fn<() => Promise<boolean>>().mockResolvedValue(true),
      },
    }
    vi.spyOn(config, 'githubOrg', 'get').mockReturnValue('myorg')
    vi.spyOn(mod, 'getUserIdFromUsername').mockResolvedValue(123)
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    vi.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    await mod.addUserToGitHubOrg('foo')
    return expect(fakeOctokit.orgs.createInvitation).toMatchSnapshot()
  })

  it('removeUserFromGitHubOrg', async () => {
    const fakeOctokit = {
      orgs: {
        removeMembershipForUser: vi.fn<() => Promise<boolean>>().mockResolvedValue(true),
      },
    }
    vi.spyOn(config, 'githubOrg', 'get').mockReturnValue('myorg')
    vi.spyOn(mod, 'getUserIdFromUsername').mockResolvedValue(123)
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    vi.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    await mod.removeUserFromGitHubOrg('foo')
    return expect(fakeOctokit.orgs.removeMembershipForUser).toMatchSnapshot()
  })

  it('formatUserList', () => {
    const response = [{ login: 'chrisns' }, { login: 'chrisns' }, { login: 'foo' }, {}]
    return expect(mod.formatUserList(response)).toEqual(new Set(['chrisns', 'foo']))
  })
})
