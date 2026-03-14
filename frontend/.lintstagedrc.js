const path = require('path')

// Build ESLint command that passes relative file paths
// --fix automatically fixes auto-fixable issues (import order, etc.)
const buildEslintCommand = (filenames) =>
  `npx eslint --fix ${filenames.map((f) => `"${path.relative(process.cwd(), f)}"`).join(' ')}`

module.exports = {
  // TypeScript and JavaScript: ESLint fix then Prettier format
  '*.{js,jsx,ts,tsx}': [buildEslintCommand, 'npx prettier --write'],
  // JSON, CSS, Markdown: Prettier format only
  '*.{json,css,md}': ['npx prettier --write'],
}
