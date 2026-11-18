// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package gateway

import "strings"

// maxAttachmentsPerMessage bounds how many media items a single message can carry
// downstream, so a message with dozens of uploads cannot flood an overlay.
const maxAttachmentsPerMessage = 4

// spoilerFilenamePrefix marks an uploaded attachment as a spoiler (Discord clients
// prepend it when the user ticks "Mark as spoiler").
const spoilerFilenamePrefix = "SPOILER_"

// RawAttachment is the normalized media item forwarded on the raw message. It is
// deliberately mirrored by publisher.Attachment (matching JSON tags) so it survives
// the map -> JSON -> RawMessage round-trip performed by the publisher adapter.
type RawAttachment struct {
	Type        string `json:"type"`                   // "image" or "video"
	URL         string `json:"url"`                    // display URL (Discord proxy preferred)
	ContentType string `json:"content_type,omitempty"` // MIME, e.g. image/gif, video/mp4
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	ThumbURL    string `json:"thumb_url,omitempty"` // poster frame for videos
	Spoiler     bool   `json:"spoiler,omitempty"`
	Filename    string `json:"filename,omitempty"` // for alt text (spoiler prefix stripped)
}

// buildRawAttachments extracts renderable image/video media from a Discord message:
// uploaded attachments first, then media-bearing embeds (Tenor/Giphy link previews
// and pasted image links). Non-media attachments/embeds (documents, plain link
// previews, articles) are ignored. The result is capped at maxAttachmentsPerMessage.
func buildRawAttachments(msg MessageCreateData) []RawAttachment {
	out := make([]RawAttachment, 0)

	for _, a := range msg.Attachments {
		kind := classifyMediaType(a.ContentType)
		if kind == "" {
			continue // not an image or video (document, audio, ...)
		}
		url := a.ProxyURL
		if url == "" {
			url = a.URL
		}
		if url == "" {
			continue
		}
		spoiler := strings.HasPrefix(a.Filename, spoilerFilenamePrefix)
		out = append(out, RawAttachment{
			Type:        kind,
			URL:         url,
			ContentType: a.ContentType,
			Width:       a.Width,
			Height:      a.Height,
			Spoiler:     spoiler,
			Filename:    strings.TrimPrefix(a.Filename, spoilerFilenamePrefix),
		})
		if len(out) >= maxAttachmentsPerMessage {
			return out
		}
	}

	for _, e := range msg.Embeds {
		att, ok := attachmentFromEmbed(e)
		if !ok {
			continue
		}
		out = append(out, att)
		if len(out) >= maxAttachmentsPerMessage {
			return out
		}
	}

	return out
}

// classifyMediaType maps a MIME type to the coarse attachment kind the overlay
// knows how to render. GIFs are image/gif and animate natively in an <img>, so
// they are classified as images. Anything that is not image/* or video/* is
// dropped (returns "").
func classifyMediaType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	default:
		return ""
	}
}

// attachmentFromEmbed pulls renderable media out of an auto-generated embed.
// gifv/video embeds (Tenor, Giphy, direct video links) become looping videos;
// image embeds become images. Discord always re-hosts embed media on its own
// CDN, so proxy_url is preferred over the third-party url. Returns ok=false for
// embeds with no usable media (articles, plain links, rich embeds).
func attachmentFromEmbed(e DiscordEmbed) (RawAttachment, bool) {
	switch e.Type {
	case "gifv", "video":
		if e.Video == nil {
			return RawAttachment{}, false
		}
		url := firstNonEmptyURL(e.Video.ProxyURL, e.Video.URL)
		if url == "" {
			return RawAttachment{}, false
		}
		att := RawAttachment{
			Type:   "video",
			URL:    url,
			Width:  e.Video.Width,
			Height: e.Video.Height,
		}
		if e.Thumbnail != nil {
			att.ThumbURL = firstNonEmptyURL(e.Thumbnail.ProxyURL, e.Thumbnail.URL)
		}
		return att, true
	case "image":
		// Discord puts a pasted image link's media in thumbnail and usually leaves
		// image unset; image is still read first for embeds that do carry it.
		media := e.Image
		if media == nil {
			media = e.Thumbnail
		}
		if media == nil {
			return RawAttachment{}, false
		}
		url := firstNonEmptyURL(media.ProxyURL, media.URL)
		if url == "" {
			return RawAttachment{}, false
		}
		return RawAttachment{
			Type:   "image",
			URL:    url,
			Width:  media.Width,
			Height: media.Height,
		}, true
	default:
		return RawAttachment{}, false
	}
}

func firstNonEmptyURL(candidates ...string) string {
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return ""
}

// hasEmbedMedia reports whether any embed carries renderable media. Discord adds
// link previews (Tenor/Giphy GIFs, pasted image links) in a MESSAGE_UPDATE after
// the message was created, so this is what makes such an update worth forwarding.
func hasEmbedMedia(msg MessageCreateData) bool {
	for _, e := range msg.Embeds {
		if _, ok := attachmentFromEmbed(e); ok {
			return true
		}
	}
	return false
}
