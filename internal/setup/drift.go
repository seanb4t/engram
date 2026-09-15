// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file declares Phase 4's D-01 predicate and D-12 facet vocabulary
// ONCE, as pure functions over a shared OUTPUT type (Observation) —
// mirroring aggregate.go's single-declared-precedence-table discipline.
// Parsing is never shared: each runtime's own file (codex.go,
// claudecode.go) owns its own scanner end-to-end (the AUTHORED-HERE
// invariant, extended to probe OUTPUT by this phase, 04-RESEARCH.md
// Pitfall 3) — this file, and the executor that consults it (apply.go),
// stay content-blind to any one runtime's shape.
package setup

import (
	"net/url"
	"slices"
	"strings"
)

// Facet is the closed, typed vocabulary of ways an observed registration
// can differ from what the current Options would author (D-12). Every
// value is REAL and explicit — the zero value is a programming error,
// never meaningful data, mirroring SkillFormat's own discipline (plan.go).
type Facet string

const (
	// FacetURL means the observed endpoint URL differs from opts.URL.
	FacetURL Facet = "url"
	// FacetAuthMode means the observed auth mode (bearer vs. none/oauth)
	// differs from what opts.Auth would author.
	FacetAuthMode Facet = "auth-mode"
	// FacetHeaderName means a header is present or missing by NAME —
	// either an observed header setup was never asked to write (D-01
	// preserved cause), or a planned header the observation never saw.
	FacetHeaderName Facet = "header-name"
	// FacetHeaderValueRef means a header NAME setup planned is present
	// but its observed VALUE differs from what setup would write —
	// including Authorization when a bearer registration's observed
	// credential is not the runtime's own authored bearer form.
	FacetHeaderValueRef Facet = "header-value-ref"
	// FacetUnrecognizedContent means the observation's D-11 totality
	// parse found a field, key, or line the scanner's known vocabulary
	// does not account for — always a preserved cause (D-11 enforces D-01
	// structurally: a runtime release that adds a field makes setup
	// cautious, never blind).
	FacetUnrecognizedContent Facet = "unrecognized-content"
)

// facetOrder states D-12's fixed, stable rendering order for the Facet
// vocabulary. Declared once here so every consumer — Compare's own
// deduplication, joinFacets' string rendering — iterates the SAME slice
// and can never drift from each other or from this doc comment.
var facetOrder = []Facet{
	FacetURL,
	FacetAuthMode,
	FacetHeaderName,
	FacetHeaderValueRef,
	FacetUnrecognizedContent,
}

// redactedValue is the fixed placeholder every observed header/credential
// value renders as (D-02): unconditional, regardless of whether the
// observed value looks like a reference or a literal secret.
const redactedValue = "<redacted>"

// AuthState classifies what an observed registration's Authorization
// material looks like relative to the runtime's own authored bearer form.
// Every value is REAL and explicit; the zero value is a programming
// error.
type AuthState string

const (
	// AuthNone means no Authorization material was observed at all.
	AuthNone AuthState = "none"
	// AuthBearer means the observed Authorization material is EXACTLY the
	// runtime's own authored bearer form (Observation.BearerForm) —
	// setup's own vocabulary, reproducible from opts.Auth == "bearer".
	AuthBearer AuthState = "bearer"
	// AuthForeign means Authorization material was observed, but it is
	// NOT the runtime's own authored bearer form — setup did not author
	// it and cannot reproduce it. The observed VALUE is never carried
	// here or anywhere else (D-02); only this classification is.
	AuthForeign AuthState = "foreign"
)

// HeaderState classifies one observed-vs-planned header comparison
// (joinHeaders). Every value is REAL and explicit; the zero value is a
// programming error.
type HeaderState string

const (
	// HeaderMatches means a planned header's name was observed with the
	// SAME raw value setup would write for it (D-02: this is the ONLY
	// value comparison anywhere in this package).
	HeaderMatches HeaderState = "matches"
	// HeaderDiffers means a planned header's name was observed with a
	// DIFFERENT raw value.
	HeaderDiffers HeaderState = "differs"
	// HeaderMissing means a planned header's name was not observed at
	// all.
	HeaderMissing HeaderState = "missing"
	// HeaderUnplanned means an observed header's name matches nothing
	// the current Options plan to write — a D-01 preserved cause.
	HeaderUnplanned HeaderState = "unplanned"
)

// ObservedHeader is one header name's joined observed-vs-planned state.
// Planned is the non-secret reference (e.g. "${GATEWAY_KEY}") setup would
// write for this name, empty when State is HeaderUnplanned (nothing was
// planned for this name at all). The observed VALUE never appears here —
// only State, which is derived from it — mirroring AuthState's own
// discipline.
type ObservedHeader struct {
	Name    string
	State   HeaderState
	Planned string
}

// rawHeader is one header name/value pair as a runtime's own Observe
// implementation parses it directly out of probe output. It is
// TRANSIENT: it exists only as a local variable inside a runtime's own
// Observe frame and as joinHeaders' argument — its Value is compared for
// equality inside joinHeaders and then discarded. It is never assigned to
// any field of Observation, Drift, or Result (D-02/D-03 by construction).
type rawHeader struct {
	Name, Value string
}

// plannedHeader is one header name and the non-secret rendered reference
// setup would write for it — never a secret, since Options.Headers itself
// never carries one (runtime.go's HeaderSpec doc comment).
type plannedHeader struct {
	Name, Value string
}

// sortedPlannedHeaders returns a clone of phs ordered by
// strings.ToLower(Name) ascending, ties broken by an exact byte-wise
// strings.Compare(Name) — the SAME total order runtime.go's sortedHeaders
// already establishes for Options.Headers, reimplemented here because
// plannedHeader (this file's own transient shape) is not a HeaderSpec.
func sortedPlannedHeaders(phs []plannedHeader) []plannedHeader {
	out := slices.Clone(phs)
	slices.SortFunc(out, func(a, b plannedHeader) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// sortObservedHeaders returns a clone of hs ordered by the SAME total
// order sortedPlannedHeaders uses — case-insensitive by Name, ties broken
// byte-wise — so joinHeaders' result never depends on the observed or
// planned side's original ordering.
func sortObservedHeaders(hs []ObservedHeader) []ObservedHeader {
	out := slices.Clone(hs)
	slices.SortFunc(out, func(a, b ObservedHeader) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// joinHeaders joins observed against planned by NAME, case-insensitively
// (strings.EqualFold — Phase 2's D-02/D-08 case-insensitive header-name
// vocabulary), and returns one ObservedHeader per name that appears on
// EITHER side. For each planned entry (visited in sortedPlannedHeaders'
// order), the FIRST unconsumed observed entry sharing its name (case-
// insensitively) is matched: HeaderMatches when the raw values are BYTE-
// IDENTICAL (the ONLY value comparison anywhere in this package, D-02),
// else HeaderDiffers; a planned entry with no observed match is
// HeaderMissing. Every observed entry NOT consumed by a planned name —
// including a second observed entry sharing an already-matched name — is
// HeaderUnplanned. The result is sorted by sortObservedHeaders, so it
// never depends on either input's original order.
func joinHeaders(observed []rawHeader, planned []plannedHeader) []ObservedHeader {
	sortedPlanned := sortedPlannedHeaders(planned)
	consumed := make([]bool, len(observed))
	var out []ObservedHeader
	for _, p := range sortedPlanned {
		matched := -1
		for i, o := range observed {
			if consumed[i] {
				continue
			}
			if strings.EqualFold(o.Name, p.Name) {
				matched = i
				break
			}
		}
		if matched == -1 {
			out = append(out, ObservedHeader{Name: p.Name, State: HeaderMissing, Planned: p.Value})
			continue
		}
		consumed[matched] = true
		if observed[matched].Value == p.Value {
			out = append(out, ObservedHeader{Name: p.Name, State: HeaderMatches, Planned: p.Value})
		} else {
			out = append(out, ObservedHeader{Name: p.Name, State: HeaderDiffers, Planned: p.Value})
		}
	}
	for i, o := range observed {
		if consumed[i] {
			continue
		}
		out = append(out, ObservedHeader{Name: o.Name, State: HeaderUnplanned})
	}
	return sortObservedHeaders(out)
}

// Observation is one runtime's parsed-and-redacted view of its own
// existing engram registration, as read back through that runtime's own
// read verb (D-01). It is the SHARED output type every runtime's Observe
// implementation produces — never a shared PARSING function (04-RESEARCH.md
// Pitfall 3) — and the ONLY thing Compare consumes. URL is carried in
// full: a URL is not a secret (setup's own D-02, never normalized).
// BearerForm is the runtime's own authored bearer form as its read verb
// renders it (e.g. codex's "ENGRAM_TOKEN") — never a credential value.
// Headers carries no observed VALUE anywhere (D-02/D-03: values are
// compared and discarded inside the runtime's own Observe frame, never
// copied here). Unrecognized carries D-11's totality-parse labels/keys
// only — already bounded and quoteWord'ed by the observing runtime, never
// a raw probe fragment. WholeEntryNote is a fixed, runtime-authored
// sentence (e.g. codex's remove-or-add, never-merge semantics) appended
// to a preserved row's Reason.
type Observation struct {
	URL            string
	Auth           AuthState
	BearerForm     string
	Headers        []ObservedHeader
	Unrecognized   []string
	WholeEntryNote string
}

// DriftRuntime is an OPTIONAL interface a Runtime may implement to
// declare that it can parse its own existing registration state back out
// of its own read verb's output — the exact PluginRuntime precedent
// (plugin.go), type-asserted exactly once, at this capability's entry
// point (apply.go's execute), never a by-name branch. Only claude-code
// and codex implement it (D-10): opencode authors no scanner because its
// only read verb (`mcp list`) prints a table this design already declines
// to parse (03-CONTEXT.md D-11), and generic has no probe at all.
//
// Observe parses probeOutput — the SAME combined stdout+stderr capture
// execute() already made from Plan.Probe, regardless of the probe's exit
// code (D-11: engram reports rather than diagnoses) — comparing any
// observed header value against what THIS runtime's own Plan() would
// write for the same name under opts, INSIDE this call (the raw value
// never survives past it, D-02/D-03). ok is false ONLY when probeOutput
// cannot be framed as a registration AT ALL (empty/unparseable body, or a
// document that is plainly not this registration) — never for
// unrecognized CONTENT within an otherwise-framed registration, which is
// reported on Observation.Unrecognized instead (D-11).
type DriftRuntime interface {
	Observe(probeOutput string, opts Options) (Observation, bool)
}

// Drift is the result of comparing one Observation against Options: the
// classified Outcome, the differing Facets (deduplicated, ordered per
// facetOrder), Details (one fixed-composition line per differing facet,
// in emission order — never string-matched from probe content), and
// Preserved (the subset of Details that are D-01 preserved causes, same
// order — always non-empty when Outcome is OutcomePreserved, D-09).
type Drift struct {
	Outcome   Outcome
	Facets    []Facet
	Details   []string
	Preserved []string
}

// orderedFacets returns the distinct values of fs, ordered per
// facetOrder — the single place both Compare (building Drift.Facets) and
// joinFacets (rendering a string) derive that order from, so neither can
// drift from facetOrder's own declaration.
func orderedFacets(fs []Facet) []Facet {
	if len(fs) == 0 {
		return nil
	}
	present := make(map[Facet]bool, len(fs))
	for _, f := range fs {
		present[f] = true
	}
	var out []Facet
	for _, f := range facetOrder {
		if present[f] {
			out = append(out, f)
		}
	}
	return out
}

// joinFacets renders fs as a comma-joined string in facetOrder — D-12's
// fixed stable order, regardless of fs' own order or duplicates. Returns
// "" for an empty or nil fs.
func joinFacets(fs []Facet) string {
	ordered := orderedFacets(fs)
	if len(ordered) == 0 {
		return ""
	}
	strs := make([]string, len(ordered))
	for i, f := range ordered {
		strs[i] = string(f)
	}
	return strings.Join(strs, ",")
}

// displayURL renders s for display: when s parses as a URL carrying
// userinfo (a literal password embedded in the URL itself, T-04-05), the
// password is masked via the stdlib's own url.URL.Redacted() — never a
// hand-rolled regex. Any other s, including one that fails to parse at
// all, renders unchanged: a URL is not a secret (D-02), so the common
// case never gains a rendering step.
func displayURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	return u.Redacted()
}

// notComparedNote composes the fixed "<name>: not compared: <why>" shape
// Drift's absence carries when a comparison was not possible at all
// (D-09: ambiguity of every kind resolves to would-write, with a note
// that quotes no probe bytes) — never a probe fragment, always a typed
// reason string the caller already composed.
func notComparedNote(name, why string) string {
	return name + ": not compared: " + why
}

// renderObservation rebuilds Result.Registered from obs — D-03's
// redaction-by-construction rendering, replacing the old raw probe
// capture entirely. Every observed-PRESENT header (HeaderMatches,
// HeaderDiffers, HeaderUnplanned — never HeaderMissing, which was never
// actually observed) renders as "name=<redacted>", comma-joined, or
// "none" when there are none; a non-empty Unrecognized list appends
// " unrecognized=k1,k2" naming only the D-11 labels/keys, never a probe
// fragment.
func renderObservation(obs Observation) string {
	var headerParts []string
	for _, h := range obs.Headers {
		if h.State == HeaderMissing {
			continue
		}
		headerParts = append(headerParts, quoteWord(h.Name)+"="+redactedValue)
	}
	headersStr := "none"
	if len(headerParts) > 0 {
		headersStr = strings.Join(headerParts, ",")
	}
	s := "url=" + quoteWord(displayURL(obs.URL)) + " auth=" + string(obs.Auth) + " headers=" + headersStr
	if len(obs.Unrecognized) > 0 {
		s += " unrecognized=" + strings.Join(obs.Unrecognized, ",")
	}
	return s
}

// Compare is THE D-01 predicate, declared once, pure: given obs (a
// runtime's parsed-and-redacted Observation) and opts (the already-known
// intent), it decides preserved/would-write/already-correct and names
// every differing facet, in this fixed order:
//
//  1. URL: obs.URL != opts.URL -> FacetURL.
//  2. Auth mode: comparing plannedBearer := opts.Auth == "bearer" against
//     obs.Auth. plannedBearer && AuthNone -> FacetAuthMode ("observed
//     none, would write bearer"). !plannedBearer && AuthBearer ->
//     FacetAuthMode ("observed bearer, would write <opts.Auth>").
//     plannedBearer && AuthForeign -> FacetHeaderValueRef (setup would
//     overwrite a foreign Authorization with its own bearer form — a
//     reproducible difference, not a preserved cause). !plannedBearer &&
//     AuthForeign -> a PRESERVED cause + FacetHeaderName (setup never
//     planned any Authorization at all, so an observed one is something
//     it cannot re-create). AuthNone paired with !plannedBearer, and
//     AuthBearer paired with plannedBearer, contribute nothing — the
//     observed and planned auth shapes already agree.
//  3. Each obs.Headers entry, in order: HeaderMatches contributes
//     nothing; HeaderDiffers -> FacetHeaderValueRef; HeaderMissing ->
//     FacetHeaderName (a REPRODUCIBLE difference: setup would simply add
//     it); HeaderUnplanned -> a PRESERVED cause + FacetHeaderName (setup
//     never planned this name at all).
//  4. obs.Unrecognized, when non-empty -> a PRESERVED cause +
//     FacetUnrecognizedContent, one line naming every label (D-11).
//
// Outcome: any preserved cause -> OutcomePreserved; else any facet at all
// -> OutcomeWouldWrite; else OutcomeAlreadyCorrect (D-01). Facets is
// deduplicated and ordered via orderedFacets; Details keeps the emission
// order above; Preserved is the subset of Details that are preserved
// causes, same order — never empty when Outcome is OutcomePreserved
// (D-09).
func Compare(obs Observation, opts Options) Drift {
	var facets []Facet
	var details []string
	var preserved []string

	if obs.URL != opts.URL {
		facets = append(facets, FacetURL)
		details = append(details, "url: observed "+quoteWord(displayURL(obs.URL))+", would write "+quoteWord(opts.URL))
	}

	plannedBearer := opts.Auth == "bearer"
	switch {
	case plannedBearer && obs.Auth == AuthNone:
		facets = append(facets, FacetAuthMode)
		details = append(details, "auth-mode: observed none, would write bearer")
	case !plannedBearer && obs.Auth == AuthBearer:
		facets = append(facets, FacetAuthMode)
		details = append(details, "auth-mode: observed bearer, would write "+opts.Auth)
	case plannedBearer && obs.Auth == AuthForeign:
		facets = append(facets, FacetHeaderValueRef)
		details = append(details, "Authorization: observed "+redactedValue+", would write "+obs.BearerForm)
	case !plannedBearer && obs.Auth == AuthForeign:
		facets = append(facets, FacetHeaderName)
		line := "Authorization: observed " + redactedValue + ", not authored by setup"
		details = append(details, line)
		preserved = append(preserved, line)
	}

	for _, h := range obs.Headers {
		switch h.State {
		case HeaderMatches:
			// Nothing to report.
		case HeaderDiffers:
			facets = append(facets, FacetHeaderValueRef)
			details = append(details, h.Name+": observed "+redactedValue+", would write "+h.Planned)
		case HeaderMissing:
			facets = append(facets, FacetHeaderName)
			details = append(details, h.Name+": not registered, would write "+h.Planned)
		case HeaderUnplanned:
			facets = append(facets, FacetHeaderName)
			line := h.Name + ": observed " + redactedValue + ", not authored by setup"
			details = append(details, line)
			preserved = append(preserved, line)
		}
	}

	if len(obs.Unrecognized) > 0 {
		facets = append(facets, FacetUnrecognizedContent)
		line := "unrecognized-content: " + strings.Join(obs.Unrecognized, ", ")
		details = append(details, line)
		preserved = append(preserved, line)
	}

	outcome := OutcomeAlreadyCorrect
	switch {
	case len(preserved) > 0:
		outcome = OutcomePreserved
	case len(facets) > 0:
		outcome = OutcomeWouldWrite
	}

	return Drift{
		Outcome:   outcome,
		Facets:    orderedFacets(facets),
		Details:   details,
		Preserved: preserved,
	}
}
