'use strict';

// --- Configuration ---
const BACKEND_URL = 'https://chat.allchat.live';
// For local dev: change to 'http://localhost:8080'

// --- Utility: decode JWT without verification ---
function decodeJWT(token) {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    if (payload.exp * 1000 < Date.now()) return null; // expired
    return payload;
  } catch { return null; }
}

// --- Storage helpers ---
async function getLocalStorage(keys) {
  return chrome.storage.local.get(keys);
}
async function setLocalStorage(data) {
  return chrome.storage.local.set(data);
}

// --- Auth state ---
async function getViewerInfo() {
  const { viewer_jwt } = await getLocalStorage('viewer_jwt');
  if (!viewer_jwt) return null;
  return decodeJWT(viewer_jwt);
}

// --- OAuth sign-in ---
async function signInWithPlatform(platform) {
  try {
    // 1. Get auth URL from backend
    const loginResp = await fetch(`${BACKEND_URL}/api/v1/auth/viewer/${platform}/login`, {
      method: 'GET',
    });
    if (!loginResp.ok) throw new Error(`Login endpoint failed: ${loginResp.status}`);
    const { auth_url } = await loginResp.json();

    // 2. Replace backend redirect_uri with extension redirect URI
    const extensionRedirectURI = chrome.identity.getRedirectURL('oauth');
    const url = new URL(auth_url);
    url.searchParams.set('redirect_uri', extensionRedirectURI);

    // 3. Launch Chrome OAuth popup
    const responseUrl = await new Promise((resolve, reject) => {
      chrome.identity.launchWebAuthFlow(
        { url: url.toString(), interactive: true },
        (callbackUrl) => {
          if (chrome.runtime.lastError) {
            reject(new Error(chrome.runtime.lastError.message));
          } else {
            resolve(callbackUrl);
          }
        }
      );
    });

    // 4. Extract code + state from the callback URL
    const callbackParams = new URL(responseUrl).searchParams;
    const code = callbackParams.get('code');
    const state = callbackParams.get('state');
    if (!code || !state) throw new Error('Missing code or state in callback');

    // 5. POST exchange to get JWT
    const exchangeResp = await fetch(`${BACKEND_URL}/api/v1/auth/viewer/${platform}/exchange`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code, state }),
    });
    if (!exchangeResp.ok) throw new Error(`Exchange failed: ${exchangeResp.status}`);
    const { token } = await exchangeResp.json();

    // 6. Store JWT
    await setLocalStorage({ viewer_jwt: token });
    renderPopup(); // Re-render in signed-in state
  } catch (err) {
    console.error('[All-Chat] Sign-in error:', err);
    renderError(err.message);
  }
}

// --- Sign out ---
async function signOut() {
  await chrome.storage.local.remove(['viewer_jwt', 'name_color']);
  renderPopup();
}

// --- Color save ---
let colorSaveTimeout = null;
async function saveColor(color) {
  const { viewer_jwt } = await getLocalStorage('viewer_jwt');
  if (!viewer_jwt) return;

  // Optimistic local save
  await setLocalStorage({ name_color: color });

  // Debounce server save (300ms)
  clearTimeout(colorSaveTimeout);
  colorSaveTimeout = setTimeout(async () => {
    try {
      const resp = await fetch(`${BACKEND_URL}/api/v1/auth/viewer/cosmetics`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${viewer_jwt}`,
        },
        body: JSON.stringify({ name_color: color }),
      });
      if (resp.ok) {
        showSaveIndicator();
      }
    } catch (err) {
      console.error('[All-Chat] Color save error:', err);
    }
  }, 300);
}

async function resetColor() {
  const { viewer_jwt } = await getLocalStorage('viewer_jwt');
  await chrome.storage.local.remove('name_color');
  if (viewer_jwt) {
    fetch(`${BACKEND_URL}/api/v1/auth/viewer/cosmetics`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${viewer_jwt}`,
      },
      body: JSON.stringify({ name_color: null }),
    });
  }
  document.getElementById('color-picker').value = '#ffffff';
  showSaveIndicator();
}

function showSaveIndicator() {
  const el = document.getElementById('save-indicator');
  if (!el) return;
  el.textContent = 'Saved';
  el.style.opacity = '1';
  setTimeout(() => { el.style.opacity = '0'; }, 1500);
}

// --- Open Settings ---
function openSettings() {
  chrome.tabs.create({ url: `${BACKEND_URL}/settings/viewer` });
}

// --- Render error state ---
function renderError(message) {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="error-state">
      <p>Sign-in failed</p>
      <p class="error-detail">${message}</p>
      <button id="btn-retry">Try again</button>
    </div>
  `;
  document.getElementById('btn-retry').addEventListener('click', renderPopup);
}

// --- Main render ---
async function renderPopup() {
  const app = document.getElementById('app');
  const viewerInfo = await getViewerInfo();
  const { current_platform } = await chrome.storage.session.get('current_platform');
  const { name_color } = await getLocalStorage('name_color');

  if (viewerInfo) {
    // --- SIGNED-IN STATE ---
    const avatarUrl = viewerInfo.avatar_url || '';
    const displayName = viewerInfo.display_name || viewerInfo.username || 'Viewer';
    const currentColor = name_color || '#ffffff';

    app.innerHTML = `
      <div class="signed-in">
        <div class="user-row">
          ${avatarUrl ? `<img class="avatar" src="${avatarUrl}" alt="">` : '<div class="avatar-placeholder"></div>'}
          <span class="display-name">${escapeHtml(displayName)}</span>
        </div>
        <div class="color-row">
          <label for="color-picker">Name Color</label>
          <div class="color-controls">
            <input type="color" id="color-picker" value="${currentColor}">
            <button id="btn-reset-color" title="Reset to default">&#x21BA;</button>
            <span id="save-indicator" class="save-indicator"></span>
          </div>
        </div>
        <div class="action-row">
          <button id="btn-settings" class="btn-settings">Open Settings</button>
          <button id="btn-signout" class="btn-signout">Sign Out</button>
        </div>
      </div>
    `;

    document.getElementById('color-picker').addEventListener('input', (e) => {
      saveColor(e.target.value);
    });
    document.getElementById('btn-reset-color').addEventListener('click', resetColor);
    document.getElementById('btn-settings').addEventListener('click', openSettings);
    document.getElementById('btn-signout').addEventListener('click', signOut);

  } else {
    // --- SIGNED-OUT STATE ---
    // Context-aware: show only the current platform's button, or all three
    const platforms = current_platform ? [current_platform] : ['twitch', 'youtube', 'kick'];
    const buttons = platforms.map(p => `
      <button class="btn-signin btn-${p}" data-platform="${p}">
        Sign in with ${capitalize(p)}
      </button>
    `).join('');

    app.innerHTML = `
      <div class="signed-out">
        <p class="tagline">Personalize your chat identity</p>
        ${buttons}
      </div>
    `;

    document.querySelectorAll('.btn-signin').forEach(btn => {
      btn.addEventListener('click', () => signInWithPlatform(btn.dataset.platform));
    });
  }
}

// --- Helpers ---
function escapeHtml(str) {
  return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

// --- Boot ---
document.addEventListener('DOMContentLoaded', renderPopup);
