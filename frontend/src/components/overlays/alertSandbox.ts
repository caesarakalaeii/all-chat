/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * The premium custom-code alert sandbox (issue #954, ADR-0071).
 *
 * A profile with custom HTML/CSS/JS renders inside
 * `<iframe sandbox="allow-scripts" srcdoc=...>`. The sandbox is the ONLY
 * isolation layer for streamer-written JS (ADR-0068 accepts this risk), so its
 * token list is a security contract:
 *
 *   - `allow-scripts` and nothing else. Without the same-origin token the frame
 *     runs in a unique opaque origin: no access to the overlay page, its
 *     storage or cookies, and no credentialed same-origin requests.
 *   - never the same-origin token (with allow-scripts it lets the frame remove
 *     its own sandbox), `allow-forms`, `allow-top-navigation*` or
 *     `allow-popups*`. A unit test asserts the attribute string exactly.
 *
 * The srcdoc inherits the overlay page's CSP (next.config.js): inline scripts
 * run under its `script-src 'unsafe-inline'`, external scripts only from the
 * hosts that policy lists, and `eval`/`new Function` are blocked.
 *
 * Message contract (parent → frame, once per alert, after the frame loads):
 *
 *   { type: "alert", profile: AlertProfile, event: AlertEvent }
 *
 * `profile` is the gateway-resolved profile (its `text` is the rendered
 * template); `event` is the alert envelope without `profile` (alert_id,
 * platform, event_type, event_data, user, message, occurred_at).
 *
 * Runtime API injected before the streamer's code (the bootstrap below):
 *
 *   onAlert(callback)        call callback(message) for each alert; a handler
 *                            registered late still gets the current alert
 *   allchat.fill(tpl, event) substitute {username} {amount} {count} {message}
 *                            like the gateway does (shared/alerts RenderText):
 *                            a placeholder whose field is missing stays literal
 *
 * Built-in bindings: elements with `data-alert-text` get the rendered text,
 * `img[data-alert-image]` gets the profile image (hidden without one). With
 * no custom HTML the frame uses a default layout made of exactly those two, so
 * custom CSS alone can restyle the alert. The parent still owns timing (the
 * profile duration) and the profile sound.
 */

import type { AlertFrame, AlertProfile, EditableAlertProfile } from '@/lib/types/alert'

/** The complete sandbox token list. Do not add to it: see the header. */
export const ALERT_SANDBOX = 'allow-scripts'

export type AlertEvent = Omit<AlertFrame, 'profile'>

export interface AlertBridgeMessage {
  type: 'alert'
  profile: AlertProfile
  event: AlertEvent
}

export function hasCustomCode(profile: EditableAlertProfile | undefined): boolean {
  return Boolean(
    profile?.custom_html?.trim() || profile?.custom_css?.trim() || profile?.custom_js?.trim()
  )
}

/** The postMessage payload for one alert (only called for alerts with a profile). */
export function alertBridgeMessage(
  alert: AlertFrame & { profile: AlertProfile }
): AlertBridgeMessage {
  const { profile, ...event } = alert
  return { type: 'alert', profile, event }
}

export const ALERT_SANDBOX_DEFAULT_HTML =
  '<div class="alert"><img data-alert-image alt=""><p data-alert-text></p></div>'

/** Plain ES5 on purpose: it runs as an inline script in whatever OBS's browser is. */
export const ALERT_SANDBOX_BOOTSTRAP = `(function () {
  var handlers = [];
  var current = null;
  function fill(template, event) {
    var pairs = [];
    var value = event && event.event_data && event.event_data.value;
    if (event && event.user && event.user.name) pairs.push(['{username}', event.user.name]);
    if (value && value.display_text) pairs.push(['{amount}', value.display_text]);
    if (value) pairs.push(['{count}', String(value.amount)]);
    if (event && event.message) pairs.push(['{message}', event.message]);
    var out = String(template);
    for (var i = 0; i < pairs.length; i++) out = out.split(pairs[i][0]).join(pairs[i][1]);
    return out;
  }
  function bind(msg) {
    var texts = document.querySelectorAll('[data-alert-text]');
    for (var i = 0; i < texts.length; i++) texts[i].textContent = msg.profile.text || '';
    var images = document.querySelectorAll('img[data-alert-image]');
    for (var j = 0; j < images.length; j++) {
      if (msg.profile.image_url) images[j].src = msg.profile.image_url;
      else images[j].hidden = true;
    }
  }
  function call(handler, msg) {
    try { handler(msg); } catch (err) { console.error('[alert] onAlert handler failed', err); }
  }
  function onAlert(handler) {
    handlers.push(handler);
    if (current) call(handler, current);
  }
  window.allchat = { onAlert: onAlert, fill: fill };
  window.onAlert = onAlert;
  window.addEventListener('message', function (e) {
    if (e.source !== window.parent || !e.data || e.data.type !== 'alert') return;
    current = e.data;
    bind(current);
    for (var i = 0; i < handlers.length; i++) call(handlers[i], current);
  });
})();`

// The streamer's text sits inside <style>/<script>: an early closing tag would
// end the element mid-code. `<\/` means the same thing inside both.
const escapeClose = (code: string, tag: string) =>
  code.replace(new RegExp(`</(${tag})`, 'gi'), '<\\/$1')

/** The iframe document for a profile with custom code. */
export function buildAlertSrcdoc(profile: EditableAlertProfile): string {
  const html = profile.custom_html?.trim() ? profile.custom_html : ALERT_SANDBOX_DEFAULT_HTML
  return [
    '<!doctype html><html><head><meta charset="utf-8">',
    '<style>html,body{margin:0;background:transparent;color-scheme:normal;overflow:hidden}</style>',
    `<style>${escapeClose(profile.custom_css ?? '', 'style')}</style>`,
    `<script>${ALERT_SANDBOX_BOOTSTRAP}</script>`,
    `</head><body>${html}`,
    `<script>${escapeClose(profile.custom_js ?? '', 'script')}</script>`,
    '</body></html>',
  ].join('')
}
