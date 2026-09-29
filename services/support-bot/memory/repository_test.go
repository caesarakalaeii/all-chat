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

package memory

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractTags(t *testing.T) {
	got := ExtractTags("Why does kick-listener keep getting OOMKilled and timeout on redis?")
	sort.Strings(got)
	want := []string{"kick-listener", "oomkill", "redis", "timeout"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractTags = %v, want %v", got, want)
	}
}

func TestExtractTagsNoMatch(t *testing.T) {
	if got := ExtractTags("how do I set up an overlay?"); len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Redis ", "OOMKill", "", "  "})
	want := []string{"redis", "oomkill"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTags = %v, want %v", got, want)
	}
}

func TestValidType(t *testing.T) {
	for _, ty := range []Type{TypeErrorPattern, TypeCorrection, TypeCodebaseInsight} {
		if !ValidType(ty) {
			t.Errorf("%q should be valid", ty)
		}
	}
	if ValidType(Type("nonsense")) {
		t.Error("unknown type should be invalid")
	}
}
