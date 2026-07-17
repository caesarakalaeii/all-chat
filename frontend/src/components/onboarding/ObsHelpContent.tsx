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
 * The canonical "add your overlay to OBS" instructions, shared by the
 * onboarding setup guide (step 4) and the editor's OBS help dialog so the
 * copy can never drift between the two.
 */

export function ObsHelpContent() {
  return (
    <ol className="list-decimal space-y-1.5 pl-5 text-sm text-text-sub">
      <li>
        In OBS, click <strong className="text-text">+</strong> under Sources and choose{' '}
        <strong className="text-text">Browser</strong>.
      </li>
      <li>Paste the copied overlay link into the URL field.</li>
      <li>
        Size the source to the area chat should fill (a tall, narrow box like 450×800 works well,
        not your full canvas), then drag it into place. Chat appears as soon as the overlay
        connects.
      </li>
    </ol>
  )
}
