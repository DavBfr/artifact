package main

import (
	"strings"
	"testing"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantSuffix string
		wantErr    bool
	}{
		{name: "explicit suffix", input: "cat:3.5", wantName: "cat", wantSuffix: "3.5"},
		{name: "namespaced name", input: "pets/cat:6.0", wantName: "pets/cat", wantSuffix: "6.0"},
		{name: "deep namespace", input: "a/b/c:1", wantName: "a/b/c", wantSuffix: "1"},
		{name: "bare name defaults to latest", input: "cat", wantName: "cat", wantSuffix: "latest"},
		{name: "bare namespaced name", input: "pets/cat", wantName: "pets/cat", wantSuffix: "latest"},
		{name: "case is preserved", input: "Cat:Latest", wantName: "Cat", wantSuffix: "Latest"},
		{name: "surrounding whitespace is trimmed", input: "  cat:3.5  ", wantName: "cat", wantSuffix: "3.5"},
		{name: "dashes underscores and dots", input: "my-cat_2024:1.0-rc.2", wantName: "my-cat_2024", wantSuffix: "1.0-rc.2"},
		{name: "exactly at the length limit", input: strings.Repeat("a", 248) + ":latest", wantName: strings.Repeat("a", 248), wantSuffix: "latest"},

		{name: "empty", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "empty suffix", input: "cat:", wantErr: true},
		{name: "empty name", input: ":latest", wantErr: true},
		// The split is on the last colon, so an earlier one ends up in the name,
		// which names are not allowed to contain.
		{name: "second colon lands in the name", input: "a/b:c:1.0", wantErr: true},
		{name: "over the length limit", input: strings.Repeat("a", 249) + ":latest", wantErr: true},
		{name: "leading slash", input: "/cat:1", wantErr: true},
		{name: "trailing slash", input: "cat/:1", wantErr: true},
		{name: "empty segment", input: "pets//cat:1", wantErr: true},
		{name: "dot segment", input: "pets/./cat:1", wantErr: true},
		{name: "dotdot segment", input: "pets/../cat:1", wantErr: true},
		{name: "slash in suffix", input: "cat:1/2", wantErr: true},
		{name: "space inside", input: "my cat:1", wantErr: true},
		{name: "non-ascii", input: "cät:1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tag, err := parseTag(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTag(%q) = %q, want an error", tc.input, tag.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTag(%q) returned unexpected error: %v", tc.input, err)
			}
			if tag.Name != tc.wantName || tag.Suffix != tc.wantSuffix {
				t.Fatalf(
					"parseTag(%q) = %q:%q, want %q:%q",
					tc.input, tag.Name, tag.Suffix, tc.wantName, tc.wantSuffix,
				)
			}
		})
	}
}

func TestTagString(t *testing.T) {
	tag := Tag{Name: "pets/cat", Suffix: "6.0"}
	if got := tag.String(); got != "pets/cat:6.0" {
		t.Fatalf("Tag.String() = %q, want %q", got, "pets/cat:6.0")
	}
}

func TestParseTagList(t *testing.T) {
	tags, err := parseTagList([]string{"cat:latest", "pets/cat:6.0", "cat:latest", "cat"})
	if err != nil {
		t.Fatalf("parseTagList returned unexpected error: %v", err)
	}
	// Duplicates are dropped (a bare "cat" is the same tag as "cat:latest"),
	// and the caller's order is preserved.
	want := []string{"cat:latest", "pets/cat:6.0"}
	if len(tags) != len(want) {
		t.Fatalf("parseTagList returned %d tags, want %d: %v", len(tags), len(want), tags)
	}
	for i, tag := range tags {
		if tag.String() != want[i] {
			t.Fatalf("parseTagList()[%d] = %q, want %q", i, tag.String(), want[i])
		}
	}
}

func TestParseTagListEmpty(t *testing.T) {
	tags, err := parseTagList(nil)
	if err != nil {
		t.Fatalf("parseTagList(nil) returned unexpected error: %v", err)
	}
	// An upload without tags is valid, so an empty list is not an error - the
	// result just has to be usable rather than nil.
	if tags == nil || len(tags) != 0 {
		t.Fatalf("parseTagList(nil) = %v, want an empty non-nil slice", tags)
	}
}

func TestParseTagListRejectsInvalid(t *testing.T) {
	if _, err := parseTagList([]string{"ok:1", "not a tag"}); err == nil {
		t.Fatal("parseTagList accepted an invalid tag")
	}
}
