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
);
