//go:build ignore

// Spike 004: rerank quality — vector order vs engram's lexical reranker
// (store.RerankHits) vs Jev relevance scores, over the #261 corpus.
//
//	go run main.go   # from this directory; writes results.json
//
// Env: ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, ENGRAM_EMBED_MODEL.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/seanb4t/engram/internal/embed"
	"github.com/seanb4t/engram/internal/store"
)

type doc struct{ key, content string; tags []string }

// Corpus copied verbatim from internal/retrievaleval/fixtures.go (gh261Case).
var corpus = []doc{
	{"record-t", "Run `task lint` before every commit; golangci-lint's config lives in .golangci.yaml and must stay clean.", []string{"lint", "task", "golangci-lint"}},
	{"distractor-01", "Run `task fmt` before committing; it applies gofmt, dprint, and yamlfmt.", []string{"fmt", "task"}},
	{"distractor-02", "The bare `task` target runs lint then test; CI invokes it directly.", []string{"task", "ci"}},
	{"distractor-03", "yamlfmt formatting is enforced via `task fmt`; a diff there fails CI.", []string{"yamlfmt", "task"}},
	{"distractor-04", "actionlint checks GitHub Actions workflow YAML; wired into `task lint`.", []string{"actionlint", "task"}},
	{"distractor-05", "rumdl lints Markdown files; part of the `task lint` aggregate target.", []string{"rumdl", "task"}},
	{"distractor-06", "`task test` runs `go test ./...`, including the Qdrant-backed integration suites.", []string{"test", "task"}},
	{"distractor-07", "`task proto:lint` runs buf lint against the proto/ tree before generation.", []string{"buf", "task"}},
	{"distractor-08", "CI's required test job runs `go test ./...` plus gofmt and go mod tidy checks.", []string{"ci", "test"}},
	{"distractor-09", "`task license:check` verifies every Go and Markdown file carries the Apache-2.0 SPDX header.", []string{"license", "task"}},
	{"distractor-10", "`task chart:lint` runs helm lint against charts/engram.", []string{"helm", "task"}},
	{"distractor-11", "`golangci-lint run ./...` is the single entrypoint `task lint` wraps for Go static analysis.", []string{"golangci-lint", "task"}},
	{"distractor-12", "`task bench` runs `go test -bench=. -benchmem ./...` for performance regressions.", []string{"bench", "task"}},
	{"distractor-13", "Prefer `task fmt` before `task lint` locally; CI runs them in that order too.", []string{"fmt", "lint", "task"}},
	{"distractor-14", "dprint formats non-Go files (JSON, TOML, Markdown) as part of `task fmt`.", []string{"dprint", "task"}},
	{"distractor-15", "`task proto:gen` regenerates the committed gen/ tree via buf; CI checks for drift.", []string{"buf", "task"}},
}

type query struct{ name, text, want, set string }

var queries = []query{
	// #261 regression queries (near-verbatim restatements of record-t).
	{"gh261-a", "Before committing, run `task lint`; the golangci-lint config is .golangci.yaml and it needs to stay clean.", "record-t", "gh261"},
	{"gh261-b", "Run `task lint` prior to every commit — golangci-lint's config file is .golangci.yaml and must remain clean.", "record-t", "gh261"},
	// Low-lexical-overlap paraphrases: one per record, avoiding its tool names where possible.
	{"para-t", "Which static analyzer has to pass before I commit, and where is its settings file?", "record-t", "para"},
	{"para-01", "How do I auto-format all the source before I commit?", "distractor-01", "para"},
	{"para-02", "What happens if I invoke the task runner with no target at all?", "distractor-02", "para"},
	{"para-03", "Can badly indented YAML break the build?", "distractor-03", "para"},
	{"para-04", "How are the GitHub workflow definitions validated?", "distractor-04", "para"},
	{"para-05", "What checks the documentation prose files for style problems?", "distractor-05", "para"},
	{"para-06", "How do I run the suites that need the vector database?", "distractor-06", "para"},
	{"para-07", "How is the protobuf schema checked for style before codegen?", "distractor-07", "para"},
	{"para-08", "What does the mandatory pipeline job verify besides tests?", "distractor-08", "para"},
	{"para-09", "How do I confirm every file carries the open-source licence notice?", "distractor-09", "para"},
	{"para-10", "How is the Kubernetes packaging validated?", "distractor-10", "para"},
	{"para-11", "What single command sits behind the Go static checks?", "distractor-11", "para"},
	{"para-12", "How would I catch a slowdown or allocation regression?", "distractor-12", "para"},
	{"para-13", "Should formatting or static checks run first on my machine?", "distractor-13", "para"},
	{"para-14", "What formats config files like JSON and TOML?", "distractor-14", "para"},
	{"para-15", "How do I rebuild the generated API stubs and what catches staleness?", "distractor-15", "para"},
}

func cosine(a, b []float32) float64 {
	var d, na, nb float64
	for i := range a {
		d += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return d / (math.Sqrt(na) * math.Sqrt(nb))
}

func rankOf(order []string, want string) int {
	for i, k := range order {
		if k == want {
			return i + 1
		}
	}
	return 0
}

type jevResult struct {
	order  []string
	scores map[string]float64
	ms     int64
	cost   float64
}

// jevRerank asks one Noul per candidate in a single request, then sorts by
// P(relevant), breaking ties by the incoming (vector) order.
func jevRerank(base, key, q string, cands []store.Memory) (jevResult, error) {
	state := map[string]any{"query": q}
	qs := map[string]any{}
	for i, m := range cands {
		ck := fmt.Sprintf("c%02d", i)
		state[ck] = m.Content
		qs[ck] = map[string]any{
			"type":         "noul",
			"instructions": fmt.Sprintf("Does memory record %s directly answer the query?", ck),
			"criteria": map[string]any{
				"true":  "The record directly answers what the query asks.",
				"false": "The record is about something else, or only shares vocabulary with the query.",
			},
		}
	}
	b, _ := json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "state": state, "questions": qs})
	req, _ := http.NewRequest("POST", base+"/alpha/decisions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return jevResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	ms := time.Since(start).Milliseconds()
	if resp.StatusCode != 200 {
		return jevResult{}, fmt.Errorf("%d %s", resp.StatusCode, body)
	}
	var r struct {
		Answers map[string]struct{ Noul float64 } `json:"answers"`
		Usage   struct{ Cost float64 }            `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return jevResult{}, err
	}
	idx := make([]int, len(cands))
	for i := range idx {
		idx[i] = i
	}
	score := func(i int) float64 { return r.Answers[fmt.Sprintf("c%02d", i)].Noul }
	sort.SliceStable(idx, func(a, b int) bool { return score(idx[a]) > score(idx[b]) })
	out := jevResult{scores: map[string]float64{}, ms: ms, cost: r.Usage.Cost}
	for _, i := range idx {
		out.order = append(out.order, cands[i].ID)
		out.scores[cands[i].ID] = score(i)
	}
	return out, nil
}

type metrics struct{ r1, r3, mrr float64; n int }

func (m *metrics) add(rank int) {
	m.n++
	if rank == 1 {
		m.r1++
	}
	if rank >= 1 && rank <= 3 {
		m.r3++
	}
	if rank > 0 {
		m.mrr += 1 / float64(rank)
	}
}

func (m metrics) String() string {
	n := float64(m.n)
	return fmt.Sprintf("R@1=%.2f R@3=%.2f MRR=%.3f (n=%d)", m.r1/n, m.r3/n, m.mrr/n, m.n)
}

func main() {
	ctx := context.Background()
	base := strings.TrimRight(os.Getenv("ENGRAM_OPENAI_BASE_URL"), "/")
	key := os.Getenv("ENGRAM_OPENAI_API_KEY")
	em := embed.New(base, key, os.Getenv("ENGRAM_EMBED_MODEL"))

	docVecs := map[string][]float32{}
	for _, d := range corpus {
		v, err := em.Embed(ctx, d.content)
		if err != nil {
			panic(fmt.Sprintf("embed %s: %v", d.key, err))
		}
		docVecs[d.key] = v
	}

	type row struct {
		Query, Set, Want               string
		VecRank, LexRank, JevRank      int
		JevWantScore, JevTopScore      float64
		JevTop                         string
		JevMs                          int64
	}
	var rows []row
	agg := map[string]*metrics{}
	for _, k := range []string{"vector/gh261", "lexical/gh261", "jev/gh261", "vector/para", "lexical/para", "jev/para"} {
		agg[k] = &metrics{}
	}
	var jevMs []int64
	var cost float64
	for _, q := range queries {
		qv, err := em.EmbedQuery(ctx, q.text)
		if err != nil {
			panic(err)
		}
		hits := make([]store.Memory, 0, len(corpus))
		for _, d := range corpus {
			hits = append(hits, store.Memory{ID: d.key, Content: d.content, Tags: d.tags, Score: float32(cosine(qv, docVecs[d.key]))})
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
		vecOrder := make([]string, len(hits))
		for i, h := range hits {
			vecOrder[i] = h.ID
		}
		lex := store.RerankHits(q.text, hits, len(hits))
		lexOrder := make([]string, len(lex))
		for i, h := range lex {
			lexOrder[i] = h.ID
		}
		jr, err := jevRerank(base, key, q.text, hits)
		if err != nil {
			panic(err)
		}
		jevMs = append(jevMs, jr.ms)
		cost += jr.cost
		r := row{Query: q.name, Set: q.set, Want: q.want,
			VecRank: rankOf(vecOrder, q.want), LexRank: rankOf(lexOrder, q.want), JevRank: rankOf(jr.order, q.want),
			JevWantScore: jr.scores[q.want], JevTop: jr.order[0], JevTopScore: jr.scores[jr.order[0]], JevMs: jr.ms}
		rows = append(rows, r)
		agg["vector/"+q.set].add(r.VecRank)
		agg["lexical/"+q.set].add(r.LexRank)
		agg["jev/"+q.set].add(r.JevRank)
	}

	fmt.Printf("%-9s %-14s %4s %4s %4s  %-14s %s\n", "query", "want", "vec", "lex", "jev", "jev top", "P(want)/P(top)")
	for _, r := range rows {
		fmt.Printf("%-9s %-14s %4d %4d %4d  %-14s %.2f/%.2f\n", r.Query, r.Want, r.VecRank, r.LexRank, r.JevRank, r.JevTop, r.JevWantScore, r.JevTopScore)
	}
	fmt.Println()
	for _, set := range []string{"gh261", "para"} {
		for _, m := range []string{"vector", "lexical", "jev"} {
			fmt.Printf("%-6s %-8s %s\n", set, m, agg[m+"/"+set])
		}
	}
	sort.Slice(jevMs, func(i, j int) bool { return jevMs[i] < jevMs[j] })
	fmt.Printf("\njev per-query latency (16 candidates): p50=%dms p90=%dms max=%dms · total cost $%.5f\n",
		jevMs[len(jevMs)/2], jevMs[len(jevMs)*9/10], jevMs[len(jevMs)-1], cost)

	f, _ := os.Create("results.json")
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}
