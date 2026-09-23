//go:build ignore

// Spike 001: probe Jev on OpenRouter's Decisions API.
//
//	go run main.go            # all probes, writes results.json
//
// Reads ENGRAM_OPENAI_BASE_URL (expects https://openrouter.ai/api) and
// ENGRAM_OPENAI_API_KEY. Throwaway code — not part of the module build.
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
	"time"
)

const model = "typesafe/jev-1.13"

type event struct {
	At       string         `json:"at"`
	Probe    string         `json:"probe"`
	Status   int            `json:"status"`
	Millis   int64          `json:"ms"`
	Bytes    int            `json:"bytes"`
	Body     string         `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

var (
	base   = strings.TrimRight(os.Getenv("ENGRAM_OPENAI_BASE_URL"), "/")
	key    = os.Getenv("ENGRAM_OPENAI_API_KEY")
	client = &http.Client{Timeout: 60 * time.Second}
	log    []event
)

func post(probe, path, apiKey string, body any, keepBody bool) (int, []byte, time.Duration) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	d := time.Since(start)
	if err != nil {
		log = append(log, event{At: time.Now().UTC().Format(time.RFC3339Nano), Probe: probe, Status: -1, Millis: d.Milliseconds(), Body: err.Error()})
		return -1, nil, d
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	d = time.Since(start)
	ev := event{At: time.Now().UTC().Format(time.RFC3339Nano), Probe: probe, Status: resp.StatusCode, Millis: d.Milliseconds(), Bytes: len(out)}
	if keepBody || resp.StatusCode != 200 {
		ev.Body = string(out)
		ev.Headers = map[string]string{}
		for _, h := range []string{"Content-Type", "X-Generation-Id", "Retry-After"} {
			if v := resp.Header.Get(h); v != "" {
				ev.Headers[h] = v
			}
		}
	}
	log = append(log, ev)
	return resp.StatusCode, out, d
}

func dedupRequest(a, b string) map[string]any {
	return map[string]any{
		"model": model,
		"state": map[string]any{"record_a": a, "record_b": b},
		"questions": map[string]any{
			"relation": map[string]any{
				"type":         "choice",
				"instructions": "How does record_b relate to record_a? Both are durable project-memory records for a coding agent.",
				"criteria": map[string]any{
					"duplicate":   "Both state the same fact; one is redundant.",
					"contradicts": "They make incompatible claims about the same subject; one corrects the other.",
					"related":     "Same subject area but different, compatible facts.",
					"unrelated":   "Different subjects.",
				},
			},
			"same_subject": map[string]any{
				"type":         "noul",
				"instructions": "Are both records about the same specific subject?",
				"criteria":     map[string]any{"true": "Same component, decision, or fact.", "false": "Different components or topics."},
			},
		},
	}
}

func pct(xs []int64, p float64) int64 {
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	i := int(p * float64(len(s)-1))
	return s[i]
}

func main() {
	if base == "" || key == "" {
		fmt.Fprintln(os.Stderr, "ENGRAM_OPENAI_BASE_URL / ENGRAM_OPENAI_API_KEY unset")
		os.Exit(1)
	}
	a := "internal/store tests are `package store`, so a harness importing store is usable only from `package store_test` files — import cycle otherwise."
	b := "The storetest harness can only be imported from external test packages (package store_test); importing it from package store creates an import cycle."
	c := "Commit signing is OFF since 2026-09-18; engram commits are intentionally unsigned."

	// 1. Happy path with full body.
	st, body, d := post("happy", "/alpha/decisions", key, dedupRequest(a, b), true)
	fmt.Printf("happy: %d in %v\n%s\n\n", st, d, body)

	// 2. Latency distribution: 30 sequential calls, alternating dup / unrelated pairs.
	var lat []int64
	for i := 0; i < 30; i++ {
		other := b
		if i%2 == 1 {
			other = c
		}
		st, out, d := post("latency", "/alpha/decisions", key, dedupRequest(a, other), i < 2)
		if st == 200 {
			lat = append(lat, d.Milliseconds())
		} else {
			fmt.Printf("latency call %d: %d %s\n", i, st, out)
		}
	}
	if len(lat) > 0 {
		fmt.Printf("latency n=%d p50=%dms p90=%dms p95=%dms max=%dms\n\n", len(lat), pct(lat, .5), pct(lat, .9), pct(lat, .95), pct(lat, 1))
	}

	// 3. Rerank-shaped batch: one query, 50 candidates as 50 noul questions in ONE request.
	qs := map[string]any{}
	state := map[string]any{"query": "why do red-evidence harness runs time out"}
	for i := 0; i < 50; i++ {
		k := fmt.Sprintf("c%02d", i)
		txt := c
		if i == 17 {
			txt = "The red-evidence harness costs ~4.5 s/patch; observed timeouts were machine contention, not harness cost."
		}
		state[k] = txt
		qs["rel_"+k] = map[string]any{
			"type":         "noul",
			"instructions": fmt.Sprintf("Does candidate %s answer the query?", k),
			"criteria":     map[string]any{"true": "It directly answers the query.", "false": "It does not."},
		}
	}
	st, body, d = post("batch50", "/alpha/decisions", key, map[string]any{"model": model, "state": state, "questions": qs}, true)
	fmt.Printf("batch50: %d in %v (%d bytes)\n", st, d, len(body))
	var br struct {
		Answers map[string]struct{ Noul float64 } `json:"answers"`
		Usage   map[string]any                    `json:"usage"`
	}
	_ = json.Unmarshal(body, &br)
	fmt.Printf("  rel_c17=%.3f rel_c00=%.3f usage=%v\n\n", br.Answers["rel_c17"].Noul, br.Answers["rel_c00"].Noul, br.Usage)

	// 4. Cardinality: 255 vs 256 choice options.
	for _, n := range []int{255, 256} {
		crit := map[string]any{}
		for i := 0; i < n; i++ {
			crit[fmt.Sprintf("opt%03d", i)] = fmt.Sprintf("Option number %d", i)
		}
		st, out, d := post(fmt.Sprintf("card%d", n), "/alpha/decisions", key, map[string]any{
			"model": model, "state": map[string]any{"x": "pick option number 42"},
			"questions": map[string]any{"q": map[string]any{"type": "choice", "instructions": "Which option?", "criteria": crit}},
		}, false)
		fmt.Printf("card%d: %d in %v %s\n", n, st, d, trunc(out))
	}

	// 5. Error shapes: bad key, bad question type, oversized state (> 32k tokens), chat path.
	st, out, _ := post("badkey", "/alpha/decisions", "sk-or-invalid", dedupRequest(a, b), true)
	fmt.Printf("\nbadkey: %d %s\n", st, trunc(out))
	bad := dedupRequest(a, b)
	bad["questions"] = map[string]any{"q": map[string]any{"type": "essay", "instructions": "x"}}
	st, out, _ = post("badtype", "/alpha/decisions", key, bad, true)
	fmt.Printf("badtype: %d %s\n", st, trunc(out))
	huge := dedupRequest(strings.Repeat("memory record content ", 40000), b)
	st, out, _ = post("oversize", "/alpha/decisions", key, huge, true)
	fmt.Printf("oversize: %d %s\n", st, trunc(out))
	st, out, _ = post("chatpath", "/v1/chat/completions", key, map[string]any{"model": model, "messages": []any{map[string]any{"role": "user", "content": "hi"}}}, true)
	fmt.Printf("chatpath: %d %s\n", st, trunc(out))

	// 6. Determinism: same request 5x, spread of the duplicate probability.
	var ps []float64
	for i := 0; i < 5; i++ {
		_, out, _ := post("determinism", "/alpha/decisions", key, dedupRequest(a, b), false)
		var r struct {
			Answers map[string]struct {
				Probabilities map[string]float64 `json:"probabilities"`
			} `json:"answers"`
		}
		_ = json.Unmarshal(out, &r)
		ps = append(ps, r.Answers["relation"].Probabilities["duplicate"])
	}
	fmt.Printf("\ndeterminism P(duplicate) x5: %v\n", ps)

	f, _ := os.Create("results.json")
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"events": log, "latency_ms": lat})
	fmt.Printf("\nwrote results.json (%d events)\n", len(log))
}

func trunc(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
