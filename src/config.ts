export const config = {
  get addUsers(): boolean {
    return process.env.ADD_USERS?.toLowerCase() === 'true'
  },
  get exitCodeOnMissmatch(): number {
    return Number.parseInt(process.env.EXIT_CODE_ON_MISMATCH ?? '0') || 0
  },
  get githubAppID(): number {
    return Number.parseInt(process.env.GITHUB_APP_ID)
  },
  get githubInstallationID(): number {
    return Number.parseInt(process.env.GITHUB_INSTALLATION_ID)
  },
  get githubOrg(): string {
    return process.env.GITHUB_ORG ?? ''
  },
  get githubPrivateKey(): string {
    return Buffer.from(process.env.GITHUB_PRIVATE_KEY, 'base64').toString('utf8')
  },
  get googleCredentials(): googleCredentials {
    return JSON.parse(Buffer.from(process.env.GOOGLE_CREDENTIALS, 'base64').toString('utf8'))
  },
  get googleEmailAddress(): string {
    return process.env.GOOGLE_EMAIL_ADDRESS ?? ''
  },
  get ignoredUsers(): string[] {
    return process.env.IGNORED_USERS?.toLowerCase().split(',') ?? []
  },
  get maxRemoveUsers(): number {
    return Number.parseInt(process.env.MAX_REMOVE_USERS ?? '0') || 100
  },
  get removeSuspendedUsers(): boolean {
    return process.env.REMOVE_SUSPENDED_USERS?.toLowerCase() === 'true'
  },
  get removeUsers(): boolean {
    return process.env.REMOVE_USERS?.toLowerCase() === 'true'
  },
}

export interface googleCredentials {
  type: string
  project_id: string
  private_key_id: string
  private_key: string
  client_email: string
  client_id: string
  auth_uri: string
  token_uri: string
  auth_provider_x509_cert_url: string
  client_x509_cert_url: string
}
