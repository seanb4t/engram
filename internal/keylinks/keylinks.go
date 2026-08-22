// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package keylinks decides whether a plan's key-link pattern: field is a
// real gate or a silent no-op. gsd-core's parseMustHavesBlock hands that
// field to JavaScript's `new RegExp` without ever unescaping it
// (verify.cjs:1049-1117), so a pattern authored with doubled escape
// characters (`\\.`) reaches the regex engine meaning "a literal
// backslash, then any character" — valid regex, silently unmatchable
// (#479). A second silent-no-op shape exists alongside it: a pattern
// that compiles and is correctly escaped, yet can never match its from
// file's actual content, because the symbol it names moved (#479's
// second finding).
//
// This package's two callers are this phase's own recurring gate test
// (internal/keylinks/keylinks_test.go, exercised by `go test ./...`) and
// the one-time v0.13.x Phase 1-2 reassessment sweep. It is a stdlib-only
// leaf — no repo-internal imports — so either caller can depend on it
// with zero import-cycle risk.
//
// Per D-01 this is a repo-local guard: it normalizes and re-validates
// this repo's own patterns, but it does not patch gsd-core's
// parseMustHavesBlock. The unescaping gap in the upstream tool is
// reported separately, outside this package's scope.
package keylinks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// KeyLink is one `- from:`/`to:`/`via:`/`pattern:` entry parsed out of a
// plan's must_haves.key_links frontmatter block. File and Line locate the
// entry's pattern: key in the SOURCE plan file, for offender reporting.
type KeyLink struct {
	From    string
	To      string
	Via     string
	Pattern string
	File    string
	Line    int
}

// Shape classifies why a pattern: value failed validation.
type Shape string

const (
	// ShapeEscaping marks a pattern whose raw string carries a backslash
	// character — the doubled-escape corruption D-02 exists to catch.
	// This compiles cleanly under RE2 with the wrong semantics, so it is
	// detected by an unconditional string scan that runs first, never by
	// compile failure (Pitfall 1). Because this check is unconditional,
	// a backreference pattern (which necessarily contains a backslash,
	// e.g. `\1`) is also caught here rather than reaching regexp.Compile
	// — it is still correctly rejected, just under this shape instead of
	// ShapeCompileError.
	ShapeEscaping Shape = "escaping"

	// ShapeUnsupportedSyntax is reserved for a compile error whose
	// message names a specifically JS-only construct, as distinct from a
	// generically malformed pattern. This package does not currently
	// classify into it: RE2's error messages for lookahead, lookbehind,
	// and backreferences are not uniform enough to substring-match
	// without a bespoke detector — the exact duplication 01-RESEARCH.md's
	// "Don't Hand-Roll" guidance warns against. Every such compile
	// failure instead surfaces as ShapeCompileError, carrying RE2's own
	// message verbatim.
	ShapeUnsupportedSyntax Shape = "unsupported-syntax"

	// ShapeNamedGroup marks a pattern carrying a named capture group, in
	// either Go's (?P<name>...) or JavaScript's (?<name>...) syntax —
	// both compile successfully under this repo's Go 1.26 toolchain, so
	// one re.SubexpNames() sweep after compile catches both uniformly.
	ShapeNamedGroup Shape = "named-group"

	// ShapeCompileError marks any pattern regexp.Compile rejects: RE2
	// already refuses lookahead, negative lookahead, and lookbehind at
	// parse time with a stable message, so this shape needs no bespoke
	// per-construct detector.
	ShapeCompileError Shape = "compile-error"

	// ShapeUnsatisfiable marks a pattern that compiles, is escape-clean,
	// and carries no named group, but cannot match its from file's
	// content (nor, as a fallback, its to file's) — #479's second
	// finding.
	ShapeUnsatisfiable Shape = "unsatisfiable"
)

// Offender is one reported violation: where it was found, which shape it
// is, the raw pattern that failed, and — when mechanically derivable —
// the corrected form.
type Offender struct {
	File  string
	Line  int
	Shape Shape
	Raw   string
	Fix   string
}

// Mode selects which checks ScanPlans runs. The two halves of the guard
// have different time-sensitivity (D-04): escaping is true or false
// forever and runs over every plan; satisfiability depends on the code
// as it stands right now and is meaningful only for the active
// milestone.
type Mode int

const (
	// ModeEscapingOnly runs only ValidatePattern's checks.
	ModeEscapingOnly Mode = iota
	// ModeSatisfiability additionally runs CheckSatisfiable for every
	// pattern that validated cleanly.
	ModeSatisfiability
)

// ParsePlanKeyLinks scans path's frontmatter for a must_haves.key_links
// block and returns one KeyLink per `- from:` entry, mirroring the flat,
// YAML-ish parsing gsd-core's own parseMustHavesBlock does. It enters
// link-parsing state only after a key_links: key found inside the
// frontmatter's must_haves: block, and leaves that state at the next key
// at or above must_haves:'s indentation — a pattern: key mentioned
// anywhere else in the document (including this phase's own plan prose)
// is never treated as a key link. It strips a single pair of
// leading/trailing quote characters from each scalar value exactly as
// the consuming tool does, and never backslash-unescapes.
func ParsePlanKeyLinks(path string) ([]KeyLink, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("keylinks: parse %s: %w", path, err)
	}

	rawLines := strings.Split(string(data), "\n")
	if len(rawLines) == 0 || strings.TrimRight(rawLines[0], "\r") != "---" {
		return nil, nil
	}
	end := -1
	for i := 1; i < len(rawLines); i++ {
		if strings.TrimRight(rawLines[i], "\r") == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, nil
	}

	const (
		stateOutside = iota
		stateMustHaves
		stateKeyLinks
	)

	state := stateOutside
	mustHavesIndent := -1
	keyLinksIndent := -1
	listItemIndent := -1

	var links []KeyLink
	var current *KeyLink

	flush := func() {
		if current != nil {
			links = append(links, *current)
			current = nil
		}
	}

	for i := 1; i < end; i++ {
		line := strings.TrimRight(rawLines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if state == stateKeyLinks && indent <= keyLinksIndent {
			flush()
			state = stateMustHaves
			listItemIndent = -1
		}
		if state == stateMustHaves && indent <= mustHavesIndent {
			state = stateOutside
		}

		switch state {
		case stateOutside:
			if trimmed == "must_haves:" {
				state = stateMustHaves
				mustHavesIndent = indent
			}
		case stateMustHaves:
			if trimmed == "key_links:" {
				state = stateKeyLinks
				keyLinksIndent = indent
				listItemIndent = -1
			}
		case stateKeyLinks:
			if strings.HasPrefix(trimmed, "- ") {
				if listItemIndent == -1 {
					listItemIndent = indent
				}
				if indent == listItemIndent {
					flush()
					current = &KeyLink{File: path}
					applyKeyLinkField(current, strings.TrimSpace(trimmed[2:]), i+1)
				}
			} else if current != nil && indent > listItemIndent {
				applyKeyLinkField(current, trimmed, i+1)
			}
		}
	}
	flush()

	return links, nil
}

// applyKeyLinkField parses one "key: value" line and, if key names a
// KeyLink field, sets it. lineNum is the 1-based source line, recorded
// onto link.Line only for the pattern: key, since that is the line an
// offender must point a reader at.
func applyKeyLinkField(link *KeyLink, kv string, lineNum int) {
	idx := strings.Index(kv, ":")
	if idx == -1 {
		return
	}
	key := strings.TrimSpace(kv[:idx])
	val := stripQuotes(strings.TrimSpace(kv[idx+1:]))

	switch key {
	case "from":
		link.From = val
	case "to":
		link.To = val
	case "via":
		link.Via = val
	case "pattern":
		link.Pattern = val
		link.Line = lineNum
	}
}

// stripQuotes removes a single leading and trailing matching quote
// character, exactly as gsd-core's parser does — never backslash-
// unescaping, since a pattern's own escaping correctness is precisely
// what this package validates.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ValidatePattern decides whether raw is a real, matchable regex under
// D-08's RE2 ∩ JavaScript common-subset restriction. The escaping check
// runs first and unconditionally: `\\.` compiles cleanly under RE2 with
// the wrong semantics ("a backslash, then any character"), so gating
// this on compile failure would be the defect's own logic relocated
// (Pitfall 1). A nil offender with a non-nil *regexp.Regexp means raw is
// valid; a non-nil offender means raw is rejected and the regexp is nil.
func ValidatePattern(raw string) (*regexp.Regexp, *Offender) {
	if strings.ContainsRune(raw, '\\') {
		return nil, &Offender{
			Shape: ShapeEscaping,
			Raw:   raw,
			Fix:   SuggestCharClassForm(raw),
		}
	}

	re, err := regexp.Compile(raw)
	if err != nil {
		// RE2 already refuses lookahead, negative lookahead, and
		// lookbehind at parse time with a stable message — no bespoke
		// detector needed for any of them (Pattern 1, 01-RESEARCH.md).
		return nil, &Offender{
			Shape: ShapeCompileError,
			Raw:   raw,
			Fix:   err.Error(),
		}
	}

	// One SubexpNames() sweep catches a named capture group in either
	// Go's (?P<name>...) syntax or JavaScript's (?<name>...) syntax —
	// both compile successfully under this repo's Go 1.26 toolchain, so
	// there is no need to distinguish which syntax was written.
	for _, name := range re.SubexpNames() {
		if name != "" {
			return nil, &Offender{
				Shape: ShapeNamedGroup,
				Raw:   raw,
				Fix:   "remove the named capture group — key-link patterns must not use named groups (D-08)",
			}
		}
	}

	return re, nil
}

// CheckSatisfiable reports whether link's already-validated pattern re
// actually matches something, mirroring verify.cjs's own resolution
// order exactly (verify.cjs:1098-1119, read-only reference per D-01):
// try the from file first, fall back to the to file. A from file that
// does not exist yet is a promise from a plan that has not executed —
// not a broken gate — so it returns nil rather than an offender.
//
// This is only valid evidence about the JavaScript-side tool's behavior
// because D-08 already restricts patterns to the RE2 ∩ JavaScript common
// subset: a future contributor who loosens D-08 breaks this equivalence.
func CheckSatisfiable(link KeyLink, re *regexp.Regexp, repoRoot string) *Offender {
	fromPath := filepath.Join(repoRoot, link.From)
	fromContent, err := os.ReadFile(fromPath)
	switch {
	case err == nil:
		if re.Match(fromContent) {
			return nil
		}
	case os.IsNotExist(err):
		return nil
	}

	toPath := filepath.Join(repoRoot, link.To)
	if toContent, err := os.ReadFile(toPath); err == nil && re.Match(toContent) {
		return nil
	}

	return &Offender{
		File:  link.File,
		Line:  link.Line,
		Shape: ShapeUnsatisfiable,
		Raw:   link.Pattern,
		Fix:   fmt.Sprintf("pattern does not match %s or %s — update the pattern or the from/to paths", fromPath, toPath),
	}
}

// metaCharsToClass are the regex metacharacters SuggestCharClassForm
// rewrites into an escape-free character-class form, per D-02.
var metaCharsToClass = map[byte]bool{
	'.': true, '(': true, ')': true, '[': true, ']': true, '{': true,
	'}': true, '*': true, '+': true, '?': true, '^': true, '$': true,
	'|': true,
}

// SuggestCharClassForm mechanically derives D-02's escape-free form from
// raw. It first collapses a doubled backslash to a single one, since the
// observed corruption is a doubled escape. For each backslash followed
// by a regex metacharacter, it emits that metacharacter wrapped in a
// character class. A backslash followed by a double-quote character is
// handled separately: the correct fix drops the escape entirely, since a
// quote needs no escaping inside a regex (the shape at
// .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-03-PLAN.md:51).
// When no mechanical rewrite is derivable, it returns "".
func SuggestCharClassForm(raw string) string {
	collapsed := strings.ReplaceAll(raw, `\\`, `\`)

	var b strings.Builder
	rewritten := false
	for i := 0; i < len(collapsed); {
		c := collapsed[i]
		if c == '\\' && i+1 < len(collapsed) {
			next := collapsed[i+1]
			switch {
			case next == '"':
				b.WriteByte(next)
			case metaCharsToClass[next]:
				b.WriteByte('[')
				b.WriteByte(next)
				b.WriteByte(']')
			default:
				return ""
			}
			rewritten = true
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	if !rewritten {
		return ""
	}
	return b.String()
}

// ScanPlans is the single entry point both this phase's recurring gate
// tests and the one-time v0.13.x reassessment sweep call, so the two
// never drift into two matchers that disagree. It walks each root under
// repoRoot, selecting files whose base name ends in "-PLAN.md", and
// validates every key link found. Under ModeSatisfiability it also skips
// any plan that has not been executed yet (D-04): a plan file's sibling
// SUMMARY.md is what marks it executed, and an un-executed plan's
// key-links describe files it is about to create, not files that exist
// at HEAD. Offenders are sorted by file, then line, then shape before
// being returned, so two runs over the same inputs produce byte-
// identical output (D-07).
func ScanPlans(repoRoot string, roots []string, mode Mode) ([]Offender, error) {
	offenders, _, err := ScanPlansWithStats(repoRoot, roots, mode)
	return offenders, err
}

// ScanStats reports what a scan actually EXAMINED, as opposed to what it
// found. A gate that reports zero offenders because it inspected zero
// files is indistinguishable, from its return value alone, from a gate
// over a clean repo — and that silent no-op is the exact failure class
// this package exists to detect. Callers that gate CI on a clean result
// MUST assert these counts are nonzero; `ScanPlans` alone cannot give
// them that guarantee, which is why it is a thin wrapper over this.
type ScanStats struct {
	// PlanFiles counts *-PLAN.md files actually parsed, after every
	// mode-specific skip rule has been applied.
	PlanFiles int
	// KeyLinks counts the key_links entries read out of those files.
	// PlanFiles can be nonzero while this is zero (plans exist, none
	// declare a key link), so a gate wanting to prove it checked
	// something must look at this one, not just PlanFiles.
	KeyLinks int
}

// ScanPlansWithStats is ScanPlans plus the coverage counts needed to
// tell "found nothing wrong" apart from "looked at nothing".
func ScanPlansWithStats(repoRoot string, roots []string, mode Mode) ([]Offender, ScanStats, error) {
	var offenders []Offender
	var stats ScanStats

	for _, root := range roots {
		full := filepath.Join(repoRoot, root)
		if _, err := os.Stat(full); err != nil {
			return nil, ScanStats{}, fmt.Errorf("keylinks: scan %s: %w", full, err)
		}

		walkErr := filepath.Walk(full, func(p string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), "-PLAN.md") {
				return nil
			}
			if mode == ModeSatisfiability && !hasSiblingSummary(p) {
				return nil
			}

			links, perr := ParsePlanKeyLinks(p)
			if perr != nil {
				return fmt.Errorf("keylinks: scan %s: %w", p, perr)
			}
			stats.PlanFiles++
			stats.KeyLinks += len(links)

			for _, link := range links {
				re, off := ValidatePattern(link.Pattern)
				if off != nil {
					off.File = link.File
					off.Line = link.Line
					offenders = append(offenders, *off)
					continue
				}
				if mode == ModeSatisfiability {
					if sOff := CheckSatisfiable(link, re, repoRoot); sOff != nil {
						offenders = append(offenders, *sOff)
					}
				}
			}
			return nil
		})
		if walkErr != nil {
			return nil, ScanStats{}, fmt.Errorf("keylinks: scan %s: %w", full, walkErr)
		}
	}

	sort.Slice(offenders, func(i, j int) bool {
		if offenders[i].File != offenders[j].File {
			return offenders[i].File < offenders[j].File
		}
		if offenders[i].Line != offenders[j].Line {
			return offenders[i].Line < offenders[j].Line
		}
		return offenders[i].Shape < offenders[j].Shape
	})

	return offenders, stats, nil
}

// hasSiblingSummary reports whether planPath (a <dir>/<phase>-<NN>-PLAN.md
// file) has a sibling <dir>/<phase>-<NN>-SUMMARY.md — the executed-plan
// marker ModeSatisfiability's skip rule depends on.
func hasSiblingSummary(planPath string) bool {
	summaryPath := strings.TrimSuffix(planPath, "-PLAN.md") + "-SUMMARY.md"
	_, err := os.Stat(summaryPath)
	return err == nil
}

// OffenderLine renders one offender as a single deterministic line
// carrying file, 1-based line number, shape, the raw pattern, and the
// corrected form — modeled on internal/surfaces/conformance_test.go's
// violationLine. %q is used for the two pattern columns so an embedded
// quote or escape character is unambiguous in the failure output.
func OffenderLine(o Offender) string {
	fix := o.Fix
	if fix == "" {
		fix = "no mechanical rewrite — rewrite by hand"
	}
	return fmt.Sprintf("file=%s:%d shape=%s pattern=%q fix=%q", o.File, o.Line, o.Shape, o.Raw, fix)
}
