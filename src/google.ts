import { google } from 'googleapis'

import * as mod from './google.js'
import { config } from './config.js'

export async function googleAuth() {
  const { googleCredentials, googleEmailAddress } = config
  const jwtClient = new google.auth.JWT(
    googleCredentials.client_email,
    null,
    googleCredentials.private_key,
    ['https://www.googleapis.com/auth/admin.directory.user.readonly'],
    googleEmailAddress,
  )
  await jwtClient.authorize()
  return jwtClient
}

export async function getAdminService() {
  return google.admin({
    version: 'directory_v1',
    auth: await googleAuth(),
  })
}

export async function getGithubUsersFromGoogle(): Promise<Set<string>> {
  const service = await mod.getAdminService()
  let githubAccounts = new Set<string>()
  let pageToken = null

  do {
    const userList = await service.users.list({
      customer: 'my_customer',
      maxResults: 250,
      projection: 'custom',
      fields: 'users(customSchemas/Accounts/github(value)),nextPageToken',
      customFieldMask: 'Accounts',
      pageToken: pageToken,
      query: config.removeSuspendedUsers ? 'isSuspended=false' : '',
    })
    pageToken = userList.data.nextPageToken
    githubAccounts = new Set([...githubAccounts, ...formatUserList(userList.data.users)])
  } while (pageToken != null)
  return githubAccounts
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function formatUserList(users: any[]): Set<string> {
  return new Set(
    users
      .map((user) =>
        user.customSchemas?.Accounts?.github?.map((account: { value: string }) => account.value?.toLowerCase().trim()),
      )
      .flat()
      .filter(Boolean),
  )
}
