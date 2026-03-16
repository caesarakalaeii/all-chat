# Extension Settings Patterns

**Domain:** Browser extension popup and settings UI, color pickers, linking to full settings
**Researched:** 2026-03-14
**Overall confidence:** HIGH (Chrome Extension APIs are stable and well-documented; MV3 is the current standard)

## Sources

- Chrome Extensions Options Page guide: https://developer.chrome.com/docs/extensions/develop/ui/options-page
- `runtime.openOptionsPage()` MDN: https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/runtime/openOptionsPage
- Chrome Storage API: https://developer.chrome.com/docs/extensions/reference/api/storage
- Existing extension codebase: `/home/caesar/git/all-chat-extension/src/`

---

## Current Extension State

The current extension popup (`src/popup/popup.tsx`) is minimal:
- Toggle to enable/disable the extension
- Status display (supported platforms)
- Version display
- Link to GitHub

Current storage schema (`SyncStorage`):
```typescript
interface SyncStorage {
  apiGatewayUrl: string;
  extensionEnabled: boolean;
  preferences: {
    autoDetectEnabled: boolean;
    replaceNativeChat: boolean;
    fontSize: 'small' | 'medium' | 'large';
  };
}
```

For v1.4 viewer identity and cosmetics, the extension needs to:
1. Allow viewers to authenticate (OAuth login via Twitch/YouTube)
2. Show authentication status (who is logged in)
3. Allow viewer to set name color / gradient preferences
4. Link to the full All-Chat settings page (`allch.at/settings/viewer`) for advanced settings

---

## Extension Settings Architecture

### Storage split: `sync` vs `local`

The existing pattern correctly splits storage:
- `chrome.storage.sync`: User preferences (follow the user across devices). Max 100KB total, 8KB per item. Appropriate for: `extensionEnabled`, `preferences`, `apiGatewayUrl`.
- `chrome.storage.local`: Ephemeral / large data (not synced). Appropriate for: `viewer_jwt_token`, `viewer_info`, cosmetics cache.

For viewer cosmetics preferences (name color, gradient), these should go in `chrome.storage.sync` since they are user preferences that should follow the user:

```typescript
interface SyncStorage {
  apiGatewayUrl: string;
  extensionEnabled: boolean;
  preferences: {
    autoDetectEnabled: boolean;
    replaceNativeChat: boolean;
    fontSize: 'small' | 'medium' | 'large';
  };
  // New for v1.4:
  viewerCosmetics: {
    nameColor: string | null;          // "#RRGGBB" or null
    nameGradient: GradientConfig | null;
    avatarFrameId: string | null;
  };
}
```

However, cosmetics are better managed server-side (attached to the viewer account) so they persist even when the extension is reinstalled. `sync` storage is just a local cache of what the server has. On login, fetch from server and store locally. On change, update both server and local.

---

## Popup Settings UI Patterns

### Pattern 1: Minimal popup + full settings page link (recommended for All-Chat)

The popup stays minimal (status, quick toggles) and includes a clear "Open Settings" button that opens the full settings page:

```tsx
// In popup.tsx
<button
  onClick={() => {
    // Option A: Open All-Chat web settings page
    chrome.tabs.create({ url: 'https://allch.at/settings/viewer' });
    // Option B: Open local options page
    chrome.runtime.openOptionsPage();
  }}
  className="w-full px-3 py-2 bg-purple-600 hover:bg-purple-700 rounded text-sm"
>
  Open Settings
</button>
```

**Why this pattern:** Extension popups close the moment the user clicks outside. A full page (either `chrome.tabs.create` to the web app or `chrome.runtime.openOptionsPage()` for a local page) stays open. For settings that require OAuth and complex UI, the web app is better than a local options page.

**When to use `chrome.tabs.create` vs `openOptionsPage`:**
- `chrome.tabs.create({ url: 'https://allch.at/settings/viewer' })` — opens the main web app. Best when viewer settings are on the web app (they are in this case). Requires `tabs` permission (already in manifest).
- `chrome.runtime.openOptionsPage()` — opens the locally-bundled options HTML. Best for settings that should work offline or in dev. Requires `options_page` or `options_ui` in manifest.json.

For All-Chat, the web app is the authoritative settings location. Use `chrome.tabs.create`.

### Pattern 2: Inline color picker in popup

If a quick name-color picker is desired directly in the popup (without opening a new page), the native HTML `<input type="color">` is the correct approach:

```tsx
function ColorPickerSection({ value, onChange }: { value: string; onChange: (hex: string) => void }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm">Name Color</span>
      <div className="flex items-center gap-2">
        <input
          type="color"
          value={value || '#FFFFFF'}
          onChange={(e) => onChange(e.target.value)}
          className="w-8 h-8 rounded cursor-pointer border-0 p-0"
          title="Choose name color"
        />
        <span className="text-xs text-gray-400 font-mono">{value || 'default'}</span>
        {value && (
          <button
            onClick={() => onChange('')}
            className="text-xs text-gray-500 hover:text-white"
            title="Reset to default"
          >
            ×
          </button>
        )}
      </div>
    </div>
  );
}
```

**Notes on `<input type="color">`:**
- Opens the OS native color picker. No library needed.
- Returns hex strings (`#RRGGBB` format, 7 characters, lowercase).
- Firefox and Chrome both support it well. Safari supports it since Safari 12.
- Width/height must be set explicitly; default rendering is a small square.
- Does not support alpha channels (no `#RRGGBBAA`). For streaming use cases, solid colors are sufficient.
- The popup should be set to a minimum width (300px) for the color picker to display correctly; the default Chrome popup width is 300px.

### Pattern 3: Gradient builder (more complex — defer to web settings)

A gradient color picker (selecting 2-4 colors with angle control) is too complex for a popup. Defer this to the web settings page. In the popup, show a preview of the current gradient and a "Change" button that opens the web settings page to the gradient editor.

---

## Authentication Flow from Extension

The existing extension already supports OAuth (`START_AUTH` message type in `ExtensionMessage`). The v1.4 viewer identity flow extends this.

### Current flow:
1. Content script sends `{ type: 'START_AUTH', platform: 'twitch' }` to service worker
2. Service worker opens OAuth popup via `chrome.identity.launchWebAuthFlow` or `chrome.tabs.create` to the All-Chat OAuth endpoint
3. OAuth token returned and stored in `chrome.storage.local.viewer_jwt_token`

### Popup auth state display:

```tsx
function AuthSection({ viewerInfo, onLogin, onLogout }: AuthSectionProps) {
  if (!viewerInfo) {
    return (
      <div className="status">
        <div className="status-label">Account</div>
        <button onClick={onLogin} className="btn-primary text-sm w-full mt-1">
          Sign in with Twitch
        </button>
        <button onClick={() => loginWith('youtube')} className="btn-secondary text-sm w-full mt-1">
          Sign in with YouTube
        </button>
      </div>
    );
  }

  return (
    <div className="status">
      <div className="status-label">Signed in as</div>
      <div className="flex items-center gap-2 mt-1">
        {viewerInfo.avatar_url && (
          <img src={viewerInfo.avatar_url} className="w-6 h-6 rounded-full" alt="" />
        )}
        <span className="font-medium">{viewerInfo.display_name}</span>
        <span className="text-xs text-gray-500">({viewerInfo.platform})</span>
      </div>
      <button onClick={onLogout} className="text-xs text-gray-500 hover:text-white mt-1">
        Sign out
      </button>
    </div>
  );
}
```

---

## Settings Persistence Patterns

### Save on change (no explicit save button):

For toggles and simple settings, save immediately on change (no "Save" button needed):

```typescript
const handleColorChange = async (hex: string) => {
  setNameColor(hex);  // Optimistic update
  try {
    await setSyncStorage({ viewerCosmetics: { ...cosmetics, nameColor: hex } });
    // Optionally sync to server:
    await fetch('/api/viewer/cosmetics', {
      method: 'PATCH',
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ name_color: hex }),
    });
  } catch (err) {
    setNameColor(prevColor);  // Rollback
    showError('Failed to save color');
  }
};
```

### Storage change listener for live updates:

If the user changes settings in the web app, the extension should reflect changes without requiring a restart. Use `chrome.storage.onChanged`:

```typescript
// In service-worker.ts
chrome.storage.onChanged.addListener((changes, area) => {
  if (area === 'sync' && changes.viewerCosmetics) {
    // Notify content scripts to re-render with new cosmetics
    broadcastToAllTabs({ type: 'COSMETICS_UPDATED', data: changes.viewerCosmetics.newValue });
  }
});
```

---

## Extension Popup Size Constraints

Chrome extension popups have constraints to keep in mind:
- **Minimum width**: None (but 200px is practical minimum)
- **Maximum width**: 800px (Chrome enforces this)
- **Maximum height**: 600px (Chrome enforces this)
- **Default width**: Matches content width; `popup.html` body can set `min-width: 300px`
- The popup closes when focus leaves it — use `chrome.tabs.create` for anything needing persistent focus

For All-Chat popup, **300px width × 400-500px height** is the sweet spot:
- Shows auth status, enable/disable toggle, color preview, and "Open Settings" link
- Does not require scrolling

---

## "Link to Full Settings" UX Pattern

The best UX pattern (used by LastPass, 1Password, uBlock Origin) for extension popups:

1. Popup shows **status at a glance** (logged in? extension on/off? current color?)
2. A clearly labeled "Settings" or "Customize" button at the bottom opens the full experience
3. The full experience lives at `https://allch.at/settings/viewer` in the web app

```tsx
// Bottom section of popup
<div className="footer">
  <button
    onClick={() => chrome.tabs.create({ url: 'https://allch.at/settings/viewer' })}
    className="w-full text-sm text-purple-400 hover:text-purple-300 py-2"
  >
    Open Full Settings →
  </button>
  <div className="text-center text-xs text-gray-600">
    Version {chrome.runtime.getManifest().version}
  </div>
</div>
```

---

## Manifest.json Changes for v1.4

The current manifest.json is mostly correct. For v1.4 settings:

No new permissions needed for:
- Color picker (`<input type="color">`) — no permission needed
- Opening tabs to allch.at — `tabs` permission already present
- `chrome.storage.sync` for cosmetics — `storage` permission already present

If an options page is added (embedded in Chrome's extension settings), add to `manifest.json`:
```json
{
  "options_ui": {
    "page": "options/options.html",
    "open_in_tab": true
  }
}
```

`"open_in_tab": true` is recommended — Chrome's embedded options UI (shown in a panel on the extensions page) is confusing UX. Opening as a tab gives the full window and works better with OAuth redirects.

---

## Implementation Checklist for v1.4 Extension Work

- [ ] Add `viewerCosmetics` to `SyncStorage` interface
- [ ] Update `DEFAULT_SETTINGS` to include `viewerCosmetics: { nameColor: null, nameGradient: null, avatarFrameId: null }`
- [ ] Add color picker section to popup
- [ ] Add auth status section to popup (show who is logged in)
- [ ] Add "Open Settings" link to `https://allch.at/settings/viewer`
- [ ] Emit `COSMETICS_UPDATED` message from service worker when cosmetics change
- [ ] Content scripts apply `nameColor` to rendered username spans
- [ ] Handle server sync of cosmetics changes
