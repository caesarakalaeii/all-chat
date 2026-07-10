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

// Package sanitize provides prompt-injection hygiene: it strips invisible
// control/format/bidi characters and escapes markup from any text that is echoed
// into the model prompt, wraps tool results in unique XML boundary tags so the model
// treats them as data rather than instructions, and strips internal scaffolding tags
// out of model-authored text before it crosses the delivery boundary into Discord.
package sanitize

import (
	"regexp"
	"strings"
	"unicode"
)

// Boundary tag names used to fence tool results in the prompt.
const (
	TagToolOutput = "tool_output"
	TagToolError  = "tool_error"
)

// internalTags are removed (with their bodies) from model output before delivery.
var internalTags = []string{
	"tool_call", "function_calls", "system-reminder", "previous_response",
	"tool_output", "tool_error", "thinking", "scratchpad", "internal", "claude_internal",
}

var (
	// scaffoldRes matches a well-formed <tag ...>...</tag> or self-closing <tag .../>
	// for each denylisted tag. Case-insensitive, dot matches newline.
	scaffoldRes = buildScaffoldRegexps()
	// boundaryRe neutralizes an opening/closing boundary tag appearing inside content.
	boundaryRe = regexp.MustCompile(`(?i)<(\s*/?\s*tool_(?:output|error))`)
)

func buildScaffoldRegexps() []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(internalTags))
	for _, t := range internalTags {
		q := regexp.QuoteMeta(t)
		res = append(res, regexp.MustCompile(`(?is)<`+q+`\b[^>]*>.*?</`+q+`\s*>|<`+q+`\b[^>]*/>`))
	}
	return res
}

// SanitizeForPrompt makes untrusted text (user messages, tool-derived strings) safe
// to embed in the prompt: it drops control chars (except \n and \t), zero-width and
// bidirectional formatting marks, line/paragraph separators, and escapes the three
// markup metacharacters plus their fullwidth homoglyphs.
func SanitizeForPrompt(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' {
			b.WriteRune(r)
			continue
		}
		if isStripped(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	// escape markup last so we don't double-process
	out = strings.ReplaceAll(out, "&", "&amp;")
	out = strings.ReplaceAll(out, "<", "&lt;")
	out = strings.ReplaceAll(out, ">", "&gt;")
	// fullwidth homoglyphs used to smuggle markup past naive filters
	out = strings.ReplaceAll(out, "\uFF06", "&amp;") // FULLWIDTH AMPERSAND
	out = strings.ReplaceAll(out, "\uFF1C", "&lt;")  // FULLWIDTH LESS-THAN SIGN
	out = strings.ReplaceAll(out, "\uFF1E", "&gt;")  // FULLWIDTH GREATER-THAN SIGN
	return out
}

// SanitizeForAttribute is SanitizeForPrompt plus quote escaping, for values placed
// inside an XML attribute (e.g. the tool name in a boundary tag).
func SanitizeForAttribute(s string) string {
	out := SanitizeForPrompt(s)
	out = strings.ReplaceAll(out, "\"", "&quot;")
	out = strings.ReplaceAll(out, "'", "&#39;")
	return out
}

// isStripped reports whether a rune is an invisible control/format character that
// must never survive into the prompt.
func isStripped(r rune) bool {
	switch {
	case r == '\u00AD': // soft hyphen
		return true
	case r >= '\u200B' && r <= '\u200F': // zero-width space/non-joiner/joiner + LR/RL marks
		return true
	case r >= '\u202A' && r <= '\u202E': // bidi embeddings/overrides
		return true
	case r >= '\u2060' && r <= '\u2064': // word joiner + invisible math operators
		return true
	case r >= '\u2066' && r <= '\u2069': // bidi isolates (LRI/RLI/FSI/PDI)
		return true
	case r == '\u2028' || r == '\u2029': // line/paragraph separators
		return true
	case r == '\uFEFF': // BOM / zero-width no-break space
		return true
	case unicode.IsControl(r): // remaining C0/C1 controls (\n,\t handled above)
		return true
	case unicode.Is(unicode.Cf, r): // any other format char
		return true
	}
	return false
}

// WrapToolOutput fences a successful tool result in a boundary tag. The tool name is
// attribute-escaped and any nested boundary tag in the content is neutralized so the
// model cannot be tricked into treating tool content as a new instruction block.
func WrapToolOutput(toolName, out string) string {
	return wrap(TagToolOutput, toolName, out, "")
}

// WrapToolError fences a failed tool result.
func WrapToolError(toolName, errMsg string) string {
	return wrap(TagToolError, toolName, errMsg, ` is_error="true"`)
}

func wrap(tag, toolName, body, extraAttr string) string {
	name := SanitizeForAttribute(toolName)
	safe := escapeBoundaryTags(body)
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(tag)
	b.WriteString(` name="`)
	b.WriteString(name)
	b.WriteString(`"`)
	b.WriteString(extraAttr)
	b.WriteString(">\n")
	b.WriteString(safe)
	b.WriteString("\n</")
	b.WriteString(tag)
	b.WriteString(">")
	return b.String()
}

// escapeBoundaryTags neutralizes any literal tool_output/tool_error open/close tag in
// the content by escaping its leading angle bracket.
func escapeBoundaryTags(s string) string {
	return boundaryRe.ReplaceAllString(s, "&lt;$1")
}

// StripInternalScaffolds removes well-formed internal/instruction tags (and their
// bodies) from model-authored text before it is delivered to Discord. Unknown tags
// and bare comparisons like "5 < 7" are preserved.
func StripInternalScaffolds(s string) string {
	for _, re := range scaffoldRes {
		s = re.ReplaceAllString(s, "")
	}
	return s
}
