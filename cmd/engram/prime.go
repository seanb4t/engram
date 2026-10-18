// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/scope"
)

// primeDigestLimit is how many of the most recent records the primer lists
// per scope, the same bound the session-start hook's instruction text asks
// the agent to use.
const primeDigestLimit = 10

// The values of a scope's "source" in the JSON primer.
const (
	primeSourceFlag     = "flag"
	primeSourceEnv      = "env"
	primeSourceOrigin   = "git_origin_remote"
	primeSourceDirName  = "repo_directory_name"
	primeSourceWorktree = "linked_worktree"
)

// primeSourceText is how the text primer names each source.
var primeSourceText = map[string]string{
	primeSourceFlag:     "--scope",
	primeSourceEnv:      "ENGRAM_SCOPE",
	primeSourceOrigin:   "git origin remote",
	primeSourceDirName:  "repository directory name (no origin remote; probably not a scope on the server)",
	primeSourceWorktree: "linked git worktree",
}

// primeCmd writes a session primer for an agent. Its main reader is the
// plugin's session-start hook, which pipes stdout into agent context, so
// it writes text whether or not stdout is a terminal (design D4).
var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Print a session primer for an agent: memory scope, rules index and recent memories",
	Long: "Print a session primer for an agent: the memory scope and where it came from, the rules index " +
		"for that scope, the most recent memories in the spine and the workspace overlay, and the command " +
		"that prints the machine-readable catalog. Without --scope or ENGRAM_SCOPE the scope is derived " +
		"from local git. Outside a git repository it says so, calls no server and exits 0. It only reads: " +
		"it never writes, prompts, starts a login or reads stdin.",
	Args: noPositionalArgs,
	RunE: runPrime,
}

func init() {
	addConnectionFlags(primeCmd)
	f := primeCmd.Flags()
	// prime's deadline covers the whole command, not each request.
	f.Lookup("timeout").Usage = "deadline for the whole command: git probes, the credential and every request " +
		"together (must be > 0, e.g. 3s, 30s; a value of 0 is rejected)"
	f.String("output", "text",
		`output format: "text" or "json"; text whether or not stdout is a terminal`)
	f.String("scope", config.FlagDefault("scope"),
		"memory scope to read, used exactly as given with no workspace overlay "+
			"(default: ENGRAM_SCOPE, then the scope derived from local git)")
	rootCmd.AddCommand(primeCmd)
}

// primeScope is one scope the primer reads and where it came from.
type primeScope struct {
	Scope  string `json:"scope"`
	Source string `json:"source"`
}

// primeScopes is the scope header: no spine means no scope was derived.
type primeScopes struct {
	spine      *primeScope
	overlay    *primeScope
	ruleScopes []string
}

// primeRecall is what the server returned for the primer's scopes.
type primeRecall struct {
	rules       []*engramv1.Memory
	spine       []*engramv1.Memory
	overlay     []*engramv1.Memory
	spineFull   bool
	overlayFull bool
}

func runPrime(cmd *cobra.Command, _ []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	if output != "text" && output != "json" {
		return usageErrorf(`--output %q: must be "text" or "json"`, output)
	}
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return usageErrorf("load client configuration: %w", err)
	}
	if err := config.ValidatePrimeScope(cfg.Prime.Scope, cmd.Flags().Changed("scope")); err != nil {
		return usageErrorf("%w", err)
	}
	if err := config.ValidateClient(cfg.Client); err != nil {
		return usageErrorf("%w", err)
	}
	// ValidateClient has already confirmed the timeout is a positive
	// duration. It bounds the whole command: git probes, the credential
	// and the RPCs together (design D7).
	timeout, err := time.ParseDuration(cfg.Client.Timeout)
	if err != nil {
		return usageErrorf("ENGRAM_TIMEOUT %q: must be a Go duration: %w", cfg.Client.Timeout, err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	scopes, err := resolvePrimeScopes(ctx, cfg.Prime, cmd.Flags().Changed("scope"))
	if err != nil {
		return err
	}
	var recall primeRecall
	if scopes.spine != nil {
		client, err := primeClient(ctx, cmd)
		if err != nil {
			return err
		}
		if recall, err = fetchPrimeRecall(ctx, client, scopes); err != nil {
			return wrapRPCError(err)
		}
	}
	if output == "json" {
		return renderPrimeJSON(cmd.OutOrStdout(), scopes, recall)
	}
	return renderPrimeText(cmd.OutOrStdout(), scopes, recall)
}

// resolvePrimeScopes picks the spine from --scope, then ENGRAM_SCOPE, then
// local git (design D2). An override is used exactly as given, with no
// overlay. Outside a repository it returns no spine and a nil error.
func resolvePrimeScopes(ctx context.Context, p config.PrimeConfig, fromFlag bool) (primeScopes, error) {
	var out primeScopes
	if p.Scope != "" {
		source := primeSourceEnv
		if fromFlag {
			source = primeSourceFlag
		}
		out.spine = &primeScope{Scope: p.Scope, Source: source}
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return out, fmt.Errorf("read the working directory: %w", err)
		}
		s, err := scope.Derive(ctx, wd)
		if err != nil {
			return out, primeDeadlineErr("derive the memory scope from git", err)
		}
		if s.RepoID == "" {
			return out, nil
		}
		source := primeSourceDirName
		if s.FromOrigin {
			source = primeSourceOrigin
		}
		out.spine = &primeScope{Scope: s.Spine(), Source: source}
		if overlay := s.Overlay(); overlay != "" {
			out.overlay = &primeScope{Scope: overlay, Source: primeSourceWorktree}
		}
	}
	out.ruleScopes = primeRuleScopes(out.spine.Scope, p.Project)
	return out, nil
}

// primeRuleScopes returns rule:repo:<repo-id> when the memory scope has the
// form repo:<repo-id>[...], plus rule:project:<project> when ENGRAM_PROJECT
// is set, as the session-start hook builds them.
func primeRuleScopes(memoryScope, project string) []string {
	var scopes []string
	if rest, ok := strings.CutPrefix(memoryScope, "repo:"); ok {
		if id, _, _ := strings.Cut(rest, ":"); id != "" {
			scopes = append(scopes, "rule:repo:"+id)
		}
	}
	if project = strings.TrimSpace(project); project != "" {
		scopes = append(scopes, "rule:project:"+project)
	}
	return scopes
}

// primeClient builds the client under ctx, so a slow token refresh at the
// issuer counts against prime's one deadline.
func primeClient(ctx context.Context, cmd *cobra.Command) (engramv1connect.EngramServiceClient, error) {
	type built struct {
		client engramv1connect.EngramServiceClient
		err    error
	}
	ch := make(chan built, 1)
	go func() {
		client, _, _, err := clientFromFlags(cmd)
		ch <- built{client, err}
	}()
	select {
	case b := <-ch:
		return b.client, b.err
	case <-ctx.Done():
		return nil, primeDeadlineErr("resolve the server and credential", ctx.Err())
	}
}

// primeDeadlineErr classifies a context error the way exitCodeForConnectErr
// classifies its Connect equivalent: a deadline is a timeout (6), a
// cancellation is unavailable (5). Any other error passes through.
func primeDeadlineErr(what string, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &cliError{code: exitTimeout, err: fmt.Errorf("%s: %w", what, err)}
	case errors.Is(err, context.Canceled):
		return &cliError{code: exitUnavailable, err: fmt.Errorf("%s: %w", what, err)}
	}
	return err
}

// fetchPrimeRecall makes the primer's RPCs at once under ctx: the rules
// index (when there is a rule scope) and the most recent records in the
// spine and the overlay, all compact and recall-gated. It fails if any call
// fails, so the caller writes nothing on a failure.
func fetchPrimeRecall(ctx context.Context, client engramv1connect.EngramServiceClient, s primeScopes) (primeRecall, error) {
	var (
		wg                             sync.WaitGroup
		r                              primeRecall
		rulesErr, spineErr, overlayErr error
	)
	listRecent := func(memoryScope string) ([]*engramv1.Memory, error) {
		resp, err := client.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: memoryScope,
			Limit: primeDigestLimit,
		}))
		if err != nil {
			return nil, err
		}
		return resp.Msg.GetMemories(), nil
	}
	if len(s.ruleScopes) > 0 {
		wg.Go(func() {
			resp, err := client.ListRules(ctx, connect.NewRequest(&engramv1.ListRulesRequest{Scopes: s.ruleScopes}))
			if err != nil {
				rulesErr = err
				return
			}
			r.rules = resp.Msg.GetRules()
		})
	}
	wg.Go(func() { r.spine, spineErr = listRecent(s.spine.Scope) })
	if s.overlay != nil {
		wg.Go(func() { r.overlay, overlayErr = listRecent(s.overlay.Scope) })
	}
	wg.Wait()
	if err := cmp.Or(rulesErr, spineErr, overlayErr); err != nil {
		return primeRecall{}, err
	}
	r.spineFull = len(r.spine) >= primeDigestLimit
	r.overlayFull = len(r.overlay) >= primeDigestLimit
	// A record in both groups is shown once, in the repo-wide group.
	inSpine := make(map[string]bool, len(r.spine))
	for _, m := range r.spine {
		inSpine[m.GetId()] = true
	}
	overlay := r.overlay[:0:0]
	for _, m := range r.overlay {
		if !inSpine[m.GetId()] {
			overlay = append(overlay, m)
		}
	}
	r.overlay = overlay
	return r, nil
}

// primeCatalogCommand is the command that prints the machine-readable
// catalog, read from the live command tree.
func primeCatalogCommand() string {
	return catalogCmd.CommandPath()
}

// primeHandle is a record's full short_id, or its UUID when it has none.
func primeHandle(m *engramv1.Memory) string {
	return consoleText(cmp.Or(m.GetShortId(), m.GetId()), false)
}

// renderPrimeText writes the text primer (design D5). Every server value
// passes through consoleText: another actor's shared record is untrusted,
// and a control rune or a line break must not reach a terminal or forge a
// line of the primer.
func renderPrimeText(w io.Writer, s primeScopes, r primeRecall) error {
	var b strings.Builder
	b.WriteString("# engram session primer\n\n")
	if s.spine == nil {
		b.WriteString("No memory scope was derived: this directory is not inside a git repository, or git could not run.\n")
		b.WriteString("Pass --scope (or set ENGRAM_SCOPE) to choose one, for example: engram prime --scope repo:<host>/<org>/<repo>\n")
	} else {
		fmt.Fprintf(&b, "Memory spine scope: %s\n", consoleText(s.spine.Scope, false))
		fmt.Fprintf(&b, "Spine scope source: %s\n", primeSourceText[s.spine.Source])
		if s.overlay != nil {
			fmt.Fprintf(&b, "Memory workspace scope: %s\n", consoleText(s.overlay.Scope, false))
			fmt.Fprintf(&b, "Workspace scope source: %s\n", primeSourceText[s.overlay.Source])
		}
		ruleScopes := "none"
		if len(s.ruleScopes) > 0 {
			ruleScopes = consoleText(strings.Join(s.ruleScopes, ", "), false)
		}
		fmt.Fprintf(&b, "Rule scopes: %s\n", ruleScopes)
		if len(r.rules) > 0 {
			b.WriteString("\n## Rules\n\n")
			for _, rule := range r.rules {
				fmt.Fprintf(&b, "- %s — %s", primeHandle(rule), consoleText(rule.GetSummary(), false))
				if tags := rule.GetTags(); len(tags) > 0 {
					fmt.Fprintf(&b, " [%s]", consoleText(strings.Join(tags, ", "), false))
				}
				b.WriteString("\n")
			}
		}
		writePrimeGroup(&b, "Repo-wide memories", r.spine, r.spineFull)
		writePrimeGroup(&b, "This workspace's memories", r.overlay, r.overlayFull)
	}
	fmt.Fprintf(&b, "\nMachine-readable command catalog: %s\n", primeCatalogCommand())
	_, err := io.WriteString(w, b.String())
	return err
}

// writePrimeGroup writes one digest group, or nothing when it is empty.
func writePrimeGroup(b *strings.Builder, heading string, mems []*engramv1.Memory, full bool) {
	if len(mems) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n", heading)
	for _, m := range mems {
		summary := cmp.Or(m.GetSummary(), "(no summary yet)")
		fmt.Fprintf(b, "- %s — %s\n", primeHandle(m), consoleText(summary, false))
	}
	if full {
		fmt.Fprintf(b, "- (%d shown; older memories are reachable with search_memory or a higher list_memory limit)\n", len(mems))
	}
}

// primeDoc is the --output json primer (design D4). Records keep the
// compact shape `engram list` writes, so no new record shape exists.
type primeDoc struct {
	Spine          *primeScope       `json:"spine"`
	Overlay        *primeScope       `json:"overlay"`
	RuleScopes     []string          `json:"rule_scopes"`
	Rules          []json.RawMessage `json:"rules"`
	Memories       primeMemoriesDoc  `json:"memories"`
	CatalogCommand string            `json:"catalog_command"`
}

type primeMemoriesDoc struct {
	Spine   []json.RawMessage `json:"spine"`
	Overlay []json.RawMessage `json:"overlay"`
}

func renderPrimeJSON(w io.Writer, s primeScopes, r primeRecall) error {
	doc := primeDoc{
		Spine:          s.spine,
		Overlay:        s.overlay,
		RuleScopes:     append([]string{}, s.ruleScopes...),
		CatalogCommand: primeCatalogCommand(),
	}
	var err error
	if doc.Rules, err = primeJSONRecords(r.rules); err != nil {
		return err
	}
	if doc.Memories.Spine, err = primeJSONRecords(r.spine); err != nil {
		return err
	}
	if doc.Memories.Overlay, err = primeJSONRecords(r.overlay); err != nil {
		return err
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// primeJSONRecords marshals each record with renderJSON's options.
func primeJSONRecords(mems []*engramv1.Memory) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(mems))
	opts := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}
	for _, m := range mems {
		b, err := opts.Marshal(m)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
