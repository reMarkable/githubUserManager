import { jest } from '@jest/globals'

jest.mock('googleapis')

import { google } from 'googleapis'
import * as mod from '../src/google'

const fakeUsersResponse = [
  { customSchemas: { Accounts: { github: [{ value: 'chrisns' }] } } },
  {
    customSchemas: {
      Accounts: { github: [{ value: 'Foo' }, , { value: 'tar' }] },
    },
  },
  {
    customSchemas: {
      Accounts: { github: [{ value: 'foo' }, { value: 'bar' }] },
    },
  },
]

describe('google integration', () => {
  beforeEach(() => {
    process.env.GOOGLE_EMAIL_ADDRESS = 'hello@example.com'
    process.env.GOOGLE_CREDENTIALS = Buffer.from(JSON.stringify({ client_email: 'foo', private_key: 'bar' })).toString(
      'base64',
    )
    jest.spyOn(global.console, 'log').mockImplementation(() => {})
  })
  it('googleAuth', () => {
    mod.googleAuth()
    return expect(google.auth.JWT).toMatchSnapshot()
  })
  it('getAdminService', () => {
    // FIXME: TypeError: google.admin.mockReturnValue is not a function
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    google.admin.mockReturnValue('adminservice')
    const result = mod.getAdminService()
    return expect(result).resolves.toBe('adminservice')
  })

  it('getGithubUsersFromGoogle', () => {
    const service = {
      users: {
        list: jest
          .fn<() => Promise<{ data: { users: typeof fakeUsersResponse } }>>()
          .mockResolvedValue({ data: { users: fakeUsersResponse } }),
      },
    }
    // FIXME: TypeError: Cannot assign to read only property 'getAdminService' of object '[object Module]'
    // @ts-expect-error mock service isn't a complete implementation, so being lazy and just doing the bare minimum
    jest.spyOn(mod, 'getAdminService').mockResolvedValue(service)

    const result = mod.getGithubUsersFromGoogle()
    expect(result).resolves.toMatchSnapshot()
  })

  it('formatUserList', () => expect(mod.formatUserList(fakeUsersResponse)).toMatchSnapshot())

  it('formatUserList bad', () =>
    expect(
      mod.formatUserList([
        {},
        { customSchemas: {} },
        { customSchemas: { Accounts: {} } },
        { customSchemas: { Accounts: { github: [] } } },
        { customSchemas: { Accounts: { github: [{}] } } },
        { customSchemas: { Accounts: { github: [{ value: 'chrisns' }] } } },
      ]),
    ).toMatchSnapshot())
})
