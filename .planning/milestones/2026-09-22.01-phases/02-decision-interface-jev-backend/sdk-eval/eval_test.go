package sdkeval

// Behavioral evaluation harness for DEC-05 (plan 02-02). Replays the spike's
// captured OpenRouter and LiteLLM bodies, plus synthetic fixtures, through
// openrouter.Alpha.Decisions.Create against httptest servers and fake
// RoundTrippers, and applies the D-06 decision table. Every request state
// built here is synthetic text — never copied from the spike's real spine
// records.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
	"github.com/OpenRouterTeam/go-sdk/models/sdkerrors"
	"github.com/OpenRouterTeam/go-sdk/retry"
)

// criteriaPtr builds a *components.Criteria from synthetic guidance text.
func criteriaPtr(s string) *components.Criteria {
	c := components.CreateCriteriaStr(s)
	return &c
}

// buildTestRequest builds one noul, one choice and one score question over a
// fully synthetic state — never text derived from the spike's real records.
func buildTestRequest() components.DecisionsRequest {
	return components.DecisionsRequest{
		Model: "typesafe/jev-1.13",
		State: components.CreateStateStr("synthetic-state: widget alpha vs synthetic-state: widget beta"),
		Questions: map[string]components.Questions{
			"relation": components.CreateQuestionsChoice(components.DecisionsChoiceQuestion{
				Criteria: map[string]*components.Criteria{
					"duplicate": criteriaPtr("the two synthetic states describe the same fact"),
					"unrelated": criteriaPtr("the two synthetic states describe different facts"),
				},
				Instructions: components.CreateDecisionsChoiceQuestionInstructionsStr("How does synthetic-state B relate to synthetic-state A?"),
			}),
			"same_subject": components.CreateQuestionsNoul(components.DecisionsNoulQuestion{
				Criteria: &components.DecisionsNoulQuestionCriteria{
					True:  components.CreateTrueStr("the two synthetic states concern the same subject"),
					False: components.CreateFalseStr("the two synthetic states concern different subjects"),
				},
				Instructions: components.CreateDecisionsNoulQuestionInstructionsStr("Do the synthetic states share a subject?"),
			}),
			"urgency": components.CreateQuestionsScore(components.DecisionsScoreQuestion{
				Criteria: []components.Criterion{
					components.CreateCriterionStr("can wait"),
					components.CreateCriterionStr("this week"),
					components.CreateCriterionStr("blocking"),
				},
				Instructions: components.CreateDecisionsScoreQuestionInstructionsStr("How urgent is the synthetic finding?"),
			}),
		},
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// countingRoundTripper counts every RoundTrip call and delegates to base
// (defaulting to http.DefaultTransport).
type countingRoundTripper struct {
	count atomic.Int64
	base  http.RoundTripper
}

func (c *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.count.Add(1)
	base := c.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// errAfterReader returns data, then err once data is exhausted.
type errAfterReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *errAfterReader) Close() error { return nil }

// countingReadCloser counts every byte Read returns.
type countingReadCloser struct {
	r io.Reader
	n *int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.n += int64(n)
	return n, err
}

func (c *countingReadCloser) Close() error { return nil }

// jsonKind classifies a decoded JSON value's Go type for E02's shape checks.
func jsonKind(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case bool:
		return "bool"
	case float64:
		return "number"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// classifyDecisionError reports whether the HTTP status is derivable from an
// error Decisions.Create returned, either via *sdkerrors.APIError.StatusCode
// or via one of the SDK's per-status named error types.
func classifyDecisionError(err error) (status int, isAPIError bool, isNamedType bool, typeName string) {
	typeName = fmt.Sprintf("%T", err)

	var apiErr *sdkerrors.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode, true, false, typeName
	}

	switch err.(type) {
	case *sdkerrors.BadRequestResponseError:
		return 400, false, true, typeName
	case *sdkerrors.UnauthorizedResponseError:
		return 401, false, true, typeName
	case *sdkerrors.PaymentRequiredResponseError:
		return 402, false, true, typeName
	case *sdkerrors.ForbiddenResponseError:
		return 403, false, true, typeName
	case *sdkerrors.NotFoundResponseError:
		return 404, false, true, typeName
	case *sdkerrors.PayloadTooLargeResponseError:
		return 413, false, true, typeName
	case *sdkerrors.TooManyRequestsResponseError:
		return 429, false, true, typeName
	case *sdkerrors.InternalServerResponseError:
		return 500, false, true, typeName
	case *sdkerrors.BadGatewayResponseError:
		return 502, false, true, typeName
	case *sdkerrors.ServiceUnavailableResponseError:
		return 503, false, true, typeName
	case *sdkerrors.EdgeNetworkTimeoutResponseError:
		return 524, false, true, typeName
	case *sdkerrors.ProviderOverloadedResponseError:
		return 529, false, true, typeName
	}
	return 0, false, false, typeName
}

func TestSDKEvaluation(t *testing.T) {
	t.Run("E01_path_fidelity", func(t *testing.T) {
		var mu sync.Mutex
		var observedPath string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			observedPath = r.URL.Path
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureHappy))
		}))
		defer srv.Close()

		client := openrouter.New(openrouter.WithSecurity("test-key"))

		tryBase := func(base, want string) (matchedVia string, ok bool, attempts []string) {
			candidates := []string{base}
			if trimmed := strings.TrimSuffix(base, "/api"); trimmed != base {
				candidates = append(candidates, trimmed)
			}
			for _, c := range candidates {
				mu.Lock()
				observedPath = ""
				mu.Unlock()
				if _, err := client.Alpha.Decisions.Create(context.Background(), buildTestRequest(), operations.WithServerURL(c)); err != nil {
					t.Fatalf("harness fault: Create against %q: %v", c, err)
				}
				mu.Lock()
				got := observedPath
				mu.Unlock()
				attempts = append(attempts, fmt.Sprintf("WithServerURL(%q)->path=%q", c, got))
				if got == want {
					return c, true, attempts
				}
			}
			return "", false, attempts
		}

		b1 := srv.URL + "/api"
		b2 := srv.URL + "/openrouter"

		via1, ok1, attempts1 := tryBase(b1, "/api/alpha/decisions")
		via2, ok2, attempts2 := tryBase(b2, "/openrouter/alpha/decisions")

		pass := ok1 && ok2
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		obs := fmt.Sprintf("B1(OpenRouter shape, base=%q, want=/api/alpha/decisions)=%v via=%q attempts=[%s]; B2(LiteLLM shape, base=%q, want=/openrouter/alpha/decisions)=%v via=%q attempts=[%s]",
			b1, ok1, via1, strings.Join(attempts1, ", "), b2, ok2, via2, strings.Join(attempts2, ", "))
		t.Logf("EVAL-E01 %s %s", status, obs)
	})

	t.Run("E02_request_fidelity", func(t *testing.T) {
		var capturedBody []byte

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("harness fault: reading captured request body: %v", err)
			}
			capturedBody = b
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureHappy))
		}))
		defer srv.Close()

		client := openrouter.New(openrouter.WithSecurity("test-key"))
		if _, err := client.Alpha.Decisions.Create(context.Background(), buildTestRequest(), operations.WithServerURL(srv.URL)); err != nil {
			t.Fatalf("harness fault: Create: %v", err)
		}

		var decoded map[string]any
		if err := json.Unmarshal(capturedBody, &decoded); err != nil {
			t.Fatalf("harness fault: captured request body not JSON: %v", err)
		}

		wantKeys := map[string]bool{"model": true, "state": true, "questions": true}
		var extraKeys []string
		for k := range decoded {
			if !wantKeys[k] {
				extraKeys = append(extraKeys, k)
			}
		}

		questions, _ := decoded["questions"].(map[string]any)
		noulQ, _ := questions["same_subject"].(map[string]any)
		noulCriteria, _ := noulQ["criteria"].(map[string]any)
		_, hasTrue := noulCriteria["true"]
		_, hasFalse := noulCriteria["false"]
		noulInstrKind := jsonKind(noulQ["instructions"])

		choiceQ, _ := questions["relation"].(map[string]any)
		choiceCriteriaKind := jsonKind(choiceQ["criteria"])
		choiceInstrKind := jsonKind(choiceQ["instructions"])

		scoreQ, _ := questions["urgency"].(map[string]any)
		scoreCriteriaKind := jsonKind(scoreQ["criteria"])
		scoreInstrKind := jsonKind(scoreQ["instructions"])

		pass := len(extraKeys) == 0 &&
			hasTrue && hasFalse &&
			choiceCriteriaKind == "object" &&
			scoreCriteriaKind == "array" &&
			noulInstrKind == "string" && choiceInstrKind == "string" && scoreInstrKind == "string"

		status := "FAIL"
		if pass {
			status = "PASS"
		}
		obs := fmt.Sprintf("top_level_keys_extra=%v noul_criteria_has_true=%v noul_criteria_has_false=%v noul_instructions_kind=%s choice_criteria_kind=%s choice_instructions_kind=%s score_criteria_kind=%s score_instructions_kind=%s",
			extraKeys, hasTrue, hasFalse, noulInstrKind, choiceCriteriaKind, choiceInstrKind, scoreCriteriaKind, scoreInstrKind)
		t.Logf("EVAL-E02 %s %s", status, obs)
	})

	t.Run("E03_typed_decode", func(t *testing.T) {
		var failures []string

		// fixtureHappy: choice + noul answers, usage, model, id, provider.
		var happy components.DecisionsResponse
		if err := json.Unmarshal([]byte(fixtureHappy), &happy); err != nil {
			t.Fatalf("harness fault: fixtureHappy did not decode: %v", err)
		}
		if rel, ok := happy.Answers["relation"]; !ok || rel.DecisionsChoiceAnswer == nil {
			failures = append(failures, "happy: answers.relation not reachable as a typed choice answer")
		} else {
			ca := rel.DecisionsChoiceAnswer
			if ca.Choice == "" {
				failures = append(failures, "happy: choice.Choice empty")
			}
			if ca.Confidence == nil {
				failures = append(failures, "happy: choice.Confidence nil")
			}
			if len(ca.Probabilities) == 0 {
				failures = append(failures, "happy: choice.Probabilities empty")
			}
		}
		if ss, ok := happy.Answers["same_subject"]; !ok || ss.DecisionsNoulAnswer == nil {
			failures = append(failures, "happy: answers.same_subject not reachable as a typed noul answer")
		}
		if happy.Usage.InputTokens == 0 || happy.Usage.OutputTokens == 0 {
			failures = append(failures, "happy: usage input/output tokens not reachable")
		}
		if happy.Usage.Cost == nil {
			failures = append(failures, "happy: usage.Cost nil")
		}
		if happy.Model == "" {
			failures = append(failures, "happy: Model empty")
		}
		if happy.ID == nil {
			failures = append(failures, "happy: ID nil")
		}
		if happy.Provider == nil {
			failures = append(failures, "happy: Provider nil")
		}
		for name, a := range happy.Answers {
			if a.IsUnknown() {
				failures = append(failures, fmt.Sprintf("happy: answer %q decoded as UnknownRaw", name))
			}
		}

		// fixtureBatch50: 50 typed noul answers, none unknown.
		var batch components.DecisionsResponse
		if err := json.Unmarshal([]byte(fixtureBatch50), &batch); err != nil {
			t.Fatalf("harness fault: fixtureBatch50 did not decode: %v", err)
		}
		if len(batch.Answers) != 50 {
			failures = append(failures, fmt.Sprintf("batch50: got %d answers, want 50", len(batch.Answers)))
		}
		for name, a := range batch.Answers {
			if a.DecisionsNoulAnswer == nil {
				failures = append(failures, fmt.Sprintf("batch50: answer %q not reachable as a typed noul answer", name))
			}
			if a.IsUnknown() {
				failures = append(failures, fmt.Sprintf("batch50: answer %q decoded as UnknownRaw", name))
			}
		}

		// fixtureScore: score answer, including the Legend union members.
		var score components.DecisionsResponse
		if err := json.Unmarshal([]byte(fixtureScore), &score); err != nil {
			t.Fatalf("harness fault: fixtureScore did not decode: %v", err)
		}
		if ans, ok := score.Answers["urgency"]; !ok || ans.DecisionsScoreAnswer == nil {
			failures = append(failures, "score: answers.urgency not reachable as a typed score answer")
		} else {
			sa := ans.DecisionsScoreAnswer
			if sa.Confidence == nil {
				failures = append(failures, "score: Confidence nil")
			}
			if len(sa.Probabilities) == 0 {
				failures = append(failures, "score: Probabilities empty")
			}
			if len(sa.Legend) == 0 {
				failures = append(failures, "score: Legend empty")
			}
			for k, lg := range sa.Legend {
				if lg.Str == nil {
					failures = append(failures, fmt.Sprintf("score: legend[%q] not reachable through a typed Str field (type=%v)", k, lg.Type))
				}
			}
		}

		pass := len(failures) == 0
		status := "FAIL"
		obs := strings.Join(failures, "; ")
		if pass {
			status = "PASS"
			obs = "happy/batch50/score answers, usage, model, id and provider all reachable through typed fields; no map[string]any/[]any/UnknownRaw on the answer/usage side"
		}
		t.Logf("EVAL-E03 %s %s", status, obs)
	})

	t.Run("E04_caller_supplied_client", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureHappy))
		}))
		defer srv.Close()

		rt := &countingRoundTripper{}
		client := openrouter.New(openrouter.WithClient(&http.Client{Transport: rt}), openrouter.WithSecurity("test-key"))

		const calls = 3
		for i := 0; i < calls; i++ {
			if _, err := client.Alpha.Decisions.Create(context.Background(), buildTestRequest(), operations.WithServerURL(srv.URL)); err != nil {
				t.Fatalf("harness fault: Create #%d: %v", i, err)
			}
		}

		got := rt.count.Load()
		pass := got == int64(calls)
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		t.Logf("EVAL-E04 %s calls_made=%d requests_seen_by_caller_supplied_client=%d", status, calls, got)
	})

	t.Run("E05_error_dialects", func(t *testing.T) {
		fixtures := []struct {
			name        string
			status      int
			body        string
			contentType string
		}{
			{"OpenRouter400Choices(card256)", 400, fixtureOpenRouter400Choices, "application/json"},
			{"OpenRouter401(badkey)", 401, fixtureOpenRouter401, "application/json"},
			{"OpenRouter400BadType(badtype)", 400, fixtureOpenRouter400BadType, "application/json"},
			{"OpenRouter400MaxTokens(oversize)", 400, fixtureOpenRouter400MaxTokens, "application/json"},
			{"ChatPath400(chatpath)", 400, fixtureChatPath400, "application/json"},
			{"LiteLLM401(string code)", 401, fixtureLiteLLM401, "application/json"},
			{"LiteLLM403(string code)", 403, fixtureLiteLLM403, "application/json"},
			{"OpenRouter402", 402, fixtureOpenRouter402, "application/json"},
			{"OpenRouter404", 404, fixtureOpenRouter404, "application/json"},
			{"OpenRouter429", 429, fixtureOpenRouter429, "application/json"},
			{"OpenRouter500", 500, fixtureOpenRouter500, "application/json"},
			{"OpenRouter502", 502, fixtureOpenRouter502, "application/json"},
			{"OpenRouter503", 503, fixtureOpenRouter503, "application/json"},
			{"OpenRouter524", 524, fixtureOpenRouter524, "application/json"},
			{"OpenRouter529", 529, fixtureOpenRouter529, "application/json"},
			{"HTML502(non-JSON)", 502, fixtureHTML502, "text/html"},
		}

		client := openrouter.New(openrouter.WithSecurity("test-key"))

		var lines []string
		allPass := true
		for _, f := range fixtures {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", f.contentType)
				w.WriteHeader(f.status)
				_, _ = w.Write([]byte(f.body))
			}))

			_, err := client.Alpha.Decisions.Create(context.Background(), buildTestRequest(),
				operations.WithServerURL(srv.URL),
				operations.WithRetries(retry.Config{Strategy: "none"}))
			srv.Close()

			if err == nil {
				t.Fatalf("harness fault: fixture %s (status %d) returned no error", f.name, f.status)
			}

			status, isAPIError, isNamed, typeName := classifyDecisionError(err)
			derivable := isAPIError || isNamed
			ok := derivable && status == f.status
			if !ok {
				allPass = false
			}
			lines = append(lines, fmt.Sprintf("%s(want=%d): type=%s api_error=%v named_type=%v derived_status=%d derivable=%v",
				f.name, f.status, typeName, isAPIError, isNamed, status, derivable))
		}

		outerStatus := "FAIL"
		if allPass {
			outerStatus = "PASS"
		}
		t.Logf("EVAL-E05 %s %s", outerStatus, strings.Join(lines, "; "))
	})

	t.Run("E06_errors_is_propagation", func(t *testing.T) {
		errSentinel := errors.New("eval-e06-sentinel")

		// (a) RoundTripper transport error.
		rtA := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errSentinel
		})
		clientA := openrouter.New(openrouter.WithClient(&http.Client{Transport: rtA}), openrouter.WithSecurity("k"))
		_, errA := clientA.Alpha.Decisions.Create(context.Background(), buildTestRequest(),
			operations.WithServerURL("http://sdkeval.invalid"),
			operations.WithRetries(retry.Config{Strategy: "none"}))
		aOK := errA != nil && errors.Is(errA, errSentinel)

		// (b) Response body Read error after 64 bytes.
		rtB := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := &errAfterReader{data: bytes.Repeat([]byte("x"), 64), err: errSentinel}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       body,
				Request:    req,
				ProtoMajor: 1,
				ProtoMinor: 1,
			}, nil
		})
		clientB := openrouter.New(openrouter.WithClient(&http.Client{Transport: rtB}), openrouter.WithSecurity("k"))
		_, errB := clientB.Alpha.Decisions.Create(context.Background(), buildTestRequest(),
			operations.WithServerURL("http://sdkeval.invalid"),
			operations.WithRetries(retry.Config{Strategy: "none"}))
		bOK := errB != nil && errors.Is(errB, errSentinel)

		// (c) 5 MiB body: how many bytes did the SDK read?
		var bytesRead int64
		data := bytes.Repeat([]byte("x"), 5*1024*1024)
		rtC := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       &countingReadCloser{r: bytes.NewReader(data), n: &bytesRead},
				Request:    req,
				ProtoMajor: 1,
				ProtoMinor: 1,
			}, nil
		})
		clientC := openrouter.New(openrouter.WithClient(&http.Client{Transport: rtC}), openrouter.WithSecurity("k"))
		_, _ = clientC.Alpha.Decisions.Create(context.Background(), buildTestRequest(),
			operations.WithServerURL("http://sdkeval.invalid"),
			operations.WithRetries(retry.Config{Strategy: "none"}))
		cFullRead := bytesRead == int64(len(data))

		pass := aOK && bOK
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		obs := fmt.Sprintf("a_transport_err_is_sentinel=%v b_body_read_err_is_sentinel=%v c_bytes_read=%d/%d c_read_whole_body=%v",
			aOK, bOK, bytesRead, len(data), cFullRead)
		t.Logf("EVAL-E06 %s %s", status, obs)
	})

	t.Run("E07_retries", func(t *testing.T) {
		newCountingServer := func(status int, body string) (*httptest.Server, *int64) {
			var n int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt64(&n, 1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
			}))
			return srv, &n
		}

		runWith := func(deadline time.Duration, opts ...operations.Option) int64 {
			srv, n := newCountingServer(503, fixtureOpenRouter503)
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			client := openrouter.New(openrouter.WithSecurity("k"))
			callOpts := append([]operations.Option{operations.WithServerURL(srv.URL)}, opts...)
			_, _ = client.Alpha.Decisions.Create(ctx, buildTestRequest(), callOpts...)
			return atomic.LoadInt64(n)
		}

		noConfigCount := runWith(3 * time.Second)
		disabledCount := runWith(3*time.Second, operations.WithRetries(retry.Config{Strategy: "none"}))
		oneRetryCount := runWith(3*time.Second, operations.WithRetries(retry.Config{
			Strategy: "backoff",
			Backoff: &retry.BackoffStrategy{
				InitialInterval: 50,
				MaxInterval:     50,
				Exponent:        1,
				MaxElapsedTime:  80,
			},
		}))

		pass := disabledCount == 1
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		obs := fmt.Sprintf("no_retry_config_requests=%d retries_strategy_none_requests=%d bounded_backoff_attempt_requests=%d",
			noConfigCount, disabledCount, oneRetryCount)
		t.Logf("EVAL-E07 %s %s", status, obs)
	})

	t.Run("E08_deadline", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(2 * time.Second):
			case <-r.Context().Done():
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureHappy))
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		client := openrouter.New(openrouter.WithSecurity("k"))
		_, err := client.Alpha.Decisions.Create(ctx, buildTestRequest(),
			operations.WithServerURL(srv.URL),
			operations.WithRetries(retry.Config{Strategy: "none"}))

		pass := err != nil && errors.Is(err, context.DeadlineExceeded)
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		t.Logf("EVAL-E08 %s err=%v is_deadline_exceeded=%v", status, err, pass)
	})
}

// TestSDKConcurrency is E09: one SDK client driven from 4 goroutines x 25
// calls, under -race.
func TestSDKConcurrency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureHappy))
	}))
	defer srv.Close()

	client := openrouter.New(openrouter.WithSecurity("test-key"))

	const goroutines = 4
	const perGoroutine = 25
	var wg sync.WaitGroup
	var failures int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := client.Alpha.Decisions.Create(context.Background(), buildTestRequest(), operations.WithServerURL(srv.URL)); err != nil {
					atomic.AddInt64(&failures, 1)
				}
			}
		}()
	}
	wg.Wait()

	total := goroutines * perGoroutine
	if failures == 0 {
		t.Logf("EVAL-E09 PASS all %d calls succeeded across %d goroutines (-race clean)", total, goroutines)
	} else {
		t.Logf("EVAL-E09 FAIL %d/%d calls failed across %d goroutines", failures, total, goroutines)
	}
}
