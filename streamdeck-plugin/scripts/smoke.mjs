/**
 * Starts the built plugin the way the Stream Deck app does and asserts it
 * registers.
 *
 * This is the only check in the pipeline that runs the artifact. `npm run lint`,
 * `npm run build`, `@elgato/cli validate` and `@elgato/cli pack` all pass
 * happily on a plugin that cannot start — a missing runtime dependency is a
 * launch-time failure and none of them launch anything. That is exactly how a
 * `.sdPlugin` whose every `import` was unresolvable off the dev machine got as
 * far as being packaged for a tester.
 *
 * It deliberately runs against the *copied* plugin folder, isolated from this
 * repo's `node_modules`. Run in place, Node's upward resolution walks out of
 * `com.allchat.streamdeck.sdPlugin/bin/` into `streamdeck-plugin/node_modules/`
 * and finds the SDK there, so an unbundled plugin passes — which is precisely
 * the false negative that hid the bug.
 *
 * Protocol: the app spawns the plugin with `-port/-pluginUUID/-registerEvent/
 * -info` and the plugin opens a WebSocket back to that port and sends
 * `{"event": <registerEvent>, "uuid": <pluginUUID>}`.
 *
 * Usage: `node scripts/smoke.mjs [path/to/some.sdPlugin]`. With no argument it
 * tests the freshly built folder; pass a path to test a `.streamDeckPlugin`
 * that has been unzipped, i.e. exactly the bytes a tester installs.
 */
import { spawn } from "node:child_process";
import { cp, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";
import { WebSocketServer } from "ws";

const PLUGIN_DIR = process.argv[2] ?? "com.allchat.streamdeck.sdPlugin";
const REGISTER_EVENT = "registerPlugin";
const PLUGIN_UUID = "smoke-test-uuid";
const TIMEOUT_MS = 15_000;

/** Mimics the `info` payload the Stream Deck app passes; the SDK parses it. */
const INFO = JSON.stringify({
	application: {
		font: "Arial",
		language: "en",
		platform: "windows",
		platformVersion: "10.0.0",
		version: "6.9.0",
	},
	plugin: { uuid: "com.allchat.streamdeck", version: "0.1.0.0" },
	devicePixelRatio: 1,
	colors: {},
	devices: [],
});

const stagingRoot = await mkdtemp(join(tmpdir(), "allchat-sd-smoke-"));
const stagedPlugin = join(stagingRoot, basename(PLUGIN_DIR));
await cp(PLUGIN_DIR, stagedPlugin, { recursive: true });

const server = new WebSocketServer({ host: "127.0.0.1", port: 0 });
await new Promise((resolve) => server.once("listening", resolve));
const { port } = server.address();

const registration = new Promise((resolve, reject) => {
	const timer = setTimeout(
		() => reject(new Error(`plugin did not register within ${TIMEOUT_MS}ms`)),
		TIMEOUT_MS,
	);
	server.once("connection", (socket) => {
		socket.once("message", (raw) => {
			clearTimeout(timer);
			try {
				resolve(JSON.parse(raw.toString()));
			} catch (error) {
				reject(new Error(`registration payload was not JSON: ${error.message}`));
			}
		});
	});
});

const child = spawn(
	process.execPath,
	[
		join(stagedPlugin, "bin", "plugin.js"),
		"-port", String(port),
		"-pluginUUID", PLUGIN_UUID,
		"-registerEvent", REGISTER_EVENT,
		"-info", INFO,
	],
	{ cwd: stagedPlugin, stdio: ["ignore", "pipe", "pipe"] },
);

let output = "";
child.stdout.on("data", (chunk) => { output += chunk; });
child.stderr.on("data", (chunk) => { output += chunk; });

const exited = new Promise((resolve) => child.once("exit", resolve));
const crashed = exited.then((code) => {
	throw new Error(`plugin exited before registering (code ${code})\n${output.trim()}`);
});

try {
	const message = await Promise.race([registration, crashed]);
	if (message.event !== REGISTER_EVENT || message.uuid !== PLUGIN_UUID) {
		throw new Error(`unexpected registration payload: ${JSON.stringify(message)}`);
	}
	console.log(`smoke: ${PLUGIN_DIR} registered as ${message.uuid} (${message.event})`);
} catch (error) {
	console.error(`smoke: FAILED — ${error.message}`);
	process.exitCode = 1;
} finally {
	crashed.catch(() => {});
	child.kill();
	server.close();
	await rm(stagingRoot, { recursive: true, force: true });
}
