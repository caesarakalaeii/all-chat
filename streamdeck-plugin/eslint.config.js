// Minimal but real lint config: TypeScript-aware rules over `src/`, plus the two
// project-specific rules that matter here — no stray `console` (Stream Deck
// plugins must log through `streamDeck.logger`, which writes to the plugin log)
// and no unused code.
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
	{
		ignores: ["com.allchat.streamdeck.sdPlugin/bin/**", "node_modules/**"],
	},
	js.configs.recommended,
	...tseslint.configs.recommended,
	{
		files: ["src/**/*.ts"],
		rules: {
			// The plugin writes to the Stream Deck log, never to stdout: stdout is
			// the SDK's own transport in some hosts.
			"no-console": "error",
			"@typescript-eslint/no-unused-vars": [
				"error",
				{ argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
			],
			eqeqeq: ["error", "smart"],
			"prefer-const": "error",
		},
	},
	{
		// Property inspector scripts. These run in the Stream Deck app's embedded
		// browser, not in the plugin's Node process, so they need browser globals plus
		// the `SDPIComponents` global that sdpi-components.js installs.
		//
		// They are linted at all only because they were moved out of inline <script>
		// blocks in the three HTML pages, where ESLint never saw them. The global list
		// is enumerated rather than pulled in wholesale from `globals.browser`: a
		// property inspector should be touching almost nothing, and a new name showing
		// up here is worth noticing.
		files: ["com.allchat.streamdeck.sdPlugin/ui/**/*.js"],
		languageOptions: {
			globals: {
				document: "readonly",
				setTimeout: "readonly",
				clearTimeout: "readonly",
				SDPIComponents: "readonly",
			},
		},
	},
);
