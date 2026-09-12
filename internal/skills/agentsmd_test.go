// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// wellFormedBlock builds a trivial, real, well-formed block for use as
// Splice's block argument in tests that don't need RenderBlock's actual
// rendering.
func wellFormedBlock(interiorLines ...string) string {
	var b strings.Builder
	b.WriteString(BlockStartMarker)
	b.WriteByte('\n')
	for _, l := range interiorLines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString(BlockEndMarker)
	b.WriteByte('\n')
	return b.String()
}

// TestAgentsMdSplice covers every bullet in 04-02-PLAN.md Task 2's
// behavior block as table-driven subtests over byte-slice fixtures.
func TestAgentsMdSplice(t *testing.T) {
	t.Run("scan: no marker of either kind reports absent and no offsets", func(t *testing.T) {
		state, _, _, _, _, offsets := scanBlock([]byte("just some ordinary file content\nwith a few lines\n"))
		if state != blockAbsent {
			t.Fatalf("state = %v, want blockAbsent", state)
		}
		if len(offsets) != 0 {
			t.Errorf("offsets = %v, want empty", offsets)
		}
	})

	t.Run("scan: one start then one end reports well-formed with marker line offsets", func(t *testing.T) {
		content := []byte("prefix line\n" + BlockStartMarker + "\nbody\n" + BlockEndMarker + "\nsuffix line\n")
		state, startOffset, startLen, endOffset, endLen, _ := scanBlock(content)
		if state != blockWellFormed {
			t.Fatalf("state = %v, want blockWellFormed", state)
		}
		if got := string(content[startOffset : startOffset+startLen]); got != BlockStartMarker {
			t.Errorf("start marker line slice = %q, want %q", got, BlockStartMarker)
		}
		if got := string(content[endOffset : endOffset+endLen]); got != BlockEndMarker {
			t.Errorf("end marker line slice = %q, want %q", got, BlockEndMarker)
		}
	})

	t.Run("scan: two pairs reports malformed with every marker offset", func(t *testing.T) {
		content := []byte(BlockStartMarker + "\na\n" + BlockEndMarker + "\n" + BlockStartMarker + "\nb\n" + BlockEndMarker + "\n")
		state, _, _, _, _, offsets := scanBlock(content)
		if state != blockMalformed {
			t.Fatalf("state = %v, want blockMalformed", state)
		}
		if len(offsets) != 4 {
			t.Fatalf("offsets = %v, want 4 entries", offsets)
		}
	})

	t.Run("scan: start with no end reports malformed with the start's offset", func(t *testing.T) {
		content := []byte("before\n" + BlockStartMarker + "\nbody with no closing marker\n")
		state, _, _, _, _, offsets := scanBlock(content)
		if state != blockMalformed {
			t.Fatalf("state = %v, want blockMalformed", state)
		}
		wantOffset := strings.Index(string(content), BlockStartMarker)
		if len(offsets) != 1 || offsets[0] != wantOffset {
			t.Errorf("offsets = %v, want [%d]", offsets, wantOffset)
		}
	})

	t.Run("scan: end with no start reports malformed with the end's offset", func(t *testing.T) {
		content := []byte("before\n" + BlockEndMarker + "\nafter\n")
		state, _, _, _, _, offsets := scanBlock(content)
		if state != blockMalformed {
			t.Fatalf("state = %v, want blockMalformed", state)
		}
		wantOffset := strings.Index(string(content), BlockEndMarker)
		if len(offsets) != 1 || offsets[0] != wantOffset {
			t.Errorf("offsets = %v, want [%d]", offsets, wantOffset)
		}
	})

	t.Run("scan: a second start before the first's end reports malformed with both start offsets", func(t *testing.T) {
		content := []byte(BlockStartMarker + "\na\n" + BlockStartMarker + "\nb\n" + BlockEndMarker + "\n")
		state, _, _, _, _, offsets := scanBlock(content)
		if state != blockMalformed {
			t.Fatalf("state = %v, want blockMalformed", state)
		}
		firstStart := strings.Index(string(content), BlockStartMarker)
		secondStart := strings.LastIndex(string(content), BlockStartMarker)
		if len(offsets) < 2 || offsets[0] != firstStart || offsets[1] != secondStart {
			t.Errorf("offsets = %v, want to start with [%d %d]", offsets, firstStart, secondStart)
		}
	})

	t.Run("scan: a duplicated end reports malformed", func(t *testing.T) {
		content := []byte(BlockStartMarker + "\na\n" + BlockEndMarker + "\n" + BlockEndMarker + "\n")
		state, _, _, _, _, offsets := scanBlock(content)
		if state != blockMalformed {
			t.Fatalf("state = %v, want blockMalformed", state)
		}
		if len(offsets) != 3 {
			t.Fatalf("offsets = %v, want 3 entries (one start, two ends)", offsets)
		}
	})

	t.Run("splice: absent content gets the block appended after exactly one blank line", func(t *testing.T) {
		content := []byte("existing content\nwith no trailing blank line\n")
		block := wellFormedBlock("new interior")
		got, err := Splice(content, block)
		if err != nil {
			t.Fatalf("Splice: %v", err)
		}
		want := string(content) + "\n" + block
		if string(got) != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
		if !bytes.Equal(got[:len(content)], content) {
			t.Error("preceding bytes were not left unchanged")
		}
	})

	t.Run("splice: absent content already ending with a blank line gets no extra blank line", func(t *testing.T) {
		content := []byte("existing content\n\n")
		block := wellFormedBlock("new interior")
		got, err := Splice(content, block)
		if err != nil {
			t.Fatalf("Splice: %v", err)
		}
		want := string(content) + block
		if string(got) != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("splice: content with no trailing newline gets one added before the blank line", func(t *testing.T) {
		content := []byte("existing content with no trailing newline")
		block := wellFormedBlock("new interior")
		got, err := Splice(content, block)
		if err != nil {
			t.Fatalf("Splice: %v", err)
		}
		want := string(content) + "\n\n" + block
		if string(got) != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("splice: completely empty content produces exactly the block", func(t *testing.T) {
		block := wellFormedBlock("new interior")
		got, err := Splice(nil, block)
		if err != nil {
			t.Fatalf("Splice: %v", err)
		}
		if string(got) != block {
			t.Errorf("got %q, want exactly the block %q", got, block)
		}
	})

	t.Run("splice: well-formed content has only the interior replaced", func(t *testing.T) {
		content := []byte("prefix\n" + BlockStartMarker + "\nold interior line\n" + BlockEndMarker + "\nsuffix\n")
		block := wellFormedBlock("new interior line one", "new interior line two")
		got, err := Splice(content, block)
		if err != nil {
			t.Fatalf("Splice: %v", err)
		}

		prefixWant := "prefix\n" + BlockStartMarker + "\n"
		suffixWant := BlockEndMarker + "\nsuffix\n"
		if !bytes.Equal(got[:len(prefixWant)], []byte(prefixWant)) {
			t.Errorf("bytes before and including the start marker line changed: got %q, want prefix %q", got, prefixWant)
		}
		if !bytes.Equal(got[len(got)-len(suffixWant):], []byte(suffixWant)) {
			t.Errorf("bytes from the end marker line onward changed: got %q, want suffix %q", got, suffixWant)
		}
		if !strings.Contains(string(got), "new interior line one") || !strings.Contains(string(got), "new interior line two") {
			t.Errorf("got %q, want it to contain the new interior lines", got)
		}
		if strings.Contains(string(got), "old interior line") {
			t.Errorf("got %q, old interior line was not replaced", got)
		}
	})

	t.Run("splice: idempotent — splicing the same block twice yields byte-identical content", func(t *testing.T) {
		content := []byte("prefix\nmore prefix\n")
		block := wellFormedBlock("interior line")

		first, err := Splice(content, block)
		if err != nil {
			t.Fatalf("Splice (first): %v", err)
		}
		second, err := Splice(first, block)
		if err != nil {
			t.Fatalf("Splice (second): %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("splicing twice produced different content:\nfirst:  %q\nsecond: %q", first, second)
		}
	})

	t.Run("splice: malformed content returns nil content and an error naming every offset", func(t *testing.T) {
		content := []byte(BlockStartMarker + "\na\n" + BlockEndMarker + "\n" + BlockStartMarker + "\nb\n" + BlockEndMarker + "\n")
		block := wellFormedBlock("interior")
		got, err := Splice(content, block)
		if got != nil {
			t.Errorf("got %q, want nil content on a malformed input", got)
		}
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		_, _, _, _, _, offsets := scanBlock(content)
		for _, off := range offsets {
			if !strings.Contains(err.Error(), strconv.Itoa(off)) {
				t.Errorf("error %q does not name offset %d", err.Error(), off)
			}
		}
	})
}

// TestRenderBlockIsAnIndexNotABody asserts the rendered block for the
// REAL embedded inventory is small (an index, never the skills' bodies)
// and contains no skill's body text, located structurally rather than by
// a hardcoded phrase.
func TestRenderBlockIsAnIndexNotABody(t *testing.T) {
	inv, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory(): %v", err)
	}
	if len(inv) == 0 {
		t.Fatal("Inventory() returned zero skills")
	}

	block := RenderBlock(inv, "/home/example/.claude/skills")

	if len(block) >= 4096 {
		t.Errorf("rendered block is %d bytes, want under 4096 (an index, not the skill bodies)", len(block))
	}

	first := inv[0]
	var skillMD *File
	for i := range first.Files {
		if first.Files[i].Path == "SKILL.md" {
			skillMD = &first.Files[i]
		}
	}
	if skillMD == nil {
		t.Fatalf("skill %q has no SKILL.md", first.Name)
	}
	// Locate a distinctive slice of the first skill's body — well past its
	// frontmatter fence and long enough not to coincide with the rendered
	// index line by chance.
	lines := bytes.Split(skillMD.Content, []byte("\n"))
	var bodySlice string
	for _, l := range lines {
		trimmed := strings.TrimSpace(string(l))
		if len(trimmed) > 40 && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "#") {
			bodySlice = trimmed
			break
		}
	}
	if bodySlice == "" {
		t.Fatal("could not locate a distinctive body line in the first skill's SKILL.md to test against")
	}
	if strings.Contains(block, bodySlice) {
		t.Errorf("rendered block contains a body-text slice %q — it must be an index only", bodySlice)
	}

	for _, s := range inv {
		if !strings.Contains(block, s.Name) {
			t.Errorf("rendered block does not mention skill %q", s.Name)
		}
	}
}
