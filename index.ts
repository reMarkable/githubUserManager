import { getGithubUsersFromGoogle } from './src/google.js'
import { getGithubUsersFromGithub, addUsersToGitHubOrg, removeUsersFromGitHubOrg } from './src/github.js'
import { config } from './src/config.js'

export async function run(): Promise<void> {
  const googleUsers = await getGithubUsersFromGoogle()
  console.log(`Users from google: ${Array.from(googleUsers).join(', ')}`)

  const gitHubUsers = await getGithubUsersFromGithub()
  console.log(`Users from github: ${Array.from(gitHubUsers).join(', ')}`)

  const usersNotInGithub = new Set(Array.from(googleUsers).filter((x) => !gitHubUsers.has(x)))

  const usersNotInGoogle = new Set(Array.from(gitHubUsers).filter((x) => !googleUsers.has(x)))
  if (usersNotInGoogle.size > 0) {
    console.log(`Users not in google: ${Array.from(usersNotInGoogle).join(', ')}`)

    if (config.removeUsers) {
      if (usersNotInGithub.size <= config.maxRemoveUsers) {
        await removeUsersFromGitHubOrg(usersNotInGoogle)
      } else {
        console.log(`Not removing users because there are too many`)
      }
    }
  }

  if (usersNotInGithub.size > 0) {
    console.log(`Users not in github: ${Array.from(usersNotInGithub).join(', ')}`)
    if (config.addUsers) await addUsersToGitHubOrg(usersNotInGithub)
  }

  const exitCode = usersNotInGoogle.size > 0 || usersNotInGithub.size > 0 ? config.exitCodeOnMissmatch : 0

  process.exit(exitCode)
}

// istanbul ignore next
if (import.meta.url.endsWith(process.argv[1])) run()
