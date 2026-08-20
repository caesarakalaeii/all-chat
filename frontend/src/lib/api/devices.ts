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
 * Paired-device management API (ADR-0049).
 *
 * Wraps auth-service's /me/devices surface, proxied by the API gateway at
 * /api/v1/auth/me/devices. These are the credentials a *linked* Stream Deck or
 * StreamController plugin authenticates with (`allchat_dev_…`).
 *
 * THE ONE RULE, and it is the opposite of api-tokens.ts: **no function in this
 * module ever sees a device token.** A personal access token's plaintext is
 * shown to the user exactly once by `createApiToken`; a device token's plaintext
 * goes from auth-service's exchange endpoint straight to the plugin over the
 * loopback redirect and never enters the browser at all. That is the whole point
 * of the flow — a secret that is never rendered cannot be read aloud,
 * screenshotted or leaked on camera, which is the failure mode ADR-0049 rejected
 * the pasted-token install for.
 *
 * So there is no `CreatedDevice` type here, no `token` field, and no reveal
 * component on the pages that use this. If one appears, that is the bug.
 *
 * The PAT surface is not deprecated by any of this: ADR-0049's loopback flow
 * cannot reach a headless capture box, a second PC or a CLI, which is exactly
 * what a pasted token is for. See api-tokens.ts.
 */

import { apiClient } from './client'

/**
 * The closed set of grantable scopes, mirroring `allowedAPITokenScopes` in
 * services/auth-service/handlers/api_tokens.go — the same allowlist PATs use,
 * because a device is not a different kind of caller, only a differently
 * delivered credential.
 */
export const DEVICE_SCOPES = ['chat:write', 'engagement:write'] as const

export type DeviceScope = (typeof DEVICE_SCOPES)[number]

/** The two delivery paths. Both mint the identical credential. */
export type LinkFlow = 'loopback' | 'code'

/**
 * Device METADATA — what the list endpoint returns. There is deliberately no
 * field for the plaintext or its digest: the server's projection never selects
 * `token_hash`, and unlike the PAT surface there is not even a create response
 * that could carry a secret.
 */
export interface PairedDevice {
  id: string
  name: string
  overlay_id: string
  overlay_name: string
  scopes: string[]
  created_at: string
  last_used_at: string | null
  /** Mandatory for a device token, unlike a PAT: the sliding 90-day window. */
  expires_at: string
  revoked_at: string | null
}

/**
 * A pending link request, as the approve screen renders it before the streamer
 * decides.
 *
 * `device_name_self_reported` is named that way on purpose, on both sides of the
 * wire: the value comes from the plugin, so it is a claim about what is asking,
 * not a fact. The approve screen labels it as such.
 */
export interface PendingLink {
  request_id: string
  flow: LinkFlow
  device_name_self_reported: string
  requested_scopes: string[]
  expires_at: string
}

/** What the server returns after Approve. Contains no credential. */
export interface ApprovedLink {
  request_id: string
  flow: LinkFlow
  device_name: string
  overlay_id: string
  scopes: string[]
  /**
   * Where to send the browser next, for the loopback flow only: the server-side
   * callback that validates the stored redirect again and emits the Location
   * header carrying the one-time code. Empty for the code flow, where the plugin
   * is polling and there is nothing for the browser to do.
   */
  redirect_to?: string
}

const BASE = '/api/v1/auth/me/devices'

/** Lists the signed-in user's paired devices (metadata only, newest first). */
export async function listDevices(): Promise<PairedDevice[]> {
  const response = await apiClient.get<{ devices: PairedDevice[] | null }>(BASE)
  return response.devices ?? []
}

/**
 * Loads the pending link request the approve screen is about to show.
 *
 * Identified by `request_id` (the loopback flow, where the plugin opened the
 * browser with it in the URL) or by `user_code` (the fallback, where the
 * streamer typed the code on another machine).
 */
export async function getPendingLink(params: {
  requestId?: string
  userCode?: string
}): Promise<PendingLink> {
  const query = new URLSearchParams()
  if (params.requestId) query.set('request_id', params.requestId)
  if (params.userCode) query.set('user_code', params.userCode)
  return apiClient.get<PendingLink>(`${BASE}/pending?${query.toString()}`)
}

/**
 * Approves a pending link, binding it to one overlay and a granted scope set.
 *
 * The granted scopes may be narrower than the plugin requested but never wider —
 * the server refuses a scope that was not requested, so a bug here cannot hand a
 * device more than it asked for.
 */
export async function approveDevice(body: {
  requestId?: string
  userCode?: string
  overlayId: string
  scopes: readonly string[]
  deviceName?: string
}): Promise<ApprovedLink> {
  return apiClient.post<ApprovedLink>(`${BASE}/approve`, {
    request_id: body.requestId,
    user_code: body.userCode,
    overlay_id: body.overlayId,
    scopes: body.scopes,
    device_name: body.deviceName,
  })
}

/**
 * Denies a pending link. Deny and Approve both terminate the request row, so the
 * plugin stops polling instead of waiting out the TTL with no idea it was
 * refused.
 *
 * Same endpoint as Approve, because both outcomes are the same state transition
 * on the same row — and neither an overlay nor a scope set means anything when
 * the answer is no, so neither is sent.
 */
export async function denyDevice(body: { requestId?: string; userCode?: string }): Promise<void> {
  await apiClient.post(`${BASE}/approve`, {
    request_id: body.requestId,
    user_code: body.userCode,
    deny: true,
  })
}

/**
 * Revokes a paired device. Revocation is read live by the server's resolver, so
 * it takes effect on the next request the plugin makes.
 */
export async function revokeDevice(id: string): Promise<void> {
  await apiClient.delete(`${BASE}/${encodeURIComponent(id)}`)
}
