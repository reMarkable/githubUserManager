import { google, type Auth, type admin_directory_v1 } from 'googleapis'

import * as mod from './google.js'
import { config } from './config.js'

export async function googleAuth(): Promise<Auth.JWT> {
  const { googleCredentials, googleEmailAddress } = config
  const jwtClient = new google.auth.JWT({
    email: googleCredentials.client_email,
    key: googleCredentials.private_key,
    scopes: ['https://www.googleapis.com/auth/admin.directory.user.readonly'],
    subject: googleEmailAddress,
  })
  await jwtClient.authorize()
  return jwtClient
}

export async function getAdminService(): Promise<admin_directory_v1.Admin> {
  return google.admin({
    auth: await googleAuth(),
    version: 'directory_v1',
  })
}

// oxlint-disable-next-line no-explicit-any
export function formatUserList(users: any[]): Set<string> {
  return new Set(
    users
      .flatMap((user) =>
        user.customSchemas?.Accounts?.github?.map((account: { value: string }) => account.value?.toLowerCase().trim()),
      )
      .filter(Boolean),
  )
}

export async function getGithubUsersFromGoogle(): Promise<Set<string>> {
  const service = await mod.getAdminService()
  let githubAccounts = new Set<string>()
  let pageToken = null

  do {
    const userList = await service.users.list({
      customFieldMask: 'Accounts',
      customer: 'my_customer',
      fields: 'users(customSchemas/Accounts/github(value)),nextPageToken',
      maxResults: 250,
      pageToken,
      projection: 'custom',
      query: config.removeSuspendedUsers ? 'isSuspended=false' : '',
    })
    pageToken = userList.data.nextPageToken
    githubAccounts = new Set([...githubAccounts, ...formatUserList(userList.data.users)])
  } while (pageToken != null)
  return githubAccounts
}
