//go:build ignore

// Follow-up probes for spike 004: margin between top and runner-up, and
// no-answer queries (nothing in the corpus answers them).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

var corpus = []string{
	"Run `task lint` before every commit; golangci-lint's config lives in .golangci.yaml and must stay clean.",
	"Run `task fmt` before committing; it applies gofmt, dprint, and yamlfmt.",
	"The bare `task` target runs lint then test; CI invokes it directly.",
	"yamlfmt formatting is enforced via `task fmt`; a diff there fails CI.",
	"actionlint checks GitHub Actions workflow YAML; wired into `task lint`.",
	"rumdl lints Markdown files; part of the `task lint` aggregate target.",
	"`task test` runs `go test ./...`, including the Qdrant-backed integration suites.",
	"`task proto:lint` runs buf lint against the proto/ tree before generation.",
	"CI's required test job runs `go test ./...` plus gofmt and go mod tidy checks.",
	"`task license:check` verifies every Go and Markdown file carries the Apache-2.0 SPDX header.",
	"`task chart:lint` runs helm lint against charts/engram.",
	"`golangci-lint run ./...` is the single entrypoint `task lint` wraps for Go static analysis.",
	"`task bench` runs `go test -bench=. -benchmem ./...` for performance regressions.",
	"Prefer `task fmt` before `task lint` locally; CI runs them in that order too.",
	"dprint formats non-Go files (JSON, TOML, Markdown) as part of `task fmt`.",
	"`task proto:gen` regenerates the committed gen/ tree via buf; CI checks for drift.",
}

func scores(q string) []float64 {
	state := map[string]any{"query": q}
	qs := map[string]any{}
	for i, c := range corpus {
		k := fmt.Sprintf("c%02d", i)
		state[k] = c
		qs[k] = map[string]any{"type": "noul", "instructions": fmt.Sprintf("Does memory record %s directly answer the query?", k),
			"criteria": map[string]any{"true": "The record directly answers what the query asks.", "false": "The record is about something else, or only shares vocabulary with the query."}}
	}
	b, _ := json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "state": state, "questions": qs})
	req, _ := http.NewRequest("POST", strings.TrimRight(os.Getenv("ENGRAM_OPENAI_BASE_URL"), "/")+"/alpha/decisions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("ENGRAM_OPENAI_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct{ Answers map[string]struct{ Noul float64 } }
	_ = json.Unmarshal(body, &r)
	out := make([]float64, len(corpus))
	for i := range corpus {
		out[i] = r.Answers[fmt.Sprintf("c%02d", i)].Noul
	}
	return out
}

func main() {
	for _, q := range []struct{ label, text string }{
		{"no-answer", "How do I rotate the Qdrant API key in production?"},
		{"no-answer", "What embedding model does engram use by default?"},
		{"no-answer", "Who approves releases and how is the changelog written?"},
		{"multi-answer", "Which checks can fail CI?"},
		{"multi-answer", "What does `task lint` cover?"},
	} {
		s := scores(q.text)
		idx := make([]int, len(s))
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(a, b int) bool { return s[idx[a]] > s[idx[b]] })
		var top []string
		for _, i := range idx[:5] {
			top = append(top, fmt.Sprintf("c%02d=%.2f", i, s[i]))
		}
		above := 0
		for _, v := range s {
			if v >= 0.5 {
				above++
			}
		}
		fmt.Printf("%-12s %-58q top5 %s · #>=0.5: %d\n", q.label, q.text, strings.Join(top, " "), above)
	}
}
