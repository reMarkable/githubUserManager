import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import prettierRecommended from 'eslint-plugin-prettier/recommended'

/** @type {import('@typescript-eslint/utils').TSESLint.FlatConfig.ConfigFile} */
export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  prettierRecommended,
  {
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
]
