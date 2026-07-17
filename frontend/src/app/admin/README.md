# All-Chat Admin Dashboard

Operator console for managing users, overlays, chat sources, and viewers across
the platform. All pages are gated behind `ProtectedRoute requireAdmin` (see
`layout.tsx`) and talk to real backend admin APIs over the session cookie (the
gateway's CookieToBearer middleware turns the httpOnly access cookie into a
Bearer token; there is no JS-readable token).

## Navigation model (ADR-0036)

Admin pages are **URL-addressable**: selection and filters live in query
parameters, not opaque React state, so every view is deep-linkable and entities
cross-link to one another. Layout is master-detail (a scrollable list plus a
sticky detail panel).

Recognised parameters:

- `/admin/users?user=<id>` — auto-select a user; `?filter=active|banned|premium|beta` — preselect a tab
- `/admin/overlays?overlay=<id>` — auto-select an overlay; `?connected=true` — connected-only view
- `/admin/sources?user=<id>` — scope to one owner; `?platform=<p>` — preselect a platform
- `/admin/viewers?q=<text>` — pre-fill the viewer search
- `/admin/search?q=<text>` — global search

The sidebar (`components/AdminSidebar.tsx`) is the single source of truth for the
nav link list (`ADMIN_LINKS`); the dashboard home grid is generated from it so
the two can never drift.

## Pages

### Dashboard home (`/admin`)
Platform stats (users, banned, active overlays, sources) as clickable cards that
deep-link into filtered lists, DAU/WAU/MAU active-user counts, a per-platform
source breakdown, and the nav grid.

### Search (`/admin/search`, ADR-0035)
One box that resolves a query across users, overlays, sources, and viewers and
deep-links each hit into the addressable views. Users/overlays/sources are
federated on the client over the admin list endpoints; viewers use the
server-side `?q=` search.

### Users (`/admin/users`)
List with avatar, platform badges, and status; search by username/display
name/id/platform id; filter tabs (all/active/banned/premium/beta). Detail panel:
impersonate, grant/revoke premium (optionally time-limited, ADR-0027),
grant/revoke beta-tester (ADR-0020), ban/unban, and the user's overlays (linked
to the admin overlay detail, with a secondary link to the live overlay) plus a
"view this user's sources" link.

### Overlays (`/admin/overlays`)
List with live-connection status (and a "status unavailable" notice when the
connection endpoint can't be read). Detail panel resolves the **owner** to a
linked `@username` (not a bare UUID) and lists connected sources with channel
links.

### Sources (`/admin/sources`)
Every source across all overlays, filterable by platform/status and searchable
by channel/overlay/owner. Each channel resolves to its public platform page via
`ChannelLink`; each row links to its overlay and to its owning user.

### Viewers (`/admin/viewers`, ADR-0034)
Server-side search, platform/status/premium filters, and pagination with a
correct full-dataset total. Each row shows the avatar, platform badge, platform
user id, and a link to the viewer's platform profile. An **Activity** dialog
surfaces cross-streamer context (total messages, last seen, and the
streamers/overlays the viewer chats in) from the per-session aggregate over
`viewer_message_history`. Ban/unban and grant/revoke premium are inline.

### Cosmetics / Features / Maintenance
`/admin/cosmetics` manages the avatar-frame and flair catalog; `/admin/features`
manages premium feature gates (ADR-0008); `/admin/maintenance` toggles
maintenance mode.

## Channel resolution

`lib/platform-channel-url.ts` + `components/ChannelLink.tsx` turn a source's
stored channel identifier into a link to the real platform page:

- twitch -> `twitch.tv/{login}`, kick -> `kick.com/{slug}`, tiktok -> `tiktok.com/@{username}`
- youtube -> only when a real `@handle` or canonical `UC…` channel id is present (the account-linked `youtube_id` is a Google account id, not a channel id, and is deliberately never linked)
- discord (snowflake) / shared_overlay (overlay UUID) -> plain text

## Backend endpoints

- overlay-manager: `GET /api/v1/admin/overlays`, `/admin/overlays/active`, `/admin/overlays/:id/sources`, `/admin/sources`, `/admin/user-overlays/:id` — overlay/source responses include `owner_username`/`owner_display_name` (joined from `users`) and `channel_handle`.
- auth-service: `GET /api/v1/admin/users`, `/admin/viewers` (with `q`/`is_banned`/`is_premium`/`platform`/`limit`/`offset` and a `total`), `/admin/viewers/:session_id/activity`, plus ban/unban/premium/beta mutations and impersonation.
- share-service: admin feature gates.

## Related ADRs

- ADR-0036 — admin URL-addressable selection
- ADR-0034 — admin viewer identity model
- ADR-0035 — admin global entity search
- ADR-0008 — premium feature gates · ADR-0020 — beta-tester role · ADR-0027 — time-limited premium overrides
