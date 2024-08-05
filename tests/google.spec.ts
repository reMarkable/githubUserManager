import { jest } from '@jest/globals'

// FIXME: This is not mocking anything
// Ref: https://jestjs.io/docs/ecmascript-modules
jest.unstable_mockModule('googleapis', () => ({
  google: {
    admin: jest.fn(() => {
      console.log('🔥')
    }),
  },
}))

const { google } = await import('googleapis')
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

// FIXME: This is to avoid crypto calls from google SDK from crashing while working out how to mock things
const EXAMPLE_PRIVATE_KEY = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgQCTjG4qPVrdaoQINLIVrCY1w7n2EujgalMIApYWfXuGdnw2Z+pV
eBtRhS7tHOiMvVi9Fw0Xgos+BAGN9gBVwyXTM5YDesnHEiuYyFwUMUGy9WL2FmOj
4+XWa6xuMU3gNjQUEZDd7Z2Fv1M+YujoPBnTnR4p4fmm5NJleh+Vx+RykwIDAQAB
AoGACj1Al+XSHHN73iXqBd8Ad9KxYQZG6uv1/yROzhi/LcGjNRNZC71eB/Y6H3JW
IQ4yzkx/OBZzG0aj5PohY9I7GaYIxsgq4ff20L+dJstc8gInS5zVeMdzAtG96T1w
10GkcfmRInXmMwQDzlPNtIxqZWzKfWA1G+wk9GQSbS2DVcECQQDJfDqeJNNOQynn
MEENLJdJ9IXFrPy/J3oDit1HCFnhUuElaHLmoVaVj8Egtw55EQqza8oDCJfiL1wG
3JCEXU/ZAkEAu3hNpkneqmviBYoHWQIeMnNK7rXi+41f94NXFkjc4ERJxmHUrcl5
HTiposivFqA1tT2fcAaqhWbkBFPWNcO+SwJAXXYCutjaK7N7/IGlqzbD9so2Qzu7
AfCN6JbBjfGRv7NyPKf2pISyZS4Jp+NCE2aUUCXvzvhte738MjKd1shyAQJATvV9
pwNH8Hmd6f1X2Opmc5NRxTcAkjjaabJ89JrdxOueMxsza7sG8wLRmAl0jVAPLvX0
eEemfKd4dEYnW9/E/wJAPWowAmwgxsSc4p/JyeiEsX7vIcit8elx4ZToeNzLaRXe
LFY80gky65Ccv1yHJJ+eHkRMpqiISNFJ2CWLRQ71SA==
-----END RSA PRIVATE KEY-----`

describe('google integration', () => {
  beforeEach(() => {
    process.env.GOOGLE_EMAIL_ADDRESS = 'hello@example.com'
    process.env.GOOGLE_CREDENTIALS = Buffer.from(
      JSON.stringify({
        client_email: 'foo',
        private_key: EXAMPLE_PRIVATE_KEY,
      }),
    ).toString('base64')
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
