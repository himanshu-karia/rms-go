module.exports = {
  root: true,
  env: {
    browser: true,
    es2022: true,
  },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    project: ['./tsconfig.json'],
    tsconfigRootDir: __dirname,
  },
  plugins: [
    '@typescript-eslint',
    'react',
    'react-hooks',
    'testing-library',
    'prettier',
    'tailwindcss',
  ],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
    'plugin:testing-library/react',
    'plugin:tailwindcss/recommended',
    'plugin:prettier/recommended',
  ],
  settings: {
    react: {
      version: 'detect',
    },
  },
  rules: {
    'react/react-in-jsx-scope': 'off',
    '@typescript-eslint/explicit-function-return-type': 'off',
    'no-restricted-syntax': [
      'error',
      {
        selector: "CallExpression[callee.name='fetch']",
        message: 'Use apiFetch to ensure auth headers and credentials are applied.',
      },
    ],
  },
  overrides: [
    {
      files: ['src/api/http.ts', 'src/api/http.js'],
      rules: {
        'no-restricted-syntax': 'off',
      },
    },
  ],
};
