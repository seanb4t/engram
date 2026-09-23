//go:build ignore

// Spike 003: Jev relation verdicts on labeled spine record pairs.
//
//	go run main.go   # reads pairs.json, writes verdicts.json, prints metrics
//
// Env: ENGRAM_OPENAI_BASE_URL (https://openrouter.ai/api), ENGRAM_OPENAI_API_KEY.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const model = "typesafe/jev-1.13"

var labels = []string{"duplicate", "contradicts", "related", "unrelated"}

type pair struct {
	ID        string `json:"id"`
	Gold      string `json:"gold"`
	AID       string `json:"a_id"`
	BID       string `json:"b_id"`
	A         string `json:"a"`
	B         string `json:"b"`
	Synthetic bool   `json:"synthetic"`
	Ambiguous bool   `json:"ambiguous"`
	Rationale string `json:"rationale"`
	Source    string `json:"source"`
}

type verdict struct {
	pair
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	SameSubject   float64            `json:"same_subject"`
	Correct       bool               `json:"correct"`
	Millis        int64              `json:"ms"`
	Cost          float64            `json:"cost"`
	Error         string             `json:"error,omitempty"`
}

func request(p pair) map[string]any {
	return map[string]any{
		"model": model,
		"state": map[string]any{
			"context":  "Two durable project-memory records from a coding agent's memory store for one repository.",
			"record_a": p.A,
			"record_b": p.B,
		},
		"questions": map[string]any{
			"relation": map[string]any{
				"type":         "choice",
				"instructions": "How does record_b relate to record_a?",
				"criteria": map[string]any{
					"duplicate":   "Both state the same fact; keeping both is redundant (one may be more complete).",
					"contradicts": "They make incompatible claims about the same subject; one corrects or reverses the other.",
					"related":     "Same subject area, but different and compatible facts; both are worth keeping.",
					"unrelated":   "Different subjects.",
				},
			},
			"same_subject": map[string]any{
				"type":         "noul",
				"instructions": "Are both records about the same specific subject (same component, decision, or fact)?",
				"criteria":     map[string]any{"true": "Same specific subject.", "false": "Different subjects."},
			},
		},
	}
}

func call(c *http.Client, url, key string, p pair) verdict {
	v := verdict{pair: p}
	b, _ := json.Marshal(request(p))
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	v.Millis = time.Since(start).Milliseconds()
	if resp.StatusCode != 200 {
		v.Error = fmt.Sprintf("%d %s", resp.StatusCode, body)
		return v
	}
	var r struct {
		Answers struct {
			Relation struct {
				Choice        string             `json:"choice"`
				Confidence    float64            `json:"confidence"`
				Probabilities map[string]float64 `json:"probabilities"`
			} `json:"relation"`
			SameSubject struct {
				Noul float64 `json:"noul"`
			} `json:"same_subject"`
		} `json:"answers"`
		Usage struct {
			Cost float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		v.Error = err.Error()
		return v
	}
	v.Choice = r.Answers.Relation.Choice
	v.Confidence = r.Answers.Relation.Confidence
	v.Probabilities = r.Answers.Relation.Probabilities
	v.SameSubject = r.Answers.SameSubject.Noul
	v.Cost = r.Usage.Cost
	v.Correct = v.Choice == p.Gold
	return v
}

func main() {
	base := strings.TrimRight(os.Getenv("ENGRAM_OPENAI_BASE_URL"), "/")
	key := os.Getenv("ENGRAM_OPENAI_API_KEY")
	raw, err := os.ReadFile("pairs.json")
	if err != nil {
		panic(err)
	}
	var pairs []pair
	if err := json.Unmarshal(raw, &pairs); err != nil {
		panic(err)
	}
	c := &http.Client{Timeout: 60 * time.Second}
	out := make([]verdict, len(pairs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, p := range pairs {
		wg.Add(1)
		go func(i int, p pair) {
			defer wg.Done()
			sem <- struct{}{}
			out[i] = call(c, base+"/alpha/decisions", key, p)
			<-sem
		}(i, p)
	}
	wg.Wait()

	// Metrics over non-error, non-ambiguous pairs.
	conf := map[string]map[string]int{}
	for _, g := range labels {
		conf[g] = map[string]int{}
	}
	var n, correct int
	var brier, cost float64
	var lat []int64
	type pt struct {
		conf float64
		ok   bool
	}
	var pts []pt
	for _, v := range out {
		if v.Error != "" {
			fmt.Printf("ERROR %s: %s\n", v.ID, v.Error)
			continue
		}
		cost += v.Cost
		lat = append(lat, v.Millis)
		if v.Ambiguous {
			continue
		}
		n++
		conf[v.Gold][v.Choice]++
		if v.Correct {
			correct++
		}
		for _, l := range labels {
			y := 0.0
			if l == v.Gold {
				y = 1
			}
			brier += math.Pow(v.Probabilities[l]-y, 2)
		}
		pts = append(pts, pt{v.Probabilities[v.Choice], v.Correct})
	}
	fmt.Printf("pairs=%d scored=%d accuracy=%.2f brier=%.3f (4-class, 0=perfect, uniform=0.75) cost=$%.5f\n",
		len(out), n, float64(correct)/float64(n), brier/float64(n), cost)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	if len(lat) > 0 {
		fmt.Printf("latency p50=%dms p90=%dms max=%dms (concurrency 4)\n", lat[len(lat)/2], lat[len(lat)*9/10], lat[len(lat)-1])
	}
	fmt.Println("\nconfusion (rows=gold, cols=jev):")
	fmt.Printf("%-12s", "")
	for _, l := range labels {
		fmt.Printf("%12s", l)
	}
	fmt.Println()
	for _, g := range labels {
		fmt.Printf("%-12s", g)
		for _, l := range labels {
			fmt.Printf("%12d", conf[g][l])
		}
		fmt.Println()
	}
	fmt.Println("\nreliability (top-choice probability bucket → accuracy):")
	for _, lo := range []float64{0, .5, .7, .9} {
		hi := map[float64]float64{0: .5, .5: .7, .7: .9, .9: 1.01}[lo]
		var k, ok int
		for _, p := range pts {
			if p.conf >= lo && p.conf < hi {
				k++
				if p.ok {
					ok++
				}
			}
		}
		if k > 0 {
			fmt.Printf("  [%.1f,%.1f): n=%2d acc=%.2f\n", lo, math.Min(hi, 1), k, float64(ok)/float64(k))
		}
	}
	f, _ := os.Create("verdicts.json")
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	fmt.Println("\nwrote verdicts.json — open viewer.html")
}
