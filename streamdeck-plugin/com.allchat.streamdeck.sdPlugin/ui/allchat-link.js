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
		How long to wait for a terminal message before releasing the button.

		The plugin reports "linked" or "failed" on every path it can reach, but if it
		never reports at all — the flow throws somewhere outside the reporting block, the
		plugin process dies, the message is dropped because the inspector was
		re-rendered — the old UI sat on "Starting…" with a dead button and no way back
		except reopening the key's settings. That is exactly what the bug report
		described, so the recovery is built in rather than assumed away.

		11 minutes is just past the longest flow: the pairing-code path gives the user
		600s (CODE_FLOW_TIMEOUT_MS in src/allchat/linking.ts), the loopback path 180s.
		Anything still silent past that is not slow, it is gone.
	*/
	const WATCHDOG_MS = 660_000;

	/** Statuses after which the flow is over and the button is usable again. */
	const TERMINAL = new Set(["linked", "failed"]);

	let watchdog;

	const clearWatchdog = () => {
		if (watchdog !== undefined) {
			clearTimeout(watchdog);
			watchdog = undefined;
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
		button.disabled = false;
		render(state, detail);
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

	SDPIComponents.streamDeckClient
		.getConnectionInfo()
		.then((connection) => {
			const version = connection?.info?.plugin?.version;
			if (version && versionSlot) {
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

		clearWatchdog();
		watchdog = setTimeout(() => {
			finish(
				"failed",
				"Linking timed out with no answer from the plugin. Press Link to try again, " +
					"or paste a personal access token instead.",
			);
		}, WATCHDOG_MS);

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

		const state = payload.status;

		if (state === "code" && payload.userCode) {
			// Not terminal: the plugin is still polling for the approval, so the button
			// stays disabled and the watchdog keeps running.
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
