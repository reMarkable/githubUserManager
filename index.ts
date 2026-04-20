import { getGithubUsersFromGoogle } from './src/google.js'
import { addUsersToGitHubOrg, getGithubUsersFromGithub, removeUsersFromGitHubOrg } from './src/github.js'
import { config } from './src/config.js'

export async function run(): Promise<void> {
  const googleUsers = await getGithubUsersFromGoogle()
  console.log(`Users from google: ${[...googleUsers].join(', ')}`)

  const gitHubUsers = await getGithubUsersFromGithub()
  console.log(`Users from github: ${[...gitHubUsers].join(', ')}`)

  const usersNotInGithub = new Set([...googleUsers].filter((x) => !gitHubUsers.has(x)))

  const usersNotInGoogle = new Set([...gitHubUsers].filter((x) => !googleUsers.has(x)))
  if (usersNotInGoogle.size > 0) {
    console.log(`Users not in google: ${[...usersNotInGoogle].join(', ')}`)

    if (config.removeUsers) {
      if (usersNotInGithub.size <= config.maxRemoveUsers) {
        await removeUsersFromGitHubOrg(usersNotInGoogle)
      } else {
        console.log(`Not removing users because there are too many`)
      }
    }
  }

  if (usersNotInGithub.size > 0) {
    console.log(`Users not in github: ${[...usersNotInGithub].join(', ')}`)
    if (config.addUsers) {await addUsersToGitHubOrg(usersNotInGithub)}
  }

  const exitCode = usersNotInGoogle.size > 0 || usersNotInGithub.size > 0 ? config.exitCodeOnMissmatch : 0

  // oxlint-disable-next-line no-process-exit
  process.exit(exitCode)
}

// istanbul ignore next
if (import.meta.url.endsWith(process.argv[1])) {run()}
