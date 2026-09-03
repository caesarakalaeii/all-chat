/*
	The "Link with All-Chat" button, shared by all three property inspectors.

	This script's only job is to ask the plugin to run the device-link flow (ADR-0049)
	and to render what comes back. It never sees a credential: the plugin writes the
	device token straight into the key's settings, so there is nothing here to display,
	copy or leak.

	It lives in one file rather than inline in each page for the reason ADR-0049 gives
	for the parity gate: three copies of the same logic diverge. It was already three
	copies, and the button-state handling below would have made it three copies of
	something worth getting right once.

	Loaded at the end of <body>, after the sdpi-components script in <head>, so the
	SDPIComponents global is available at parse time.
*/
(() => {
	const button = document.getElementById("allchat-link");
	const status = document.getElementById("allchat-link-status");
	if (!button || !status) {
		return;
	}

	/*
		Where a streamer gets a current build. There is no auto-update: the plugin ships
		as a .streamDeckPlugin file that has to be downloaded and double-clicked, so a
		build installed before a fix landed stays installed until somebody tells its
		owner to replace it. See "Installing and updating" in streamdeck-plugin/README.md.
	*/
	const INSTALL_PAGE_URL = "https://github.com/caesarakalaeii/all-chat/releases";

	/*
		How long to wait for a terminal message before releasing the button.

		The plugin reports "linked" or "failed" on every path it can reach, but if it
		never reports at all — the flow throws somewhere outside the reporting block, the
		plugin process dies, the message is dropped because the inspector was
		re-rendered — the old UI sat on "Starting…" with a dead button and no way back
		except reopening the key's settings. That is exactly what the bug report
		described, so the recovery is built in rather than assumed away.

		Two of them, because the two flows have honest durations an order of magnitude
		apart and one number served neither:

		  * WATCHDOG_MS covers the loopback path, bounded by LOOPBACK_TIMEOUT_MS (180s in
		    src/allchat/linking.ts). 210s leaves the flow its full budget plus a margin
		    for the plugin's own reporting.
		  * CODE_WATCHDOG_MS covers the pairing code, which really is valid for
		    CODE_FLOW_TIMEOUT_MS (600s) while the user types it on another device.
		    Cutting that off early would break a flow that was working.

		It was 660s for BOTH, so a loopback attempt that died silently held the panel for
		eleven minutes. scripts/check-plugin-parity.py asserts each of these outlasts the
		flow it waits on, reading the flow timeouts out of linking.ts.
	*/
	const WATCHDOG_MS = 210_000;
	const CODE_WATCHDOG_MS = 660_000;

	/*
		How long a total silence may be presented as progress.

		Distinct from the watchdogs above, which bound a flow that IS running. This one
		bounds the case where the plugin never answers at all — the shape of issue #816,
		where a pre-#797 property inspector threw at parse time and could never render a
		status, so "Starting…" was the last thing it ever said. Neither watchdog helps
		there: they are both far longer than anyone waits, so the streamer leaves before
		the panel admits anything is wrong.

		15s because the plugin's first report arrives within a couple of seconds on every
		path it has (loopback: as soon as the browser opens; code: as soon as the server
		answers, itself bounded by REQUEST_TIMEOUT_MS = 15s). Silence past that is not
		slowness.
	*/
	const SILENCE_MS = 15_000;

	/** Statuses after which the flow is over and the button is usable again. */
	const TERMINAL = new Set(["linked", "failed"]);

	let watchdog;
	let silenceTimer;

	const clearWatchdog = () => {
		if (watchdog !== undefined) {
			clearTimeout(watchdog);
			watchdog = undefined;
		}
	};

	const clearSilenceTimer = () => {
		if (silenceTimer !== undefined) {
			clearTimeout(silenceTimer);
			silenceTimer = undefined;
		}
	};

	/**
	 * Renders a state.
	 *
	 * `data-state` drives the colour and the ⚠ / ✓ marker from allchat-pi.css. The
	 * pairing code is appended as an element rather than interpolated into a string so
	 * it can be given its own line and monospace face, and it is set with textContent
	 * rather than innerHTML: it comes off the wire, and nothing off the wire is parsed
	 * as markup here.
	 */
	const render = (state, detail, userCode) => {
		status.dataset.state = state;
		status.textContent = detail ?? "";
		if (userCode) {
			const code = document.createElement("span");
			code.className = "allchat-code";
			code.textContent = userCode;
			status.appendChild(code);
		}
	};

	const finish = (state, detail) => {
		clearWatchdog();
		clearSilenceTimer();
		button.disabled = false;
		render(state, detail);
	};

	/**
	 * Arms the give-up timer, replacing any previous one.
	 *
	 * Rearming rather than adding a second timer is what lets the pairing-code path
	 * extend its own deadline: the message that shows the code is also the message
	 * that says "this will legitimately take minutes now".
	 */
	const armWatchdog = (timeoutMs) => {
		clearWatchdog();
		watchdog = setTimeout(() => {
			finish(
				"failed",
				"Linking timed out with no answer from the plugin. Press Link to try again, " +
					"or paste a personal access token instead.",
			);
		}, timeoutMs);
	};

	/*
		The running plugin version, shown next to the Link button.

		This is the whole of issue #816 in one line. The #797 client fix was in the repo
		and in CI artifacts for days while every streamer who had already installed the
		plugin kept running the broken build, and neither they nor anyone reading their
		bug report could tell — the panel said nothing about which build it was. It is
		read from the registration info Stream Deck hands the property inspector, which
		carries manifest.json's `Version` for the plugin that is ACTUALLY loaded. Parsing
		manifest.json here would report what is on disk, which is the same thing right up
		to the moment it is not.

		Failure is silent past a console line: an unknown version is a worse panel, not a
		broken one, and refusing to render the Link button because a cosmetic label could
		not be filled in would be the wrong trade.
	*/
	const versionSlot = document.getElementById("allchat-plugin-version");

	/*
		The same version, for the silence warning below. Held separately because that
		warning is the one message where the build number is not decoration: it is the
		answer to "why is nothing happening", and reading it back out of the DOM element
		would couple the copy to the element's own formatting.
	*/
	let pluginVersion = "unknown";

	SDPIComponents.streamDeckClient
		.getConnectionInfo()
		.then((connection) => {
			const version = connection?.info?.plugin?.version;
			if (!version) {
				return;
			}
			pluginVersion = version;
			if (versionSlot) {
				versionSlot.textContent = `Plugin version ${version}`;
			}
		})
		.catch((error) => {
			console.error("All-Chat: could not read the plugin version", error);
		});

	button.addEventListener("click", () => {
		// Guard as well as disable: a queued click on an already-disabled button would
		// otherwise start a second concurrent flow, and two flows racing to write the
		// same key's settings means the surviving credential is whichever finished last.
		if (button.disabled) {
			return;
		}
		button.disabled = true;
		render("working", "Starting…");

		armWatchdog(WATCHDOG_MS);

		clearSilenceTimer();
		silenceTimer = setTimeout(() => {
			// Deliberately NOT terminal, and deliberately not "failed": the flow may still
			// be alive and about to write a credential, and a second concurrent flow
			// racing it is worse than a slow one. So the button stays disabled and the
			// watchdog keeps running — this only replaces a claim of progress with what is
			// actually known, which is that the plugin has said nothing at all, plus the
			// most likely reason for that: a build too old to answer (issue #816).
			silenceTimer = undefined;
			render(
				"stale",
				`No answer from the plugin after ${SILENCE_MS / 1000} seconds. This build ` +
					`(version ${pluginVersion}) may be too old to report back; there is no ` +
					`auto-update, so install the current one from ${INSTALL_PAGE_URL}. Still ` +
					`waiting, in case it answers.`,
			);
		}, SILENCE_MS);

		SDPIComponents.streamDeckClient.send("sendToPlugin", { event: "link" });
	});

	/*
		`sendToPropertyInspector` is the observable that carries messages from the plugin.

		The pages used to subscribe to `didReceivePropertyInspectorMessage`, which does not
		exist on sdpi-components' client — its members are `message`,
		`didReceiveSettings`, `didReceiveGlobalSettings` and `sendToPropertyInspector`.
		Reading `.subscribe` off `undefined` threw a TypeError at parse time, and because
		the click listener was registered on the line ABOVE it, the button still worked
		while no status update could ever arrive. That is the second half of the reported
		bug: pressing Link set "Starting…" and nothing was ever able to replace it, so the
		panel sat there for good even once the server stopped answering 500.

		The observable dispatches the whole Stream Deck message, so the plugin's own
		payload is one level down in `message.payload`.
	*/
	SDPIComponents.streamDeckClient.sendToPropertyInspector.subscribe((message) => {
		const payload = message?.payload;
		if (!payload || payload.event !== "link-status") {
			return;
		}

		// A message of ANY kind proves the plugin is answering, which is the only thing
		// the silence timer was watching for. Cleared before the status is inspected, so
		// a status this script does not recognise still counts as an answer.
		clearSilenceTimer();

		const state = payload.status;

		if (state === "code" && payload.userCode) {
			// Not terminal: the plugin is still polling for the approval, so the button
			// stays disabled — but the deadline moves out to the 600s the code is genuinely
			// valid for, which is longer than the loopback budget armed on click.
			armWatchdog(CODE_WATCHDOG_MS);
			render("code", payload.detail ?? "Enter this code at allch.at/link:", payload.userCode);
			return;
		}

		if (TERMINAL.has(state)) {
			finish(state, payload.detail ?? state);
			return;
		}

		render("working", payload.detail ?? state ?? "");
	});
})();
