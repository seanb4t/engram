// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/seanb4t/engram/internal/decide"
)

// threeQuestionRequest builds a Request with one noul, one choice (3
// options) and one score (3 levels) question, plus synthetic state — the
// fixed shape TestJevRequestShape sends against every base-URL variant.
func threeQuestionRequest() decide.Request {
	return decide.Request{
		State: decide.State{"ticket": "synthetic ticket text"},
		Questions: map[string]decide.Question{
			"n": decide.Noul("is it true?", "yes it is", "no it isn't"),
			"c": decide.Choice("pick one", map[string]string{
				"opt1": "the first option",
				"opt2": "the second option",
				"opt3": "the third option",
			}),
			"s": decide.Score("rate it", []string{"low", "medium", "high"}),
		},
	}
}

// threeQuestionResponseBody is a canned 200 body answering every question
// name threeQuestionRequest asks, so Decide succeeds and TestJevRequestShape
// can focus on the captured path and encoded request body.
const threeQuestionResponseBody = `{"model":"typesafe/jev-1.13-20260917","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"opt1","probabilities":{"opt1":0.5,"opt2":0.3,"opt3":0.2},"confidence":0.8},"s":{"type":"score","score":1,"confidence":0.7,"probabilities":{"0":0.2,"1":0.6,"2":0.2},"legend":{"0":"a","1":"b","2":"c"}}},"usage":{"input_tokens":1,"output_tokens":1,"cost":0.0001},"id":"gen-test-shape","provider":"TypeSafe"}`

// captured records the last request path and decoded body a capture server
// observed, guarded by a mutex since httptest handlers run on their own
// goroutine.
type captured struct {
	mu   sync.Mutex
	path string
	body map[string]any
}

func (c *captured) set(path string, body map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path, c.body = path, body
}

func (c *captured) get() (string, map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.path, c.body
}

// newCaptureServer returns an httptest.Server that records the request path
// and decoded JSON body of every call, then responds with
// threeQuestionResponseBody.
func newCaptureServer(t *testing.T) (*httptest.Server, *captured) {
	t.Helper()
	rec := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if derr := json.NewDecoder(r.Body).Decode(&body); derr != nil {
			t.Errorf("capture server: decode request body: %v", derr)
		}
		rec.set(r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(threeQuestionResponseBody))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// TestJevRequestShape proves DEC-03 (both base-URL shapes reach
// /alpha/decisions with no doubled slash) and DEC-02's encode-side wire
// shape (top-level keys, criteria shapes, instructions, state).
func TestJevRequestShape(t *testing.T) {
	t.Run("openrouter base", func(t *testing.T) {
		srv, rec := newCaptureServer(t)
		c := New(srv.URL+"/api", "k", "m")
		noRetryDelay(c)
		if _, err := c.Decide(context.Background(), threeQuestionRequest()); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		path, _ := rec.get()
		if path != "/api/alpha/decisions" {
			t.Errorf("path = %q, want /api/alpha/decisions", path)
		}
	})

	t.Run("trailing slash", func(t *testing.T) {
		srv, rec := newCaptureServer(t)
		c := New(srv.URL+"/api/", "k", "m")
		noRetryDelay(c)
		if _, err := c.Decide(context.Background(), threeQuestionRequest()); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		path, _ := rec.get()
		if path != "/api/alpha/decisions" {
			t.Errorf("path = %q, want /api/alpha/decisions (no doubled slash)", path)
		}
	})

	t.Run("litellm base", func(t *testing.T) {
		srv, rec := newCaptureServer(t)
		c := New(srv.URL+"/openrouter", "k", "m")
		noRetryDelay(c)
		if _, err := c.Decide(context.Background(), threeQuestionRequest()); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		path, _ := rec.get()
		if path != "/openrouter/alpha/decisions" {
			t.Errorf("path = %q, want /openrouter/alpha/decisions", path)
		}
	})

	t.Run("body shape", func(t *testing.T) {
		srv, rec := newCaptureServer(t)
		c := New(srv.URL+"/api", "k", "m")
		noRetryDelay(c)
		req := threeQuestionRequest()
		if _, err := c.Decide(context.Background(), req); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		_, body := rec.get()
		if body == nil {
			t.Fatal("no request body captured")
		}

		if got, want := len(body), 3; got != want {
			t.Errorf("top-level key count = %d, want %d (keys: %v)", got, want, keysOf(body))
		}
		for _, k := range []string{"model", "state", "questions"} {
			if _, ok := body[k]; !ok {
				t.Errorf("top-level key %q missing", k)
			}
		}

		questions, ok := body["questions"].(map[string]any)
		if !ok {
			t.Fatalf("questions is %T, want map[string]any", body["questions"])
		}

		nQ, ok := questions["n"].(map[string]any)
		if !ok {
			t.Fatalf("questions.n is %T, want map[string]any", questions["n"])
		}
		nCriteria, ok := nQ["criteria"].(map[string]any)
		if !ok {
			t.Fatalf("questions.n.criteria is %T, want map[string]any (object)", nQ["criteria"])
		}
		if got, want := keysOf(nCriteria), []string{"false", "true"}; !reflect.DeepEqual(sorted(got), want) {
			t.Errorf("questions.n.criteria keys = %v, want exactly %v", got, want)
		}
		if _, ok := nQ["instructions"].(string); !ok {
			t.Errorf("questions.n.instructions is %T, want string", nQ["instructions"])
		}

		cQ, ok := questions["c"].(map[string]any)
		if !ok {
			t.Fatalf("questions.c is %T, want map[string]any", questions["c"])
		}
		cCriteria, ok := cQ["criteria"].(map[string]any)
		if !ok {
			t.Fatalf("questions.c.criteria is %T, want map[string]any (object)", cQ["criteria"])
		}
		wantOptions := map[string]string{
			"opt1": "the first option",
			"opt2": "the second option",
			"opt3": "the third option",
		}
		for k, v := range wantOptions {
			if got, _ := cCriteria[k].(string); got != v {
				t.Errorf("questions.c.criteria[%q] = %q, want %q", k, got, v)
			}
		}
		if len(cCriteria) != len(wantOptions) {
			t.Errorf("questions.c.criteria has %d entries, want %d", len(cCriteria), len(wantOptions))
		}
		if _, ok := cQ["instructions"].(string); !ok {
			t.Errorf("questions.c.instructions is %T, want string", cQ["instructions"])
		}

		sQ, ok := questions["s"].(map[string]any)
		if !ok {
			t.Fatalf("questions.s is %T, want map[string]any", questions["s"])
		}
		sCriteria, ok := sQ["criteria"].([]any)
		if !ok {
			t.Fatalf("questions.s.criteria is %T, want []any (array)", sQ["criteria"])
		}
		wantScale := []string{"low", "medium", "high"}
		if len(sCriteria) != len(wantScale) {
			t.Fatalf("questions.s.criteria has %d entries, want %d", len(sCriteria), len(wantScale))
		}
		for i, want := range wantScale {
			if got, _ := sCriteria[i].(string); got != want {
				t.Errorf("questions.s.criteria[%d] = %q, want %q", i, got, want)
			}
		}
		if _, ok := sQ["instructions"].(string); !ok {
			t.Errorf("questions.s.instructions is %T, want string", sQ["instructions"])
		}

		state, ok := body["state"].(map[string]any)
		if !ok {
			t.Fatalf("state is %T, want map[string]any", body["state"])
		}
		if got, want := state["ticket"], "synthetic ticket text"; got != want {
			t.Errorf("state.ticket = %v, want %q", got, want)
		}
		if len(state) != 1 {
			t.Errorf("state has %d entries, want 1", len(state))
		}
	})
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func sorted(ks []string) []string {
	out := append([]string(nil), ks...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// TestJevAnswerMapping proves DEC-02's decode-side wire mapping: every
// answer type decodes verbatim, malformed/missing/unknown answers are named
// errors, and unrequested answers are dropped.
func TestJevAnswerMapping(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		req := decide.Request{Questions: map[string]decide.Question{
			"relation": decide.Choice("how does b relate to a?", map[string]string{
				"duplicate":   "same record",
				"contradicts": "conflicting record",
				"related":     "related record",
				"unrelated":   "unrelated record",
			}),
			"same_subject": decide.Noul("same subject?", "yes", "no"),
		}}
		resp, err := decodeResponse([]byte(fixtureHappy), req)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}

		relation, ok := resp.Answers["relation"]
		if !ok {
			t.Fatal("relation answer missing")
		}
		if relation.Choice != "duplicate" {
			t.Errorf("relation.Choice = %q, want duplicate", relation.Choice)
		}
		if p := relation.Probabilities["duplicate"]; p < 0.87 || p > 0.97 {
			t.Errorf("relation.Probabilities[duplicate] = %v, want in [0.87, 0.97]", p)
		}
		if relation.Confidence == nil {
			t.Fatal("relation.Confidence is nil")
		}
		if c := *relation.Confidence; c < 0.85 || c > 0.95 {
			t.Errorf("relation.Confidence = %v, want in [0.85, 0.95]", c)
		}

		same, ok := resp.Answers["same_subject"]
		if !ok {
			t.Fatal("same_subject answer missing")
		}
		if p := same.Probability; p < 0.87 || p > 0.97 {
			t.Errorf("same_subject.Probability = %v, want in [0.87, 0.97]", p)
		}
	})

	t.Run("batch50", func(t *testing.T) {
		questions := make(map[string]decide.Question, 50)
		for i := range 50 {
			name := "rel_c" + pad2(i)
			questions[name] = decide.Noul("related?", "yes", "no")
		}
		req := decide.Request{Questions: questions}
		resp, err := decodeResponse([]byte(fixtureBatch50), req)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if len(resp.Answers) != 50 {
			t.Fatalf("len(Answers) = %d, want 50", len(resp.Answers))
		}
		for name := range questions {
			if _, ok := resp.Answers[name]; !ok {
				t.Errorf("answer %q missing", name)
			}
		}
	})

	t.Run("score", func(t *testing.T) {
		req := decide.Request{Questions: map[string]decide.Question{
			"score": decide.Score("how urgent?", []string{"can wait", "this week", "blocking"}),
		}}
		resp, err := decodeResponse([]byte(fixtureScore), req)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		a, ok := resp.Answers["score"]
		if !ok {
			t.Fatal("score answer missing")
		}
		if a.Score != 1 {
			t.Errorf("Score = %v, want 1", a.Score)
		}
		wantProbs := map[string]float64{"0": 0.1, "1": 0.8, "2": 0.1}
		if len(a.Probabilities) != len(wantProbs) {
			t.Fatalf("Probabilities has %d entries, want %d (got %v)", len(a.Probabilities), len(wantProbs), a.Probabilities)
		}
		for k, want := range wantProbs {
			if got := a.Probabilities[k]; got != want {
				t.Errorf("Probabilities[%q] = %v, want %v", k, got, want)
			}
		}
		if a.Confidence == nil {
			t.Fatal("Confidence is nil")
		} else if *a.Confidence != 0.8 {
			t.Errorf("Confidence = %v, want 0.8", *a.Confidence)
		}
		wantLegend := map[string]string{"0": `"can wait"`, "1": `"this week"`, "2": `"blocking"`}
		if len(a.Legend) != len(wantLegend) {
			t.Fatalf("Legend has %d entries, want %d (got %v)", len(a.Legend), len(wantLegend), a.Legend)
		}
		for k, want := range wantLegend {
			if got := string(a.Legend[k]); got != want {
				t.Errorf("Legend[%q] = %s, want %s", k, got, want)
			}
		}
	})

	t.Run("no renormalize", func(t *testing.T) {
		req := decide.Request{Questions: map[string]decide.Question{
			"c": decide.Choice("pick", map[string]string{"a": "option a", "b": "option b"}),
		}}
		resp, err := decodeResponse([]byte(fixtureChoiceSum098), req)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		a := resp.Answers["c"]
		sum := a.Probabilities["a"] + a.Probabilities["b"]
		if sum < 0.98-1e-9 || sum > 0.98+1e-9 {
			t.Errorf("probability sum = %v, want within 1e-9 of 0.98 (not renormalized)", sum)
		}
	})

	t.Run("absent optionals", func(t *testing.T) {
		t.Run("missing confidence gives nil", func(t *testing.T) {
			const body = `{"model":"typesafe/jev-1.13-20260917","answers":{"c":{"type":"choice","choice":"a","probabilities":{"a":0.6,"b":0.4}}},"usage":{"input_tokens":1,"output_tokens":1,"cost":0.00001},"id":"gen-fixture-noconf","provider":"TypeSafe"}`
			req := decide.Request{Questions: map[string]decide.Question{
				"c": decide.Choice("pick", map[string]string{"a": "option a", "b": "option b"}),
			}}
			resp, err := decodeResponse([]byte(body), req)
			if err != nil {
				t.Fatalf("decodeResponse: %v", err)
			}
			if resp.Answers["c"].Confidence != nil {
				t.Errorf("Confidence = %v, want nil", *resp.Answers["c"].Confidence)
			}
		})

		t.Run("fixtureNoulNoUsage gives a nil Usage", func(t *testing.T) {
			req := decide.Request{Questions: map[string]decide.Question{
				"q": decide.Noul("is it true?", "yes", "no"),
			}}
			resp, err := decodeResponse([]byte(fixtureNoulNoUsage), req)
			if err != nil {
				t.Fatalf("decodeResponse: %v", err)
			}
			if resp.Usage != nil {
				t.Errorf("Usage = %+v, want nil", resp.Usage)
			}
		})

		t.Run("usage without cost gives nil CostUSD", func(t *testing.T) {
			const body = `{"model":"typesafe/jev-1.13-20260917","answers":{"q":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1},"id":"gen-fixture-nocost","provider":"TypeSafe"}`
			req := decide.Request{Questions: map[string]decide.Question{
				"q": decide.Noul("is it true?", "yes", "no"),
			}}
			resp, err := decodeResponse([]byte(body), req)
			if err != nil {
				t.Fatalf("decodeResponse: %v", err)
			}
			if resp.Usage == nil {
				t.Fatal("Usage is nil, want non-nil with CostUSD nil")
			}
			if resp.Usage.CostUSD != nil {
				t.Errorf("CostUSD = %v, want nil", *resp.Usage.CostUSD)
			}
		})
	})

	t.Run("malformed", func(t *testing.T) {
		t.Run("non-JSON 200", func(t *testing.T) {
			req := decide.Request{Questions: map[string]decide.Question{
				"c": decide.Noul("is it true?", "yes", "no"),
			}}
			_, err := decodeResponse([]byte("not json"), req)
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Errorf("err = %v, want ErrDecisionMalformedResponse", err)
			}
		})

		t.Run("missing requested question", func(t *testing.T) {
			req := decide.Request{Questions: map[string]decide.Question{
				"c": decide.Noul("is it true?", "yes", "no"),
			}}
			_, err := decodeResponse([]byte(fixtureNoulOK), req)
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Errorf("err = %v, want ErrDecisionMalformedResponse", err)
			}
			var de *decide.Error
			if !errors.As(err, &de) || de.Question != "c" {
				t.Errorf("Question = %q, want c", de.Question)
			}
		})

		t.Run("unknown answer type", func(t *testing.T) {
			req := decide.Request{Questions: map[string]decide.Question{
				"c": decide.Choice("pick", map[string]string{"a": "option a"}),
			}}
			_, err := decodeResponse([]byte(fixtureUnknownAnswerType), req)
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Errorf("err = %v, want ErrDecisionMalformedResponse", err)
			}
			var de *decide.Error
			if !errors.As(err, &de) || de.Question != "c" {
				t.Errorf("Question = %q, want c", de.Question)
			}
		})

		t.Run("answer type does not match requested question type (WR-01)", func(t *testing.T) {
			req := decide.Request{Questions: map[string]decide.Question{
				"c": decide.Noul("is it true?", "yes", "no"),
			}}
			_, err := decodeResponse([]byte(fixtureTypeMismatch), req)
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Errorf("err = %v, want ErrDecisionMalformedResponse", err)
			}
			var de *decide.Error
			if !errors.As(err, &de) || de.Question != "c" {
				t.Errorf("Question = %q, want c", de.Question)
			}
		})
	})

	t.Run("extra ignored", func(t *testing.T) {
		resp, err := decodeResponse([]byte(fixtureNoulOK), decide.Request{Questions: map[string]decide.Question{}})
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if len(resp.Answers) != 0 {
			t.Errorf("Answers = %v, want empty (unrequested answer must be dropped)", resp.Answers)
		}
	})
}

// pad2 zero-pads i to 2 digits, matching the batch50 fixture's rel_cNN
// naming (rel_c00..rel_c49).
func pad2(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
