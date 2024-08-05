// This project uses ESM at runtime. Since ESM is very much unsupported in jest for now, we have to transform ESM things to CJS format
// Jest is transforming own code using the babel-jest and we need to do the same for third party ESM code
// If you add any third party ESM only modules from npm, be sure to add them here:
const ESM_MODULES = ['@octokit/.*', 'before-after-hook', 'universal-github-app-jwt', 'universal-user-agent']

module.exports = {
  coverageReporters: ['lcovonly', 'text'],
  collectCoverage: true,
  coverageDirectory: 'coverage',
  collectCoverageFrom: [
    '**/*.ts',
    '**/*.js',
    '!.**',
    '!tests/**',
    '!node_modules/**',
    '!coverage/**',
    '!jest.config.js',
  ],
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node'],
  moduleNameMapper: {
    '^(\\.{1,2}/.*)\\.js$': '$1',
  },
  restoreMocks: true,
  resetMocks: true,
  testEnvironment: 'node',
  testRegex: '(/__tests__/.*|(\\.|/)(test|spec))\\.tsx?$',
  transformIgnorePatterns: [`/node_modules/(?!(${ESM_MODULES.join('|')})/)`],
}
