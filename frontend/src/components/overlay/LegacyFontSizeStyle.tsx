/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * Delivers the legacy display-settings font-size as the `--chat-legacy-font-size`
 * custom property on `:root`, consumed by the `.break-words` rules in
 * events.css. A style tag rather than an inline style on the message rows so
 * manual custom CSS can override it like the GUI font-size control can. Both
 * overlay surfaces (live overlay and editor preview) render it through this
 * component so the property name and unit cannot drift apart; the preview
 * passes an id because its style tags are addressed by contract id.
 */
export function LegacyFontSizeStyle({
  fontSize,
  id,
}: {
  fontSize: number | null
  id?: string
}) {
  if (fontSize === null) {
    return null
  }
  return (
    <style
      id={id}
      dangerouslySetInnerHTML={{ __html: `:root { --chat-legacy-font-size: ${fontSize}px; }` }}
    />
  )
}
