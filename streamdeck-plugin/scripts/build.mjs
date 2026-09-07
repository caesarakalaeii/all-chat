/**
 * Builds the installable plugin: type-check, then bundle `src/` into the single
 * file `manifest.json` points `CodePath` at.
 *
 * WHY A BUNDLER AND NOT PLAIN `tsc`:
 *
 * The deliverable is the `.sdPlugin` folder — that is what `@elgato/cli pack`
 * zips and what a user installs. It contains no `node_modules`, so anything
 * `bin/plugin.js` still `import`s by bare specifier at runtime is unresolvable
 * on the target machine. `tsc` only transpiles; it rewrites nothing, so its
 * output kept `import ... from "@elgato/streamdeck"` and the plugin died on
 * startup with ERR_MODULE_NOT_FOUND everywhere except a dev checkout, where the
 * sibling `node_modules/` happened to satisfy it. The failure was invisible to
 * lint, build, `validate` and `pack` alike — every one of them passes on a
 * plugin that cannot start. `scripts/smoke.mjs` is the check that does not.
 *
 * So the SDK and its transitive deps (`ws`, `@elgato/schemas`, `@elgato/utils`)
 * are inlined. Only Node builtins stay external.
 */
import { rm } from "node:fs/promises";
import * as esbuild from "esbuild";

const OUT_DIR = "com.allchat.streamdeck.sdPlugin/bin";

// `ws` is CommonJS and calls `require("events")`. In an ESM bundle there is no
// ambient `require`, so esbuild's shim throws "Dynamic require of X is not
// supported" at import time. Handing it a real one built from `import.meta.url`
// is the standard fix and keeps `ws`'s optional native deps (bufferutil,
// utf-8-validate) failing softly into its own try/catch, as they do normally.
const REQUIRE_SHIM =
	'import { createRequire as __allchatCreateRequire } from "node:module";\n' +
	"const require = __allchatCreateRequire(import.meta.url);";

// Stale per-module output from the previous `tsc`-based build would otherwise
// sit next to the bundle forever, since nothing overwrites it.
await rm(OUT_DIR, { recursive: true, force: true });

/** @type {import("esbuild").BuildOptions} */
const options = {
	entryPoints: ["src/plugin.ts"],
	outfile: `${OUT_DIR}/plugin.js`,
	bundle: true,
	platform: "node",
	format: "esm",
	// Matches manifest.json's `Nodejs.Version`, which is the Node the Stream
	// Deck app supplies at runtime — not whatever built the plugin.
	target: "node20",
	sourcemap: true,
	banner: { js: REQUIRE_SHIM },
	logLevel: "info",
};

// `npm run watch`. esbuild does not type-check, so a watch session is a fast
// rebuild loop, not a substitute for `npm run build` or `npm run lint`.
if (process.argv.includes("--watch")) {
	const context = await esbuild.context(options);
	await context.watch();
} else {
	await esbuild.build(options);
}
