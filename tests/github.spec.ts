import { jest } from '@jest/globals'

jest.mock('@octokit/rest')
import { Octokit } from '@octokit/rest'
import { config } from '../src/config'
import * as mod from '../src/github'

describe('github integration', () => {
  beforeEach(() => {
    jest.spyOn(config, 'githubPrivateKey', 'get').mockReturnValue('helloworld')
    jest.spyOn(config, 'githubAppID', 'get').mockReturnValue(123)
    jest.spyOn(config, 'githubInstallationID', 'get').mockReturnValue(123)
    jest.spyOn(global.console, 'log').mockImplementation(() => {})
  })

  it('getAuthenticatedOctokit', () => {
    mod.getAuthenticatedOctokit()
    return expect(Octokit).toMatchSnapshot()
  })
  it('getGithubUsersFromGithub', () => {
    const fakeOctokit = {
      paginate: jest
        .fn<() => Promise<{ login: string }[]>>()
        .mockResolvedValueOnce([{ login: 'chrisns' }, { login: 'bar' }, { login: 'foo' }])
        .mockResolvedValueOnce([{ login: 'pending' }, { login: 'chrisns' }, { login: 'anotherpending' }]),
      orgs: { listMembers: jest.fn(), listPendingInvitations: jest.fn() },
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    expect(mod.getGithubUsersFromGithub()).resolves.toMatchSnapshot()
  })
  it('getUserIdFromUsername found', () => {
    const fakeOctokit = {
      users: {
        getByUsername: jest.fn<() => Promise<{ data: { id: number } }>>().mockResolvedValue({ data: { id: 123 } }),
      },
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    return expect(mod.getUserIdFromUsername('foo')).resolves.toMatchSnapshot()
  })

  it('getUserIdFromUsername notfound', () => {
    const fakeOctokit = {
      users: {
        getByUsername: jest.fn<() => Promise<void>>().mockRejectedValue(new Error('not found')),
      },
    }
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    return expect(mod.getUserIdFromUsername('foo')).rejects.toMatchSnapshot()
  })

  it('addUsersToGitHubOrg', async () => {
    const users = new Set(['foo', 'bar'])
    jest.spyOn(mod, 'addUserToGitHubOrg').mockResolvedValue(false)
    await mod.addUsersToGitHubOrg(users)
    return expect(mod.addUserToGitHubOrg).toMatchSnapshot()
  })

  it('removeUsersFromGitHubOrg', async () => {
    const users = new Set(['foo', 'bar'])
    jest.spyOn(mod, 'removeUserFromGitHubOrg').mockResolvedValue(false)
    await mod.removeUsersFromGitHubOrg(users)
    return expect(mod.removeUserFromGitHubOrg).toMatchSnapshot()
  })

  it('removeUserFromGitHubOrg skip ignore', () => {
    jest.spyOn(config, 'ignoredUsers', 'get').mockReturnValue(['foo'])
    expect(mod.removeUserFromGitHubOrg('foo')).resolves.toBe(false)
  })

  it('addUserToGitHubOrg skip ignore', () => {
    jest.spyOn(config, 'ignoredUsers', 'get').mockReturnValue(['foo'])
    expect(mod.addUserToGitHubOrg('foo')).resolves.toBe(false)
  })

  it('addUserToGitHubOrg', async () => {
    const fakeOctokit = {
      orgs: {
        createInvitation: jest.fn<() => Promise<boolean>>().mockResolvedValue(true),
      },
    }
    jest.spyOn(config, 'githubOrg', 'get').mockReturnValue('myorg')
    jest.spyOn(mod, 'getUserIdFromUsername').mockResolvedValue(123)
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    await mod.addUserToGitHubOrg('foo')
    return expect(fakeOctokit.orgs.createInvitation).toMatchSnapshot()
  })

  it('removeUserFromGitHubOrg', async () => {
    const fakeOctokit = {
      orgs: {
        removeMembershipForUser: jest.fn<() => Promise<boolean>>().mockResolvedValue(true),
      },
    }
    jest.spyOn(config, 'githubOrg', 'get').mockReturnValue('myorg')
    jest.spyOn(mod, 'getUserIdFromUsername').mockResolvedValue(123)
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAuthenticatedOctokit').mockReturnValue(fakeOctokit)
    await mod.removeUserFromGitHubOrg('foo')
    return expect(fakeOctokit.orgs.removeMembershipForUser).toMatchSnapshot()
  })

  it('formatUserList', () => {
    const response = [{ login: 'chrisns' }, { login: 'chrisns' }, { login: 'foo' }, {}]
    return expect(mod.formatUserList(response)).toEqual(new Set(['chrisns', 'foo']))
  })
})
