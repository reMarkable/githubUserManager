import { createAppAuth } from '@octokit/auth-app'
import { Octokit, type RestEndpointMethodTypes } from '@octokit/rest'

import * as mod from './github.js'
import { config } from './config.js'

export function getAuthenticatedOctokit(): Octokit {
  return new Octokit({
    authStrategy: createAppAuth,
    auth: {
      appId: config.githubAppID,
      privateKey: config.githubPrivateKey,
      installationId: config.githubInstallationID,
    },
  })
}

export async function getGithubUsersFromGithub(): Promise<Set<string>> {
  const octokit = mod.getAuthenticatedOctokit()
  const members = await octokit.paginate(octokit.orgs.listMembers, {
    org: config.githubOrg,
  })

  const pendingInvites = await octokit.paginate(octokit.orgs.listPendingInvitations, {
    org: config.githubOrg,
  })
  const pendingGithubAccounts = formatUserList(pendingInvites)

  const githubAccounts = formatUserList(members)

  if (pendingGithubAccounts.size > 0)
    console.log(`Outstanding GitHub invites for ${Array.from(pendingGithubAccounts).join(', ')}`)

  return new Set([...githubAccounts, ...pendingGithubAccounts])
}

export function formatUserList(users): Set<string> {
  return new Set(
    users
      .map((user) => user.login?.toLowerCase())
      .flat()
      .filter(Boolean),
  )
}

export async function getUserIdFromUsername(username: string): Promise<number> {
  const octokit = mod.getAuthenticatedOctokit()
  console.log(`Looking up user ${username}`)
  let user
  try {
    user = await octokit.users.getByUsername({ username })
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
  } catch (error) {
    throw `Unable to find user id for ${username}`
  }
  console.log(`User ${username} userid: ${user.data.id}`)
  return user.data.id
}

export async function addUsersToGitHubOrg(users: Set<string>): Promise<void> {
  for (const user of users) {
    await mod.addUserToGitHubOrg(user)
  }
}

export async function addUserToGitHubOrg(
  user: string,
): Promise<RestEndpointMethodTypes['orgs']['createInvitation']['response'] | boolean> {
  const octokit = mod.getAuthenticatedOctokit()
  if (config.ignoredUsers.includes(user.toLowerCase())) {
    console.log(`Ignoring add for ${user}`)
    return false
  }
  const userId = await mod.getUserIdFromUsername(user)
  console.log(`Inviting ${user} (${userId} to ${config.githubOrg})`)
  return await octokit.orgs.createInvitation({
    org: config.githubOrg,
    invitee_id: userId,
  })
}

export async function removeUsersFromGitHubOrg(users: Set<string>): Promise<void> {
  for (const user of users) {
    await mod.removeUserFromGitHubOrg(user)
  }
}

export async function removeUserFromGitHubOrg(
  user: string,
): Promise<RestEndpointMethodTypes['orgs']['removeMembershipForUser']['response'] | boolean> {
  const octokit = mod.getAuthenticatedOctokit()
  if (config.ignoredUsers.includes(user.toLowerCase())) {
    console.log(`Ignoring remove for ${user}`)
    return false
  }
  console.log(`Removing user/invitation ${user} from ${config.githubOrg}`)
  return octokit.orgs.removeMembershipForUser({
    org: config.githubOrg,
    username: user,
  })
}
