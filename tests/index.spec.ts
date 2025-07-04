import { jest } from '@jest/globals'

jest.mock('../src/google.js')
jest.mock('../src/github.js')

import * as google from '../src/google.js'
import * as github from '../src/github.js'
import * as mod from '../index.js'

let processExitSpy
let consoleSpy

beforeEach(() => {
  processExitSpy = jest.spyOn(global.process, 'exit').mockImplementation(() => {
    return undefined as never
  })
  consoleSpy = jest.spyOn(global.console, 'log').mockImplementation(() => {})
})

describe('missmatch', () => {
  beforeEach(() => {
    // @ts-expect-error mockResolved unexpected
    google.getGithubUsersFromGoogle.mockResolvedValue(new Set(['a', 'd']))
    // @ts-expect-error mockResolved unexpected
    github.getGithubUsersFromGithub.mockResolvedValue(new Set(['b', 'c', 'a']))
  })
  it('should have consistent console output', async () => {
    await mod.run()
    return expect(consoleSpy).toMatchSnapshot()
  })

  it('should exit with 0 by default as there is a missmatch', async () => {
    await mod.run()
    return expect(processExitSpy).toHaveBeenCalledWith(0)
  })
  it('should exit with 122 if defined when there is a missmatch', async () => {
    process.env.EXIT_CODE_ON_MISMATCH = '122'
    await mod.run()
    return expect(processExitSpy).toHaveBeenCalledWith(122)
  })
  it('should not add users if not set to', async () => {
    delete process.env.ADD_USERS
    await mod.run()
    return expect(github.addUsersToGitHubOrg).not.toHaveBeenCalled()
  })
  it('should not remove users if not set to', async () => {
    delete process.env.REMOVE_USERS
    await mod.run()
    return expect(github.removeUsersFromGitHubOrg).not.toHaveBeenCalled()
  })
  it('should add users if set to', async () => {
    process.env.ADD_USERS = 'true'
    await mod.run()
    return expect(github.addUsersToGitHubOrg).toMatchSnapshot()
  })
  it('should remove users if set to', async () => {
    process.env.REMOVE_USERS = 'true'
    await mod.run()
    return expect(github.removeUsersFromGitHubOrg).toMatchSnapshot()
  })
  it('should have consistent console output with full destructive mode on', async () => {
    process.env.REMOVE_USERS = 'true'
    process.env.ADD_USERS = 'true'
    await mod.run()
    return expect(consoleSpy).toMatchSnapshot()
  })
})

describe('match', () => {
  beforeEach(() => {
    // @ts-expect-error mockResolved unexpected
    google.getGithubUsersFromGoogle.mockResolvedValue(new Set(['a', 'b', 'c', 'd']))
    // @ts-expect-error mockResolved unexpected
    github.getGithubUsersFromGithub.mockResolvedValue(new Set(['a', 'b', 'c', 'd']))
  })
  it('should have consistent console output', async () => {
    await mod.run()
    return expect(consoleSpy).toMatchSnapshot()
  })
  it('should exit with 0 by default', async () => {
    await mod.run()
    return expect(processExitSpy).toHaveBeenCalledWith(0)
  })
  it('should not exit with 122 if defined', async () => {
    process.env.EXIT_CODE_ON_MISMATCH = '122'
    await mod.run()
    return expect(processExitSpy).not.toHaveBeenCalledWith(122)
  })
  it('should not add users', async () => {
    process.env.ADD_USERS = 'true'
    await mod.run()
    return expect(github.addUsersToGitHubOrg).not.toHaveBeenCalled()
  })
  it('should not remove users', async () => {
    process.env.REMOVE_USERS = 'true'
    await mod.run()
    return expect(github.removeUsersFromGitHubOrg).not.toHaveBeenCalled()
  })
})
