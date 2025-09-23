module.exports = {
    env: {
        browser: true,
        es2021: true,
        node: true,
    },
    extends: [
        'eslint:recommended',
        'plugin:react/recommended',
        'plugin:react-hooks/recommended',
        'plugin:@typescript-eslint/recommended',
        'plugin:prettier/recommended',
    ],
    parser: '@typescript-eslint/parser',
    parserOptions: {
        ecmaFeatures: {
            jsx: true,
        },
        ecmaVersion: 'latest',
        sourceType: 'module',
    },
    plugins: ['react', 'react-hooks', '@typescript-eslint', 'prettier'],
    settings: {
        react: {
            version: 'detect',
        },
    },
    rules: {
        'prettier/prettier': ['error'],
        'react/react-in-jsx-scope': 'off',
        'react/prop-types': 'off',
        indent: ['error', 4, { SwitchCase: 1 }],
        'react/jsx-indent': ['error', 4],
        'react/jsx-indent-props': ['error', 4],
        '@typescript-eslint/explicit-module-boundary-types': 'off',
        '@typescript-eslint/no-explicit-any': 'off',
    },
};
