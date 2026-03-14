'use strict';

// --- Platform detection ---
// Runs on twitch.tv, youtube.com, kick.com pages only (not overlay pages).
// Overlay pages are also matched in manifest content_scripts to allow applyViewerColor
// to run, but platform detection is only set when on an actual streaming platform.
(function detectPlatform() {
  const hostname = window.location.hostname;
  let platform = null;
  if (hostname.includes('twitch.tv')) platform = 'twitch';
  else if (hostname.includes('youtube.com')) platform = 'youtube';
  else if (hostname.includes('kick.com')) platform = 'kick';

  if (platform) {
    chrome.storage.session.set({ current_platform: platform });
  }
})();

// --- EXT-04: Apply viewer's name_color to own messages in overlay ---
// The overlay is served at /overlay/:id on chat.allchat.live (and localhost:3000 in dev).
// The manifest content_scripts.matches includes these URLs so this function runs there.
// It reads viewer_jwt to get the viewer's username, then applies name_color to any
// message container elements that have data-username matching the viewer's username.
// The frontend overlay component must set data-username={message.user.username} on the
// message container div — this is handled in plan 04 Task 2.
(async function applyViewerColor() {
  const { viewer_jwt, name_color } = await chrome.storage.local.get(['viewer_jwt', 'name_color']);
  if (!viewer_jwt || !name_color) return;

  let viewerUsername = null;
  try {
    const payload = JSON.parse(atob(viewer_jwt.split('.')[1]));
    viewerUsername = payload.display_name || payload.username;
  } catch { return; }

  if (!viewerUsername) return;

  function colorOwnMessages() {
    const selector = `[data-username="${CSS.escape(viewerUsername)}"] .message-author`;
    document.querySelectorAll(selector).forEach(el => {
      el.style.color = name_color;
    });
  }

  // Run immediately and observe DOM mutations for dynamically added messages
  colorOwnMessages();
  const observer = new MutationObserver(colorOwnMessages);
  observer.observe(document.body, { childList: true, subtree: true });
})();
