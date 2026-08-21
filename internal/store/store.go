// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store persists and queries memories as vectors in a Qdrant collection.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/authz"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// tracer is the package-level OTel tracer for store-layer spans. It delegates to
// the global TracerProvider at call time.
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/store")

// ownerOf returns the span-safe opaque owner for subj. A nil Subject (the
// discarded-extraction-error fail-closed case) has no owner, so it reports "" —
// never panicking on the interface method call.
func ownerOf(subj Subject) string {
	if subj == nil {
		return ""
	}
	return subj.Owner()
}

// principalParams converts a Subject into the primitives the authz PDP takes
// (owner, kind). It is the ONLY Subject->primitives converter and lives here
// (not internal/authz) to avoid an import cycle. kind is hardcoded "human"
// (A1 — no policy conditions on it this phase). The nil/unknown default arm
// returns ok=false: this is the fail-closed signal read-filter builders use
// to return matchNothing() WITHOUT calling the PDP, because own_records
// deliberately grants owner=="" for the legitimate anonymous bucket, so an
// owner=="" principal must never be conflated with a nil Subject.
func principalParams(subj Subject) (owner, kind string, ok bool) {
	switch s := subj.(type) {
	case authenticated:
		return s.sub, "human", true
	case anonymous:
		return "", "human", true
	default:
		return "", "", false
	}
}

// ErrNotFound is returned when an id is absent OR not visible to the caller —
// the two are indistinguishable by design, so ownership never leaks across actors.
var ErrNotFound = errors.New("not found")

// ErrInvalidArgument tags errors caused by a malformed caller request (a bad
// cursor, mutually exclusive paging options) as opposed to a Qdrant/infra
// failure. The Connect layer maps it to CodeInvalidArgument and everything else
// to CodeInternal, so a Count/Scroll outage is no longer mislabeled as a client
// error.
var ErrInvalidArgument = errors.New("invalid argument")

// ErrAmbiguousShortID means a short id matched more than one record — an
// invariant violation (MintShortID enforces global uniqueness), surfaced rather
// than silently resolving to an arbitrary point.
var ErrAmbiguousShortID = errors.New("ambiguous short id")

// ErrShortIDExhausted means MintShortID gave up after maxMintAttempts real
// Qdrant collision checks — a pathological Qdrant state (or a broken mint
// generator) surfaces as a normal write failure instead of hanging the
// request indefinitely.
var ErrShortIDExhausted = errors.New("short id mint exhausted")

// ErrIdempotencyConflict is returned when a keyed store_memory/schedule_memory
// replay is submitted with the same idempotency key but content that does not
// match the stored IdempotencyFingerprint (Phase 24, D-10). It is a distinct,
// reportable sentinel — deliberately NOT an alias of ErrNotFound and never
// folded into the not-found path, so a coding agent can tell "no such record"
// apart from "you reused a key with different content."
var ErrIdempotencyConflict = errors.New("idempotency key reused with different content")

// ErrAlreadySuperseded is returned when Store.Supersede's target already has a
// non-empty SupersededBy — a single live head per chain (D-05). Forward chains
// (superseding the current head) are allowed; only superseding a non-head
// record is rejected, which makes a supersession cycle structurally
// impossible (D-06).
var ErrAlreadySuperseded = errors.New("target is already superseded")

// MultiTargetError wraps a sentinel error together with every offending
// canonical (resolved) target id, for Store.Supersede's multi-target
// rejections (PD-10, 03.1-REVIEWS.md cycle-1 MEDIUM). It exists so the
// handler tier can recover the offending ids via errors.As instead of
// parsing them back out of a formatted message — echoing a resolved UUID
// pulled from parsed prose is exactly the leak INV-01 exists to prevent.
// Unwrap keeps errors.Is(err, ErrAlreadySuperseded) matching byte-for-byte
// unchanged; Error() renders in the same shape the pre-phase scalar path
// rendered ("<sentinel>: <ids>"). Nothing outside internal/ reads this type.
type MultiTargetError struct {
	Err error
	IDs []string
}

// Error renders "<sentinel>: <id1>, <id2>, ...".
func (e *MultiTargetError) Error() string {
	return e.Err.Error() + ": " + strings.Join(e.IDs, ", ")
}

// Unwrap returns the wrapped sentinel, so errors.Is(err, ErrAlreadySuperseded)
// (and any future sentinel this type wraps) keeps matching unchanged.
func (e *MultiTargetError) Unwrap() error {
	return e.Err
}

// qdrantPayloadOpBatchSize mirrors the pinned Qdrant server's own
// payload-operation batch size (qdrant/qdrant:v1.18.2,
// lib/shard/src/update.rs, PAYLOAD_OP_BATCH_SIZE): the server chunks a
// multi-ID payload write (SetPayload/DeletePayload/UpdateVectors/
// OverwritePayload) by this many point ids, and a later chunk can error
// after an earlier chunk has fully committed. Store.Supersede's target set
// is deliberately NOT capped at this value (D-15, 03.1-00 PD-07 ruling:
// no-cap-otherwise-defaults) — Store.reconcileSupersedeFailure, which
// re-reads the FULL requested target set rather than one chunk, is the
// single mechanism covering both the within-chunk and the cross-chunk
// partial-application classes. This constant exists to make the number
// visible, not to impose a limit; a Qdrant image bump must re-verify it
// against the new version's source.
const qdrantPayloadOpBatchSize = 32

// maxMintAttempts bounds MintShortID's real (Qdrant Count-checked) collision
// attempts. 16 is extra headroom over the ~8 that is already astronomically
// safe in a 32^10 Crockford base32 space (D-04).
const maxMintAttempts = 16

// maxMintSpins is an absolute upper bound on total loop iterations in
// MintShortID — including seen-map skips, which deliberately do NOT consume the
// maxMintAttempts real-collision-check budget (D-05). It exists purely as a
// belt-and-suspenders termination guarantee for the injectable mintCandidate
// seam: a degenerate generator that returns only already-seen candidates would
// otherwise spin forever (each seen hit nets zero against attempts). The
// production shortid.New generator draws from a 32^10 space, so this cap is
// unreachable in practice; a generous multiple of maxMintAttempts keeps it
// clear of any legitimate seen-map churn.
const maxMintSpins = maxMintAttempts * 100

// visibilityShared is the Visibility sentinel for a record readable by any
// authenticated caller. Sharing grants read, never write. Defined once so a typo
// in an authorization path is a compile error rather than a silent gate bypass.
const visibilityShared = "shared"

// SummarySource records the provenance of a record's summary. It is persisted
// to Qdrant (payload/fromPayload) and crosses the Connect proto and MCP recall
// boundaries as a bare string, so its values are contract-locked — use the
// constants below. The named type is a convention aid, not enforcement: Go's
// untyped string constants still assign to it, so a stray "Client" would
// compile, persist, and silently miss the stale-summary guard's
// == SummarySourceClient check. Prefer the constants.
type SummarySource string

const (
	// SummarySourceNone marks a record with no summary provenance. It is the
	// zero value so an unconfigured Memory reads as none without assignment.
	SummarySourceNone SummarySource = ""
	// SummarySourceClient marks a caller-authored summary (set by store_memory
	// and schedule_memory via toMemory, and re-stamped by update_memory) — the
	// trust signal the stale-summary guard protects.
	SummarySourceClient SummarySource = "client"
	// SummarySourceAuto marks a sweep-generated summary (regenerable,
	// auto-clears on content change).
	SummarySourceAuto SummarySource = "auto"
)

// Memory is the unit of storage. Fields map 1:1 to Qdrant payload keys.
type Memory struct {
	ID string `json:"id"`
	// ShortID is a short, case-insensitive Crockford base32 handle (see
	// internal/shortid) minted alongside ID and usable anywhere an id is
	// accepted. Stable: never rotated once assigned. Empty only for
	// pre-backfill legacy records.
	ShortID   string   `json:"short_id,omitempty"`
	Content   string   `json:"content"`
	Scope     string   `json:"scope"` // run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster
	Repo      string   `json:"repo"`
	Workspace string   `json:"workspace"`
	Worktree  string   `json:"worktree_path"`
	BaseDir   string   `json:"base_dir"`
	Source    string   `json:"source"` // user-said | agent-inferred
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	// Actor is the verified caller identity (email/username/subject) taken from
	// the validated OIDC token — never client-supplied. Empty when auth is off.
	Actor string `json:"actor"`
	// Owner is the caller's configured owner-claim value (default email) — the
	// authorization key. Server-set, never client-supplied.
	Owner string `json:"owner"`
	// Visibility gates cross-actor reads: "" (private, default) or "shared"
	// (readable by any authenticated caller). Writes always require ownership.
	Visibility string    `json:"visibility,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	// NotBefore gates deferred reveal: the record is hidden from recall until
	// now >= NotBefore. nil = always active (no lower gate).
	NotBefore *time.Time `json:"not_before,omitempty"`
	// NotAfter gates expiry: the record drops out of recall once now >= NotAfter.
	// nil = never expires.
	NotAfter *time.Time `json:"not_after,omitempty"`
	// Supersedes holds the ids of every record this one corrects/replaces
	// (D-04, promoted from a scalar to a set in phase 03.1 — D-01 promote
	// semantics: a one-element slice behaves exactly as the pre-phase
	// single-target call). Ordered as the store received them.
	// nil/empty on every record except a correcting one created via
	// Store.Supersede. Plain json tag (not "-"): the caller must be able to
	// observe this link on full=true recall and get_memory (D-08).
	Supersedes []string `json:"supersedes,omitempty"`
	// SupersededBy is the id of the record that superseded this one (D-01/D-04).
	// Server-set only, via Store.Supersede's target back-stamp — nil until then.
	// Non-nil excludes the record from Search/List recall (soft-hide, D-09) but
	// never from Get (D-08). Plain json tag, matching Supersedes above.
	SupersededBy *string `json:"superseded_by,omitempty"`
	// ArchivedAt is the epoch-second instant this record was archived via
	// Store.Archive (D-12, plan 03-06). Server-set only — no client-writable
	// tool argument sets it. nil means never archived. Non-nil excludes the
	// record from Search/List/SearchDiscovery/ListScheduled recall (soft-hide)
	// but never from Get (matching Supersedes/SupersededBy's own D-08
	// contract). Orthogonal to NotAfter: archiving never writes not_after and
	// expiring never writes archived_at, so an archived record and a
	// naturally-expired record stay independently observable states — reusing
	// NotAfter for this was the option D-12 rejected (RESEARCH.md Pitfall 3).
	// Reversible via Store.Restore, which deletes the key outright rather than
	// writing a false/zero sentinel.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// AccessCount is the monotonic total of strong-signal touches (get-by-id +
	// update; never search/list result-set membership, D-02). Server-set only —
	// no client-writable tool argument sets it. A legacy record missing the
	// payload key reads 0, no backfill required (D-03). MUST NOT be read by the
	// reranker or any recall gate (D-08).
	AccessCount uint64 `json:"access_count"`
	// LastAccessedAt is the timestamp of the most recent strong-signal touch
	// (get-by-id or update). nil when the record has never been accessed — a
	// pointer (like NotBefore/NotAfter) so json omitempty actually fires and a
	// never-accessed record omits the field rather than emitting 0001-01-01.
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	// Kind is discovery-only (zero-valued for the curated categories): "map" | "fact".
	Kind string `json:"kind,omitempty"`
	// Citations are optional source anchors on ANY category (Phase 26, D-01) —
	// required (>= 1) only for discoveries, optional and never auto-populated
	// on curated memory records. The write gate in payload() is independent of
	// the Kind gate: citations are written whenever non-empty, regardless of
	// category.
	Citations []Citation `json:"citations,omitempty"`
	// Summary is a short recall line shown in place of Content by the recall
	// path. Authored by the caller (SummarySource "client") or filled by the
	// offline summarize-missing sweep ("auto"); "" means none.
	Summary string `json:"summary,omitempty"`
	// SummarySource records summary provenance: client | auto | none. See the
	// named type and prefer its constants.
	SummarySource SummarySource `json:"summary_source,omitempty"`
	// SummaryModel names the model that produced an "auto" summary (diagnostics);
	// empty otherwise.
	SummaryModel string `json:"summary_model,omitempty"`
	// SummaryEgressAt is the k1oe.2 durable audit stamp: when this record's
	// content was egressed to the summarizer model (auto path only). Plain
	// json tag, visible on both the MCP and Connect wires. Zero if never
	// egressed or the summary was client-authored/cleared.
	SummaryEgressAt time.Time `json:"summary_egress_at"`
	// Score is the Qdrant similarity score of this record for the query that
	// returned it (higher = closer). Set only on Search results; zero on
	// list/get. Lets callers see how close a near-miss ranked (GH#261).
	Score float32 `json:"score,omitempty"`
	// EmbedderIdentity is a server-set audit stamp (config.EmbedderIdentity)
	// of the embedder config that produced this record's stored document
	// vector, so a future reindex-boundary audit can detect mixed-embedding-
	// space records (D-05). A legacy record missing the payload key reads ""
	// — no backfill. The `json:"-"` tag is deliberate and load-bearing: this
	// field is payload-only, persisted EXCLUSIVELY through the manual
	// payload()/fromPayload() codec below, and must NEVER cross any JSON
	// wire — store.Memory is returned verbatim on the full-response MCP
	// paths (shapeRecall full, get_memory, listRules full), so a normal json
	// tag here would leak the audit field onto the wire (D-06).
	EmbedderIdentity string `json:"-"`
	// IdempotencyFingerprint is a server-set sha256 content fingerprint (see
	// internal/server's contentFingerprint) of the client-authored fields
	// submitted on a keyed store_memory/schedule_memory call (Phase 24, D-06/
	// D-07). Empty for a keyless record. It is compared on replay to detect
	// same-key/different-content (ErrIdempotencyConflict) and is NEVER
	// recall-filtered or Qdrant-indexed — a point Get is the only reader. The
	// `json:"-"` tag is deliberate and load-bearing, exactly like
	// EmbedderIdentity above: this field is payload-only, persisted
	// EXCLUSIVELY through the manual payload()/fromPayload() codec below, and
	// must NEVER cross any JSON wire — store.Memory is returned verbatim on
	// the full-response MCP paths, so a normal json tag here would leak the
	// fingerprint onto the wire.
	//
	// Frozen at create time (IN-02): Store.Update re-Upserts the fetched
	// record's existing IdempotencyFingerprint verbatim — it is never
	// recomputed from the record's post-update Content. A future keyed
	// store_memory replay using the ORIGINAL creation-time content will
	// still fingerprint-match against a record whose current content has
	// since diverged via update_memory. This is intentional (update_memory
	// is a distinct, explicit mutation path outside the idempotent-capture
	// contract, not a replay), but a reader should not assume this field
	// tracks the record's current state.
	IdempotencyFingerprint string `json:"-"`
	// SchemaVersion is the record's schema-version discriminator
	// (internal/migrate.Version). Three assertions:
	//
	//  1. Server-set only and monotonic — no client-writable tool argument
	//     sets it. payload() computes it as the greater of the current
	//     schema constant and the record's own decoded value, so a newer
	//     record is never downgraded by an older binary (D-05).
	//  2. A legacy record missing the payload key reads migrate.Version(0):
	//     no backfill required (D-09).
	//  3. MUST NOT be read by any recall or authz filter (D-13/D-16) — the
	//     superseded_by/archived_at IsEmpty idiom has INVERTED cardinality
	//     here (absence is the majority state at adoption, not a minority
	//     one), and copying it would exclude every pre-migration record
	//     from recall.
	//
	// The plain json tag (no "-") is a deliberate divergence from the two
	// adjacent EmbedderIdentity/IdempotencyFingerprint fields above, whose
	// json:"-" tags are themselves deliberate and load-bearing — do not
	// "fix" this field to match them. omitempty is specifically excluded
	// too: it would hide the field exactly on the v0 records this phase
	// exists to make visible (D-10).
	//
	// Accepted limitation (D-06): an older binary that edits a newer
	// record keeps the newer stamp while rebuilding the payload from its
	// own older struct, so version-newer-only keys are dropped. This is
	// acceptable because migration steps are additive-only, what is lost
	// is re-derivable, and the recovery is re-running the migration sweep.
	//
	// Narrowed no-downgrade boundary: the guarantee is that a normal full
	// write stamps at least migrate.CurrentVersion, and that rewriting a
	// DECODED newer record through Store.Update preserves its higher
	// version. Store.Upsert is public replacement-by-id and does NOT read
	// the stored record before writing, so a caller holding a stale Memory
	// CAN lower a stored version through it — that is outside the
	// guarantee, stated here rather than papered over. A stored-version
	// read or compare-and-swap on Upsert was considered and rejected for
	// this phase: it would change the semantics of a public method every
	// existing caller depends on, which is not additive scope.
	SchemaVersion migrate.Version `json:"schema_version"`
}

// embedderIdentityKey is the shared Qdrant payload key for
// Memory.EmbedderIdentity, written by payload() and read by fromPayload().
// Reused verbatim by Store.Reindex's divergent raw-map write (13-03) — defined
// once here so the two sites cannot drift.
const embedderIdentityKey = "embedder_identity"

// idempotencyFingerprintKey is the shared Qdrant payload key for
// Memory.IdempotencyFingerprint, written by payload() and read by
// fromPayload() (Phase 24, D-06). Payload-only, unindexed (DEC-ef28 unchanged).
const idempotencyFingerprintKey = "idempotency_fingerprint"

// schemaVersionKey is the shared Qdrant payload key for Memory.SchemaVersion.
// Three readers: payload() (writes the monotonic stamp), fromPayload() (the
// absent-safe decode), and ensureIndexes() (the payload index registration).
const schemaVersionKey = "schema_version"

// EmbedText builds the text sent to the embedder for a record. Tags are folded
// into the embedded document so curated keywords contribute to vector recall
// (they are otherwise only a hard AND pre-filter). Documents are embedded via
// this helper at store, update, and reindex time; the query side is embedded
// separately (optionally with an instruction prefix) so re-embedding an existing
// corpus with `engram reindex` applies the same composition uniformly.
func EmbedText(content string, tags []string) string {
	if len(tags) == 0 {
		return content
	}
	return content + "\n\ntags: " + strings.Join(tags, ", ")
}

// Citation anchors a discovery to a source so it can be verified and aged.
type Citation struct {
	Kind    string `json:"kind"`              // file | commit | url | repo
	Ref     string `json:"ref"`               // path / repo URL / doc URL
	Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
	Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
	Excerpt string `json:"excerpt,omitempty"` // cached substance
}

// Store persists and queries memories in a Qdrant collection.
type Store struct {
	client     *qdrant.Client
	collection string
	now        func() time.Time

	// authz is the policy decision point consulted by the bulk read-filter
	// builders (ownerOrSharedCondition/ownerOnlyCondition) to decide which
	// buckets (own/shared) a caller may read. Defaulted to authz.MustDefault()
	// in New(), mirroring how `now` defaults to time.Now — WithAuthz overrides
	// it in tests. The PDP is never consulted from internal/server handlers;
	// it is owned by the store, the single default-deny chokepoint (DEC-cgb).
	authz *authz.PDP

	// decideBucket routes the read-filter builders' bucket-authz decisions;
	// nil defaults to s.authz.DecideBucket (via the decideBucket method below).
	// *authz.PDP is a sealed concrete type with no exported constructor besides
	// MustDefault, so this function-var field (mirroring mintCandidate/
	// deletePayloadKeys) is how tests inject a call-counting probe (SC3)
	// without a broader authz-interface refactor.
	decideBucketHook func(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision

	// decideRecordHook routes the id-addressed gates' (GetReadable/getWritable/
	// OwnedOrAbsent) per-record authz decisions; nil defaults to
	// s.authz.DecideRecord (via the decideRecord method below). Same sealed-type
	// rationale as decideBucketHook: *authz.PDP has no exported constructor
	// besides MustDefault, so this function-var field is how tests inject an
	// all-deny probe to prove a Cedar Deny maps to the uniform ErrNotFound (SC4)
	// without a broader authz-interface refactor.
	decideRecordHook func(owner, kind string, action authz.Action, memoryOwner, category, visibility, scope string) authz.Decision

	// mintCandidate generates a short_id candidate; nil defaults to shortid.New.
	// Overridable in tests to force MintShortID's collision-retry branch.
	mintCandidate func() (string, error)

	// deletePayloadKeys issues the targeted Qdrant DeletePayload op removing the
	// given keys from one point; nil defaults to defaultDeletePayloadKeys
	// (s.client.DeletePayload). *qdrant.Client is a concrete type with no
	// interface seam, so this function-var field (mirroring mintCandidate) is
	// how UpdatePayload's two-op provenance-clear partial-failure path is
	// injected in tests (round-8 injected-failure test) without a broader
	// client-interface refactor.
	deletePayloadKeys func(ctx context.Context, id string, keys []string) error

	// setPayloadKeys issues one multi-ID Qdrant SetPayload op merging the
	// given key/value pairs onto every point in ids; nil defaults to
	// defaultSetPayloadKeys (s.client.SetPayload). *qdrant.Client is a
	// concrete type with no interface seam, so this function-var field
	// (mirroring deletePayloadKeys/mintCandidate) is how Store.Supersede's
	// multi-target back-stamp is routed — and is how its compensation
	// branch is made reachable by a test: a nonexistent target rejects the
	// whole call in getWritable before Upsert (store.go:1621-1625), so a bad
	// input can never reach a post-write branch. Only a scripted fault at
	// this seam, AFTER every target has passed preflight and the survivor
	// has been written, can exercise the compensation that follows it.
	setPayloadKeys func(ctx context.Context, ids []string, kv map[string]any) error

	// deletePoint issues the raw, UNGATED Qdrant point-removal for one id; nil
	// defaults to defaultDeletePoint (s.client.Delete). It exists for
	// Store.Supersede's post-failure compensating removal of the survivor it
	// just created, and for a second, independent reason (plan 03.1-02):
	// Store.Delete re-runs getWritable with authz.ActionDelete, a SECOND
	// authorization decision taken during rollback — a policy permitting
	// create/write but denying delete would strand the orphan. Rollback
	// correctness must not depend on a policy file, so this primitive
	// bypasses that gate entirely. MUST NOT be used for any caller-supplied
	// id — the only legitimate argument is a record minted inside the same
	// call (newMem.ID in reconcileSupersedeFailure), never linked, never
	// recallable by another caller. *qdrant.Client is a concrete type with no
	// interface seam, so this function-var field (mirroring setPayloadKeys/
	// deletePayloadKeys/mintCandidate) is how the survivor-removal-failure
	// branch is made reachable by a test.
	deletePoint func(ctx context.Context, id string) error

	// locker serializes Supersede's check-then-act (already-superseded guard
	// through the back-stamp) per target id. Defaults to an in-process
	// sync.Mutex-backed implementation in New(); WithTargetLocker overrides it
	// (e.g. with a future distributed lock). See TargetLocker's doc comment.
	locker TargetLocker
}

// Option configures a Store at construction.
type Option func(*Store)

// WithClock overrides the time source the recall window gate reads. Defaults to
// time.Now. Tests inject a fixed clock to exercise active/scheduled/expired
// boundaries deterministically.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.now = fn }
}

// WithAuthz overrides the store's policy decision point. Defaults to
// authz.MustDefault(). Tests inject an all-deny or call-counting PDP to
// exercise Deny paths and prove per-bucket call counts.
func WithAuthz(pdp *authz.PDP) Option {
	return func(s *Store) { s.authz = pdp }
}

// WithTargetLocker overrides Supersede's per-target lock. Defaults to an
// in-process sync.Mutex-backed TargetLocker (single-instance atomicity only).
// Provided so a future distributed lock can be swapped in without touching
// Supersede's logic; tests also use this to inject a call-counting/blocking
// probe.
func WithTargetLocker(l TargetLocker) Option {
	return func(s *Store) { s.locker = l }
}

// New returns a Store backed by the given Qdrant client and collection.
func New(c *qdrant.Client, collection string, opts ...Option) *Store {
	s := &Store{
		client: c, collection: collection, now: time.Now, authz: authz.MustDefault(),
		locker: newInProcessTargetLocker(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// EnsureCollection is idempotent: creates the collection at the given vector size if absent.
func (s *Store) EnsureCollection(ctx context.Context, dim uint64) (err error) {
	ctx, span := tracer.Start(ctx, "store.EnsureCollection")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "EnsureCollection", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	return s.ensureCollection(ctx, s.collection, dim)
}

// ensureCollection idempotently creates a named collection at the given vector
// size (distance Cosine) if absent, then ensures the recall payload indexes on
// every call. Factored out of EnsureCollection so reindex can provision a
// *target* collection distinct from s.collection.
func (s *Store) ensureCollection(ctx context.Context, name string, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size: dim, Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return err
		}
	}
	// Indexes are ensured on every boot (idempotently) so existing collections
	// gain them without a data migration: Qdrant backfills the index over the
	// already-stored RFC3339 created_at strings and keyword payloads.
	return s.ensureIndexes(ctx, name)
}

// ensureIndexes idempotently creates the recall payload indexes. owner is a
// tenant-optimized keyword (authz key), scope a keyword, created_at a datetime
// (enables server-side DatetimeRange + order_by). Re-creating an existing index
// is idempotent in Qdrant; the AlreadyExists check defends against versions/races
// that return it instead of succeeding silently.
//
// schema_version is an integer index (D-12) serving the future Phase 3
// migration sweep and Phase 4's `migrate status` version histogram
// EXCLUSIVELY — it must never serve recall. See
// internal/store/schemaversion_recallgate_test.go (plan 02-03) for the gate
// that holds that line: an index that exists makes it easier for a future
// filter to reach for the field, which is precisely why that gate — not
// inconvenience — is the guard.
func (s *Store) ensureIndexes(ctx context.Context, name string) error {
	type idx struct {
		field  string
		typ    qdrant.FieldType
		params *qdrant.PayloadIndexParams
	}
	idxs := []idx{
		{"owner", qdrant.FieldType_FieldTypeKeyword,
			qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{IsTenant: qdrant.PtrOf(true)})},
		{"scope", qdrant.FieldType_FieldTypeKeyword, nil},
		{"created_at", qdrant.FieldType_FieldTypeDatetime, nil},
		{"short_id", qdrant.FieldType_FieldTypeKeyword, nil},
		{schemaVersionKey, qdrant.FieldType_FieldTypeInteger, nil},
	}
	for _, ix := range idxs {
		req := &qdrant.CreateFieldIndexCollection{
			CollectionName:   name,
			FieldName:        ix.field,
			FieldType:        qdrant.PtrOf(ix.typ),
			FieldIndexParams: ix.params,
			Wait:             qdrant.PtrOf(true),
		}
		if _, err := s.client.CreateFieldIndex(ctx, req); err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == grpccodes.AlreadyExists {
				continue
			}
			return fmt.Errorf("ensure index %q: %w", ix.field, err)
		}
	}
	return nil
}

func payload(m Memory) map[string]any {
	tags := make([]any, len(m.Tags))
	for i, t := range m.Tags {
		tags[i] = t
	}
	p := map[string]any{
		"content":       m.Content,
		"scope":         m.Scope,
		"repo":          m.Repo,
		"workspace":     m.Workspace,
		"worktree_path": m.Worktree,
		"base_dir":      m.BaseDir,
		"source":        m.Source,
		"category":      m.Category,
		"tags":          tags,
		"actor":         m.Actor,
		"owner":         m.Owner,
		"visibility":    m.Visibility,
		"created_at":    m.CreatedAt.Format(time.RFC3339),
	}
	if m.NotBefore != nil {
		p["not_before"] = m.NotBefore.Unix()
	}
	if m.NotAfter != nil {
		p["not_after"] = m.NotAfter.Unix()
	}
	if len(m.Supersedes) > 0 {
		ids := make([]any, len(m.Supersedes))
		for i, id := range m.Supersedes {
			ids[i] = id
		}
		p["supersedes"] = ids
	}
	if m.SupersededBy != nil {
		p["superseded_by"] = *m.SupersededBy
	}
	if m.ArchivedAt != nil {
		p["archived_at"] = m.ArchivedAt.Unix()
	}
	p["access_count"] = m.AccessCount
	p[embedderIdentityKey] = m.EmbedderIdentity
	p[idempotencyFingerprintKey] = m.IdempotencyFingerprint
	// int(...) is load-bearing, not cosmetic: qdrant.NewValueMap's type
	// switch matches exact concrete types, and migrate.Version (a named
	// type over int) does not match its `case int:` arm — an unconverted
	// migrate.Version value falls through to NewValue's default case and
	// panics with "invalid type: migrate.Version".
	p[schemaVersionKey] = int(max(migrate.CurrentVersion, m.SchemaVersion))
	if m.LastAccessedAt != nil {
		p["last_accessed_at"] = m.LastAccessedAt.Format(time.RFC3339)
	}
	p["summary"] = m.Summary
	p["summary_source"] = string(m.SummarySource)
	if m.SummaryModel != "" {
		p["summary_model"] = m.SummaryModel
	}
	if !m.SummaryEgressAt.IsZero() {
		p["summary_egress_at"] = m.SummaryEgressAt.Format(time.RFC3339)
	}
	if m.ShortID != "" {
		p["short_id"] = m.ShortID
	}
	if m.Category == "discovery" {
		p["kind"] = m.Kind
	}
	if len(m.Citations) > 0 {
		cites := make([]any, len(m.Citations))
		for i, c := range m.Citations {
			cites[i] = map[string]any{
				"kind": c.Kind, "ref": c.Ref, "locator": c.Locator,
				"pin": c.Pin, "excerpt": c.Excerpt,
			}
		}
		p["citations"] = cites
	}
	return p
}

func fromPayload(id string, p map[string]*qdrant.Value) Memory {
	m := Memory{ID: id}
	if v, ok := p["content"]; ok {
		m.Content = v.GetStringValue()
	}
	if v, ok := p["scope"]; ok {
		m.Scope = v.GetStringValue()
	}
	if v, ok := p["short_id"]; ok {
		m.ShortID = v.GetStringValue()
	}
	if v, ok := p["repo"]; ok {
		m.Repo = v.GetStringValue()
	}
	if v, ok := p["workspace"]; ok {
		m.Workspace = v.GetStringValue()
	}
	if v, ok := p["worktree_path"]; ok {
		m.Worktree = v.GetStringValue()
	}
	if v, ok := p["base_dir"]; ok {
		m.BaseDir = v.GetStringValue()
	}
	if v, ok := p["source"]; ok {
		m.Source = v.GetStringValue()
	}
	if v, ok := p["category"]; ok {
		m.Category = v.GetStringValue()
	}
	if v, ok := p["actor"]; ok {
		m.Actor = v.GetStringValue()
	}
	if v, ok := p["owner"]; ok {
		m.Owner = v.GetStringValue()
	}
	if v, ok := p["visibility"]; ok {
		m.Visibility = v.GetStringValue()
	}
	m.Tags = tagsFromPayload(p)
	if v, ok := p["created_at"]; ok {
		if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
			m.CreatedAt = t
		}
	}
	if v, ok := p["not_before"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotBefore = &t
	}
	if v, ok := p["not_after"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotAfter = &t
	}
	m.Supersedes = supersedesFromPayload(p)
	if v, ok := p["superseded_by"]; ok {
		s := v.GetStringValue()
		m.SupersededBy = &s
	}
	if v, ok := p["archived_at"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.ArchivedAt = &t
	}
	if v, ok := p["access_count"]; ok {
		m.AccessCount = uint64(v.GetIntegerValue())
	}
	if v, ok := p[schemaVersionKey]; ok {
		m.SchemaVersion = migrate.Version(v.GetIntegerValue())
	}
	if v, ok := p[embedderIdentityKey]; ok {
		m.EmbedderIdentity = v.GetStringValue()
	}
	if v, ok := p[idempotencyFingerprintKey]; ok {
		m.IdempotencyFingerprint = v.GetStringValue()
	}
	if v, ok := p["last_accessed_at"]; ok {
		if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
			m.LastAccessedAt = &t
		}
	}
	if v, ok := p["kind"]; ok {
		m.Kind = v.GetStringValue()
	}
	if v, ok := p["summary"]; ok {
		m.Summary = v.GetStringValue()
	}
	if v, ok := p["summary_source"]; ok {
		m.SummarySource = SummarySource(v.GetStringValue())
	}
	if v, ok := p["summary_model"]; ok {
		m.SummaryModel = v.GetStringValue()
	}
	if v, ok := p["summary_egress_at"]; ok {
		if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
			m.SummaryEgressAt = t
		}
	}
	if v, ok := p["citations"]; ok {
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				sv := item.GetStructValue()
				if sv == nil {
					continue // skip malformed (non-struct) list items
				}
				f := sv.GetFields()
				m.Citations = append(m.Citations, Citation{
					Kind:    f["kind"].GetStringValue(),
					Ref:     f["ref"].GetStringValue(),
					Locator: f["locator"].GetStringValue(),
					Pin:     f["pin"].GetStringValue(),
					Excerpt: f["excerpt"].GetStringValue(),
				})
			}
		}
	}
	return m
}

// Upsert inserts or replaces a memory (same ID replaces in place).
func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Upsert",
		trace.WithAttributes(attribute.String("engram.scope", m.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Upsert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(m.ID),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(payload(m)),
		}},
	})
	return err
}

// ownerOrSharedCondition matches records the subject may READ. The own/shared
// bucket decisions are policy-derived (s.authz.DecideBucket), but the emitted
// filter shape is numerically identical to the pre-Cedar hardcoded switch
// (D-11):
//
// Authenticated: owner==sub OR visibility=="shared" (both buckets normally
// allowed — a Should composition, so an own+shared record still matches once).
// Anonymous: owner=="" ONLY — shared records require an authenticated subject
// (DecideBucket(BucketShared) denies the anonymous principal); the anonymous
// bucket is not a back-door to all shared records.
// nil/unknown (a discarded extraction error): matchNothing — fail closed,
// WITHOUT consulting the PDP (principalParams returns ok=false).
// A decision allowing zero buckets (e.g. an all-deny PDP) also compiles to
// matchNothing — never an unfiltered query.
func (s *Store) ownerOrSharedCondition(ctx context.Context, subj Subject) *qdrant.Condition {
	owner, kind, ok := principalParams(subj)
	if !ok {
		return matchNothing()
	}
	ownAllowed := s.decideBucket(ctx, owner, kind, authz.ActionRead, authz.BucketOwn).Allow
	sharedAllowed := s.decideBucket(ctx, owner, kind, authz.ActionRead, authz.BucketShared).Allow
	var should []*qdrant.Condition
	if ownAllowed {
		should = append(should, qdrant.NewMatch("owner", owner))
	}
	if sharedAllowed {
		should = append(should, qdrant.NewMatch("visibility", visibilityShared))
	}
	if len(should) == 0 {
		return matchNothing()
	}
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should})
}

// ownerOnlyCondition restricts to records the caller OWNS — no shared-read grant.
// It backs management views (ListScheduled) where a `shared` record belonging to
// another actor must stay invisible: a shared+scheduled memory is hidden from
// everyone but its owner until it becomes active (then normal recall surfaces it).
// The own-bucket decision is policy-derived (s.authz.DecideBucket); fail-closed
// for nil/unknown Subjects and a denied own bucket, exactly like
// ownerOrSharedCondition — WITHOUT consulting the PDP for nil/unknown.
func (s *Store) ownerOnlyCondition(ctx context.Context, subj Subject) *qdrant.Condition {
	owner, kind, ok := principalParams(subj)
	if !ok {
		return matchNothing()
	}
	if s.decideBucket(ctx, owner, kind, authz.ActionRead, authz.BucketOwn).Allow {
		return qdrant.NewMatch("owner", owner)
	}
	return matchNothing()
}

// decideBucket is the single call-site indirection ownerOrSharedCondition and
// ownerOnlyCondition use to reach the PDP. It defaults to s.authz.DecideBucket;
// decideBucketHook (nil in production) lets tests observe/count invocations.
//
// This is one of the two chokepoints (with decideRecord) every production
// Decision consumption funnels through, so it is the sole place a debug-level
// decision-diagnostics line is emitted for bucket-level decisions (D-01/D-04):
// ownerOrSharedCondition/ownerOnlyCondition build a filter CONDITION once per
// request (at most two calls for a bulk Search/List), never once per result
// row (RESEARCH § Pattern 3). The call is unconditional — both the allow and
// the deny arm log, never gated on d.Allow — so "both arms are logged" is
// structurally true rather than a claim about two branches staying in sync.
func (s *Store) decideBucket(ctx context.Context, owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision {
	var d authz.Decision
	if s.decideBucketHook != nil {
		d = s.decideBucketHook(owner, kind, action, bucket)
	} else {
		d = s.authz.DecideBucket(owner, kind, action, bucket)
	}
	dl := d.Log()
	slog.DebugContext(ctx, "authz decision (bucket)",
		"allow", dl.Allow,
		"action", string(action),
		"bucket", bucket.String(),
		"policy_ids", dl.PolicyIDs,
		"policy_error_count", dl.ErrorCount,
	)
	return d
}

// decideRecord is the single call-site indirection GetReadable/getWritable/
// OwnedOrAbsent use to reach the PDP for a per-record (id-addressed) decision.
// It defaults to s.authz.DecideRecord; decideRecordHook (nil in production)
// lets tests inject an all-deny probe without a real *authz.PDP construction.
//
// The other of the two chokepoints (see decideBucket): id-addressed gates are
// single-record, so this is at most one debug line per id-addressed op, never
// O(result count). No `bucket` field — a per-record decision has no bucket
// (D-02's field list names bucket for the bucket arm only).
func (s *Store) decideRecord(ctx context.Context, owner, kind string, action authz.Action, memoryOwner, category, visibility, scope string) authz.Decision {
	var d authz.Decision
	if s.decideRecordHook != nil {
		d = s.decideRecordHook(owner, kind, action, memoryOwner, category, visibility, scope)
	} else {
		d = s.authz.DecideRecord(owner, kind, action, memoryOwner, category, visibility, scope)
	}
	dl := d.Log()
	slog.DebugContext(ctx, "authz decision (record)",
		"allow", dl.Allow,
		"action", string(action),
		"policy_ids", dl.PolicyIDs,
		"policy_error_count", dl.ErrorCount,
	)
	return d
}

// matchNothing returns a condition no record can satisfy (owner==x AND owner!=x).
// It backs the fail-closed default arm of read-filter switches when the Subject
// is nil/unknown — a query then returns zero rows rather than over-returning.
func matchNothing() *qdrant.Condition {
	const x = "\x00engram-no-such-owner"
	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		Must:    []*qdrant.Condition{qdrant.NewMatch("owner", x)},
		MustNot: []*qdrant.Condition{qdrant.NewMatch("owner", x)},
	})
}

// ownerScopeFilter restricts to a scope AND the caller's readable set (see
// ownerOrSharedCondition for anonymous vs authenticated semantics). An empty
// scope now means "every scope the caller may read" (cross-spine): the scope
// match is appended to Must only when scope != "", mirroring SearchDiscovery
// (store.go:977-987) rather than inventing a variant. The
// ownerOrSharedCondition authz element is deliberately OUTSIDE that
// conditional and always appended — this composition never depends on the
// scope value, so widening what an empty scope matches cannot weaken or drop
// the authz clause. internal/server's effectiveSearchScope is the sole guard
// against an accidental empty scope reaching this function (D-03/D-07): this
// function itself carries no cross-spine flag or opinion, by design.
func (s *Store) ownerScopeFilter(ctx context.Context, scope string, subj Subject) *qdrant.Filter {
	must := make([]*qdrant.Condition, 0, 2)
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	must = append(must, s.ownerOrSharedCondition(ctx, subj))
	return &qdrant.Filter{Must: must}
}

// tagMatchConditions returns one exact-match condition per requested tag, with
// AND semantics: appended to a filter's Must, they require a record to carry
// every listed tag. Qdrant matches a scalar value against a list-valued payload
// field by membership, so NewMatch("tags", t) means "the tags list contains t".
// Empty/nil tags yields no conditions — a passthrough, never a contradiction.
// Empty-string elements are skipped: a stored tag is never empty, so matching on
// "" is meaningless and its filter behavior is implementation-defined; dropping
// it makes [""] a passthrough and ["go", ""] equivalent to ["go"].
func tagMatchConditions(tags []string) []*qdrant.Condition {
	conds := make([]*qdrant.Condition, 0, len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		conds = append(conds, qdrant.NewMatch("tags", t))
	}
	return conds
}

// categoryMatchCondition builds the OR-composed category filter shared by
// listFilter and Search: unlike tagMatchConditions' AND semantics (every
// listed tag must be present), a record matches if its category is ANY of
// the listed values (D-08). Empty/nil categories yields nil — a passthrough,
// never a contradiction. Empty-string elements are skipped, mirroring
// tagMatchConditions, so [""] is a passthrough and ["decision", ""] is
// equivalent to ["decision"]. There is no server-side allowlist on the
// filter value (D-11): an unknown category (e.g. "nonexistent") is passed to
// Qdrant as an opaque match that simply matches nothing, never an error —
// the legitimate filter domain (which includes discovery and rule) is
// strictly larger than the write domain.
func categoryMatchCondition(categories []string) *qdrant.Condition {
	should := make([]*qdrant.Condition, 0, len(categories))
	for _, c := range categories {
		if c == "" {
			continue
		}
		should = append(should, qdrant.NewMatch("category", c))
	}
	if len(should) == 0 {
		return nil
	}
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should})
}

// activeWindowConditions gates recall to records whose validity window is open
// at now: (not_before absent OR <= now) AND (not_after absent OR > now). Stored
// window keys are epoch-second integers; the Range bound is *float64 (Qdrant's
// Range field type). Records with no window match via NewIsEmpty — unchanged
// behavior for every pre-feature record. not_after is exclusive (expires AT it).
func activeWindowConditions(now time.Time) []*qdrant.Condition {
	sec := float64(now.Unix())
	// Separate *float64 allocations per bound: proto message field pointers are
	// independently owned, so the two Range structs must not alias one pointer.
	return []*qdrant.Condition{
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_before"),
		}}),
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_after", &qdrant.Range{Gt: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_after"),
		}}),
	}
}

// recallIDCap bounds the number of record ids attached to a recall span
// (engram.recall.ids) so that a large-k recall cannot drive unbounded span
// attribute cardinality (D-06, T-12-11 mitigation). It is analytics-only: it
// never reads or writes access_count and has no effect on ranking or the
// recall gate (D-02/D-08).
const recallIDCap = 50

// recallIDs returns up to limit record ids from out, preserving order. Used
// solely to populate the engram.recall.ids span attribute on the
// store.Search/List/Get recall spans.
func recallIDs(out []Memory, limit int) []string {
	n := len(out)
	if n > limit {
		n = limit
	}
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = out[i].ID
	}
	return ids
}

// SearchOptions parameterizes Search/SearchReranked's request-supplied filter
// set. k is deliberately NOT a field here (D-09): SearchReranked's k==0
// ErrInvalidArgument guard is a caller-default-discipline check that a
// buried-in-struct k would weaken, so k stays a positional parameter on both
// Search and SearchReranked.
type SearchOptions struct {
	// Tags retains List's existing contains-ALL (AND) semantics: empty = all
	// tags; non-empty = records carrying every listed tag.
	Tags []string
	// Categories: empty = all categories; non-empty = records in ANY of the
	// listed categories (OR) — the opposite composition from Tags, per D-08.
	Categories []string
	// Half-open creation-time window. Zero value = unbounded on that side.
	CreatedAfter  time.Time
	CreatedBefore time.Time
	// IncludeArchived, IncludeSuperseded, and IncludeScheduled are orthogonal
	// opt-in relaxations of Store.Search's recall gate (phase 07, D-01/D-02),
	// mirroring ListOptions' fields of the same name. Each maps 1:1 onto
	// exactly one Must condition Search appends below: IncludeArchived
	// relaxes the archived_at IsEmpty gate, IncludeSuperseded relaxes the
	// superseded_by IsEmpty gate, and IncludeScheduled relaxes the ENTIRE
	// activeWindowConditions append (both the not_before and not_after
	// halves) as one unit — never split across two branches. All three
	// default false, reproducing today's behavior byte-identically.
	// SearchReranked forwards these unchanged; the MCP lane never sets them.
	//
	// Deliberate 2-of-4 scope: this idiom (IsEmpty("superseded_by") /
	// IsEmpty("archived_at")) appears at four sites in this file. Only
	// Store.Search and Store.List honor these fields, per D-01.
	// Store.SearchDiscovery and Store.ListScheduled are deliberately
	// EXCLUDED — see their own doc comments for why.
	IncludeArchived   bool
	IncludeSuperseded bool
	IncludeScheduled  bool
}

// Search returns the k nearest readable memories to vec within scope.
// Authenticated callers see their own records plus shared records; anonymous
// callers see only the ownerless bucket.
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, opts SearchOptions) (out []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.Search", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.Int64("engram.k", int64(k)),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Search", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int("engram.result_count", len(out)),
				attribute.StringSlice("engram.recall.ids", recallIDs(out, recallIDCap)),
				attribute.Int("engram.recall.count", len(out)),
			)
		}
	}()

	f := s.ownerScopeFilter(ctx, scope, subj)
	// IncludeScheduled relaxes the ENTIRE activeWindowConditions append as one
	// unit (both the not_before and not_after halves) — never split across two
	// branches (phase 07, D-01/D-02). Splitting it would make a genuinely
	// expired record reachable while relaxing only the not-yet-active half,
	// which is exactly the failure the STATE column's "expired" word exists to
	// prevent.
	if !opts.IncludeScheduled {
		f.Must = append(f.Must, activeWindowConditions(s.now())...)
	}
	// Soft-hide superseded records from search recall (D-09) — sibling
	// condition to activeWindowConditions, not folded into it (that helper is
	// time-window-scoped only). get_memory (Store.Get) stays ungated.
	// IncludeSuperseded relaxes this condition only (phase 07, D-01/D-02).
	if !opts.IncludeSuperseded {
		f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
	}
	// Soft-hide archived records (D-12, plan 03-06) — a SIBLING condition to
	// both activeWindowConditions and the superseded_by IsEmpty above, never
	// folded into either: archived is orthogonal to both the time window and
	// supersession, so this stays independently maintained alongside them.
	// get_memory (Store.Get) stays ungated. IncludeArchived relaxes this
	// condition only (phase 07, D-01/D-02).
	if !opts.IncludeArchived {
		f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
	}
	f.Must = append(f.Must, tagMatchConditions(opts.Tags)...)
	if c := categoryMatchCondition(opts.Categories); c != nil {
		f.Must = append(f.Must, c)
	}
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: f, Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}

// memoriesFromPoints decodes Qdrant scored points into Memory records.
func memoriesFromPoints(res []*qdrant.ScoredPoint) []Memory {
	out := make([]Memory, 0, len(res))
	for _, p := range res {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		m.Score = p.Score
		out = append(out, m)
	}
	return out
}

// SearchReranked is the shared search-with-rerank helper: it over-fetches
// candidateK(k) raw hits via the existing owner/scope-filtered Search, applies
// the pure RerankHits lexical-overlap reorder, and truncates to the caller's
// already-defaulted k. deps.searchMemory (MCP), engramAPI.SearchMemories
// (Connect), and the retrieval eval all call this — the ONE ranking path for
// every recall surface (review finding 2/5) — so no surface can drift from the
// shipped rerank behavior, and reranking runs strictly AFTER
// ownerScopeFilter's authz-scoped Query, never widening visibility.
//
// k == 0 is rejected with ErrInvalidArgument (round-2 finding 6): callers MUST
// pass the already-defaulted effective k (MCP defaults 8 at tools.go, Connect
// defaults 20 at connectapi.go — BEFORE calling this helper) — a zero k never
// silently over-fetches candidateK then truncates to an empty result.
//
// SearchReranked takes plain inputs (query text, query vector, k) and does NOT
// import internal/embed or internal/server (round-2 finding 7): embedding
// happens in the caller before this is invoked; this helper only reorders an
// already-fetched, already-authorized []Memory.
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 {
		return nil, fmt.Errorf("%w: SearchReranked requires k > 0 (caller must apply its default before calling)", ErrInvalidArgument)
	}
	hits, err := s.Search(ctx, scope, subj, vec, candidateK(k), opts)
	if err != nil {
		return nil, err
	}
	return RerankHits(query, hits, int(k)), nil
}

// SearchDiscovery runs a top-k vector search constrained to discovery records.
// Empty scope spans all discovery scopes (the cross_spine case); empty kind
// matches both map and fact. subj restricts results via ownerOrSharedCondition:
// authenticated callers see own + shared records (cross_spine = my+shared);
// anonymous callers see only ownerless records — shared requires an
// authenticated subject. Builds a compound exact-match filter — no prefix
// matching.
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string, subj Subject, vec []float32, k uint64) (out []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.SearchDiscovery", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.kind", kind),
		attribute.Int64("engram.k", int64(k)),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SearchDiscovery", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	must = append(must, s.ownerOrSharedCondition(ctx, subj))
	// Soft-hide superseded discoveries from search recall (WR-01), the same
	// gate Search/List already apply — get_memory stays ungated.
	must = append(must, qdrant.NewIsEmpty("superseded_by"))
	// Soft-hide archived discoveries (D-12, plan 03-06) — a SIBLING condition,
	// never folded into the superseded_by gate above: archived and superseded
	// are independently observable states. get_memory stays ungated.
	must = append(must, qdrant.NewIsEmpty("archived_at"))
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: &qdrant.Filter{Must: must}, Limit: qdrant.PtrOf(k),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}

// ListOptions parameterizes List: page window (Limit/Offset) and the server-side
// category/visibility filters the operator console applies. Zero value = first
// page, no filters.
type ListOptions struct {
	Limit      uint64
	Offset     uint64
	Categories []string // empty = all
	Visibility string   // "" = all | "private" | "shared"
	Tags       []string // empty = all; non-empty = records carrying ALL listed tags. Honored by List (and Search via its tags param); ListScheduled ignores it.
	// Half-open creation-time window. Zero value = unbounded on that side.
	// CreatedAfter is inclusive (gte); CreatedBefore is exclusive (lt).
	CreatedAfter  time.Time
	CreatedBefore time.Time
	// Cursor carries an opaque keyset token to resume from; "" starts at the
	// first page. Cursor mode is selected by Cursor != "" || CursorMode (a non-empty
	// Cursor alone is enough, even with CursorMode false). Mutually exclusive with
	// Offset > 0.
	Cursor     string
	CursorMode bool // true = boundary-id-set cursor paging (MCP default); false = offset paging (UI)
	// Ascending flips the offset-mode created_at ordering from the default
	// descending (recency-first recall) to ascending (oldest-first). Honored
	// only on the offset/all path used by list_rules; cursor mode is unaffected
	// (rules do not paginate). Zero value preserves the existing desc behavior.
	Ascending bool
	// IncludeArchived, IncludeSuperseded, and IncludeScheduled are orthogonal
	// opt-in relaxations of Store.List's recall gate (phase 07, D-01/D-02).
	// Each maps 1:1 onto exactly one Must condition List appends below:
	// IncludeArchived relaxes the archived_at IsEmpty gate, IncludeSuperseded
	// relaxes the superseded_by IsEmpty gate, and IncludeScheduled relaxes the
	// ENTIRE activeWindowConditions append (both the not_before and not_after
	// halves) as one unit — never split across two branches. All three
	// default false, reproducing today's behavior byte-identically. The MCP
	// lane never sets these fields.
	//
	// Deliberate 2-of-4 scope: this idiom (IsEmpty("superseded_by") /
	// IsEmpty("archived_at")) appears at four sites in this file. Only
	// Store.Search and Store.List honor these fields, per D-01.
	// Store.SearchDiscovery and Store.ListScheduled are deliberately
	// EXCLUDED — ListScheduled accepts a ListOptions value but must NOT
	// inherit these fields; it has its own inline filter and its own state
	// semantics (ScheduledState).
	IncludeArchived   bool
	IncludeSuperseded bool
	IncludeScheduled  bool
}

// createdRangeCondition builds a half-open [after, before) DatetimeRange on the
// created_at datetime index: after→Gte (inclusive), before→Lt (exclusive). A
// zero bound is omitted. Returns nil when both bounds are zero (no filter).
func createdRangeCondition(after, before time.Time) *qdrant.Condition {
	if after.IsZero() && before.IsZero() {
		return nil
	}
	dr := &qdrant.DatetimeRange{}
	if !after.IsZero() {
		dr.Gte = timestamppb.New(after.UTC())
	}
	if !before.IsZero() {
		dr.Lt = timestamppb.New(before.UTC())
	}
	return qdrant.NewDatetimeRange("created_at", dr)
}

// listFilter builds the Qdrant filter for List: scope (when non-empty; an
// empty scope spans every scope the caller may read — cross-spine, D-08) +
// per-actor authz (outer Must constraint, unconditional) AND optional
// category/visibility request filters. The authz condition stays the outer
// Must, so no filter combination can reach another actor's records.
//
// Visibility semantics:
//   - "" (empty): no visibility filter — return all readable records.
//   - "shared": match records with stored visibility=="shared".
//   - "private": match records whose stored visibility is "" (the canonical
//     private representation — the store only ever writes "" or "shared"). This
//     is expressed as MustNot(visibility=="shared") so that an empty-string match
//     in Qdrant is reliable across payload-key-absent and empty-value cases.
func (s *Store) listFilter(ctx context.Context, scope string, subj Subject, opts ListOptions) *qdrant.Filter {
	must := make([]*qdrant.Condition, 0, 2)
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	must = append(must, s.ownerOrSharedCondition(ctx, subj))
	if c := categoryMatchCondition(opts.Categories); c != nil {
		must = append(must, c)
	}
	must = append(must, tagMatchConditions(opts.Tags)...)
	switch opts.Visibility {
	case visibilityShared:
		must = append(must, qdrant.NewMatch("visibility", visibilityShared))
	case "private":
		// Private records are stored with visibility=="" (empty string). Use
		// MustNot(visibility=="shared") rather than matching empty directly, because
		// Qdrant's NewMatch on an empty string may not reliably match absent or
		// empty-value keys across all payload states.
		return &qdrant.Filter{
			Must:    must,
			MustNot: []*qdrant.Condition{qdrant.NewMatch("visibility", visibilityShared)},
		}
	}
	return &qdrant.Filter{Must: must}
}

// List returns a CreatedAt-ordered page of the caller's readable records in scope
// — descending by default, ascending when ListOptions.Ascending is set (offset/all
// mode only) — the exact matched total (server-side Count), and a nextCursor (empty
// in offset mode; populated only by cursor-mode paging). Ordering and the total are
// computed server-side. When Offset >= total, the page is empty (clamped, never a
// slice panic) and total is still the real matched count.
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) (items []Memory, total uint64, nextCursor string, err error) {
	ctx, span := tracer.Start(ctx, "store.List", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "List", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int("engram.result_count", len(items)),
				attribute.StringSlice("engram.recall.ids", recallIDs(items, recallIDCap)),
				attribute.Int("engram.recall.count", len(items)),
			)
		}
	}()

	if (opts.Cursor != "" || opts.CursorMode) && opts.Offset > 0 {
		return nil, 0, "", fmt.Errorf("list: cursor mode and offset are mutually exclusive: %w", ErrInvalidArgument)
	}
	if opts.Ascending && (opts.Cursor != "" || opts.CursorMode) {
		return nil, 0, "", fmt.Errorf("list: ascending ordering is honored only in offset/all mode, not cursor mode: %w", ErrInvalidArgument)
	}

	f := s.listFilter(ctx, scope, subj, opts)
	// IncludeScheduled relaxes the ENTIRE activeWindowConditions append as one
	// unit (both the not_before and not_after halves) — never split across two
	// branches (phase 07, D-01/D-02). Splitting it would make a genuinely
	// expired record reachable while relaxing only the not-yet-active half,
	// which is exactly the failure the STATE column's "expired" word exists to
	// prevent.
	if !opts.IncludeScheduled {
		f.Must = append(f.Must, activeWindowConditions(s.now())...)
	}
	// Soft-hide superseded records from list recall (D-09) — sibling
	// condition to activeWindowConditions, not folded into it (that helper is
	// time-window-scoped only). get_memory (Store.Get) stays ungated.
	// IncludeSuperseded relaxes this condition only (phase 07, D-01/D-02).
	if !opts.IncludeSuperseded {
		f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
	}
	// Soft-hide archived records (D-12, plan 03-06) — a SIBLING condition,
	// never folded into activeWindowConditions or the superseded_by gate
	// above: archived, expired, and superseded stay independently observable
	// states. get_memory (Store.Get) stays ungated. IncludeArchived relaxes
	// this condition only (phase 07, D-01/D-02).
	if !opts.IncludeArchived {
		f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
	}
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}

	// Exact total over the filtered set (replaces the scanCap approximation).
	total, err = s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return nil, 0, "", err
	}

	if opts.Cursor != "" || (opts.Offset == 0 && opts.Limit > 0 && opts.CursorMode) {
		items, nextCursor, err = s.listByCursor(ctx, f, opts)
		return items, total, nextCursor, err
	}

	// Offset mode: Qdrant has no numeric OFFSET, so scroll offset+limit ordered
	// records and return the trailing limit.
	fetch := opts.Offset + opts.Limit
	if opts.Limit == 0 {
		fetch = total // limit 0 = "all" (preserves prior behavior)
	}
	if fetch == 0 {
		// Reached only when Limit==0 ("all") and the filtered set is empty
		// (total==0): Qdrant's Scroll rejects Limit=0 ("must be 1 or larger")
		// and there is nothing to fetch — short-circuit to an empty page.
		return []Memory{}, total, "", nil
	}
	dir := qdrant.Direction_Desc
	if opts.Ascending {
		dir = qdrant.Direction_Asc
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy:        &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(dir)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, 0, "", err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	if opts.Offset >= uint64(len(all)) {
		return []Memory{}, total, "", nil
	}
	return all[opts.Offset:], total, "", nil
}

// maxListLimit caps a single cursor page (and bounds a decoded cursor's seen
// set), so neither a large Limit nor a crafted cursor can drive an unbounded
// Scroll over-fetch.
const maxListLimit = 1000

// listByCursor implements boundary-id-set keyset paging over the already-built
// filter f. opts.Cursor may be empty (first page); a non-empty cursor resumes at
// its created_at boundary, dropping ids already emitted at that exact timestamp.
func (s *Store) listByCursor(ctx context.Context, f *qdrant.Filter, opts ListOptions) ([]Memory, string, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var startFrom *qdrant.StartFrom
	seen := map[string]bool{}
	var boundary string
	if opts.Cursor != "" {
		c, err := decodeCursor(opts.Cursor)
		if err != nil {
			return nil, "", fmt.Errorf("list cursor: %w: %w", err, ErrInvalidArgument)
		}
		if len(c.Seen) > maxListLimit {
			return nil, "", fmt.Errorf("list cursor: seen set too large: %w", ErrInvalidArgument)
		}
		boundary = c.C
		startFrom = qdrant.NewStartFromDatetime(c.C)
		for _, id := range c.Seen {
			seen[id] = true
		}
	}

	// Over-fetch limit + len(seen) + 1: len(seen) covers the boundary ids dropped
	// at resume, and the +1 guarantees forward progress (a full page yields a
	// usable next cursor rather than silently terminating).
	fetch := limit + uint64(len(seen)) + 1
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy: &qdrant.OrderBy{
			Key:       "created_at",
			Direction: qdrant.PtrOf(qdrant.Direction_Desc),
			StartFrom: startFrom,
		},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, "", err
	}

	out := make([]Memory, 0, limit)
	for _, p := range pts {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		ts := m.CreatedAt.UTC().Format(time.RFC3339)
		if ts == boundary && seen[m.ID] {
			continue // already emitted at this exact timestamp
		}
		out = append(out, m)
		if uint64(len(out)) == limit {
			break
		}
	}

	if uint64(len(out)) < limit {
		return out, "", nil // exhausted: no next page
	}

	// Build next cursor from the last emitted record: c = its created_at, seen =
	// every emitted id sharing that timestamp (so the next page drops them).
	last := out[len(out)-1]
	nextC := last.CreatedAt.UTC().Format(time.RFC3339)
	nextSeen := make([]string, 0, 4)
	// Carry forward prior seen ids if the boundary did not advance. These are
	// disjoint from the emitted ids appended below: any emitted record at the
	// boundary timestamp passed the seen[m.ID] drop, so it was never in seen.
	if nextC == boundary {
		for id := range seen {
			nextSeen = append(nextSeen, id)
		}
	}
	// Emitted records are distinct point ids, so appending those at nextC adds no
	// duplicate — the next seen set is duplicate-free by construction.
	for _, m := range out {
		if m.CreatedAt.UTC().Format(time.RFC3339) == nextC {
			nextSeen = append(nextSeen, m.ID)
		}
	}
	return out, encodeCursor(listCursor{C: nextC, Seen: nextSeen}), nil
}

// ScheduledState selects which hidden-by-the-recall-gate records ListScheduled
// returns. Active (currently-valid) windowed records are never returned here —
// they surface through normal Search/List.
type ScheduledState string

// The recognized ScheduledState values; each inline comment gives the recall-gate
// predicate it selects.
const (
	ScheduledPending ScheduledState = "scheduled" // now < not_before (not yet active)
	ScheduledExpired ScheduledState = "expired"   // now >= not_after (already lapsed)
	ScheduledAll     ScheduledState = "all"       // union of pending and expired
)

// valid reports whether the state is one of the three recognized values. The
// store guards on this so a caller that bypasses the handler's validation cannot
// silently fall through scheduledStateCondition's default to ScheduledPending.
func (st ScheduledState) valid() bool {
	switch st {
	case ScheduledPending, ScheduledExpired, ScheduledAll:
		return true
	}
	return false
}

// scheduledStateCondition returns the inverse-window clause for a state. now is
// the comparison instant; its epoch seconds become the *float64 Qdrant Range bound.
func scheduledStateCondition(state ScheduledState, now time.Time) *qdrant.Condition {
	sec := float64(now.Unix())
	// Separate *float64 allocations per bound: proto message field pointers are
	// independently owned and must not alias.
	pending := qdrant.NewRange("not_before", &qdrant.Range{Gt: qdrant.PtrOf(sec)})
	expired := qdrant.NewRange("not_after", &qdrant.Range{Lte: qdrant.PtrOf(sec)})
	switch state {
	case ScheduledExpired:
		return expired
	case ScheduledAll:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{pending, expired}})
	default: // ScheduledPending
		return pending
	}
}

// ListScheduled returns the caller's OWN windowed records that the recall gate is
// hiding, for management (review/reschedule/delete). It applies the INVERSE
// temporal clause and an owner-only authz envelope (ownerOnlyCondition, NOT the
// shared-read grant): a `shared` scheduled/expired record belonging to another
// actor stays invisible here until it becomes active, preserving the deferred-
// reveal guarantee. It does not reuse List (whose gate would exclude exactly
// these records). Server-side order_by created_at desc bounded directly by
// opts.Limit (default 20); the created_at window (opts.CreatedAfter/Before) is
// applied as a DatetimeRange. opts.Offset is ignored — paginates by Limit alone.
func (s *Store) ListScheduled(ctx context.Context, scope string, subj Subject, state ScheduledState, opts ListOptions) (items []Memory, err error) {
	if !state.valid() {
		return nil, fmt.Errorf("invalid scheduled state %q (want scheduled|expired|all)", state)
	}
	ctx, span := tracer.Start(ctx, "store.ListScheduled", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
		attribute.String("engram.scheduled_state", string(state)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListScheduled", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		s.ownerOnlyCondition(ctx, subj),
		scheduledStateCondition(state, s.now()),
		// Soft-hide superseded scheduled records from the management view
		// (WR-02) — a corrected-away record shouldn't still surface as a
		// live pending/expired candidate. get_memory stays ungated.
		qdrant.NewIsEmpty("superseded_by"),
		// Soft-hide archived scheduled records (D-12, plan 03-06) — a SIBLING
		// condition to the superseded_by gate above, never folded into it:
		// archived and superseded are independently observable states.
		// get_memory stays ungated.
		qdrant.NewIsEmpty("archived_at"),
	}}
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit:       qdrant.PtrOf(uint32(limit)),
		OrderBy:     &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	items = make([]Memory, 0, len(pts))
	for _, p := range pts {
		items = append(items, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	return items, nil
}

// ScopeCount is a scope plus the number of records in it the caller can read.
type ScopeCount struct {
	Scope string
	Count uint64
}

// ListScopes enumerates the caller's readable scopes with per-scope counts.
// Qdrant has no GROUP BY, so it scrolls the readable set (owner OR shared, across
// ALL scopes — ownerOrSharedCondition, not ownerScopeFilter which pins a scope)
// bounded by scanCap and aggregates in-process. The second return is true when
// the scan hit scanCap, meaning the counts are a bounded sample, not exact.
func (s *Store) ListScopes(ctx context.Context, subj Subject) (out []ScopeCount, more bool, err error) {
	ctx, span := tracer.Start(ctx, "store.ListScopes",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListScopes", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(out)))
		}
	}()

	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(ctx, subj)}},
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, false, err
	}
	counts := map[string]uint64{}
	for _, p := range pts {
		counts[fromPayload(p.Id.GetUuid(), p.Payload).Scope]++
	}
	out = make([]ScopeCount, 0, len(counts))
	for sc, n := range counts {
		out = append(out, ScopeCount{Scope: sc, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, len(pts) == scanCap, nil
}

// Get returns the memory with the given id.
func (s *Store) Get(ctx context.Context, id string) (m Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.Get")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Get", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return Memory{}, err
	}
	if len(pts) == 0 {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	span.SetAttributes(
		attribute.StringSlice("engram.recall.ids", []string{id}),
		attribute.Int("engram.recall.count", 1),
	)
	return fromPayload(id, pts[0].Payload), nil
}

// ResolvePointID maps a caller-supplied identifier — a full UUID (any form
// uuid.Parse accepts) or a short id — to the canonical Qdrant point UUID. It is
// owner-agnostic and applies NO authz: the caller's downstream ownership gate
// (GetReadable / OwnedOrAbsent / getWritable) still governs access. Trims before
// the UUID check because uuid.Parse is length-strict and rejects whitespace.
func (s *Store) ResolvePointID(ctx context.Context, idOrShort string) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.ResolvePointID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ResolvePointID", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	t := strings.TrimSpace(idOrShort)
	if t == "" {
		return "", fmt.Errorf("%w: empty id", ErrInvalidArgument)
	}
	if u, perr := uuid.Parse(t); perr == nil {
		return u.String(), nil // canonicalize URN / braced / raw-hex forms to hyphenated
	}
	canonical := shortid.Canonical(t)
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", canonical)}},
		Limit:          qdrant.PtrOf(uint32(2)),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		return "", err
	}
	switch len(pts) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrNotFound, idOrShort)
	case 1:
		return pts[0].Id.GetUuid(), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)
	}
}

// GetReadable returns the record only if the caller may READ it; otherwise
// ErrNotFound, so ownership never leaks across actors.
//
// Authenticated callers: readable if owner==sub OR visibility=="shared".
// Anonymous callers: readable only if owner=="" (ownerless bucket).
// The "shared" grant requires an authenticated subject — anonymous callers
// cannot read shared records. nil/unknown Subject → fail closed (ErrNotFound).
func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (out Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.GetReadable",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "GetReadable", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	owner, kind, ok := principalParams(subj)
	if !ok {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !s.decideRecord(ctx, owner, kind, authz.ActionRead, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}

// getWritable returns the record only if the caller OWNS it (shared does NOT
// grant write); otherwise ErrNotFound. The mutate primitive. action is the
// caller's actual verb (ActionWrite/ActionDelete/ActionShare) — getWritable
// forwards it to the PDP unchanged rather than hardcoding ActionWrite, so a
// future action-discriminating policy fires correctly for Delete/SetVisibility
// as well as Update.
//
// Owner-only: anonymous requires owner=="", authenticated requires owner==sub.
// shared visibility is irrelevant to the write gate — shared grants read, not
// write. Any owner-stamped record (owner!="") is invisible to anonymous mutation,
// preserving fail-closed write isolation even in mixed-auth deployments.
// Per-actor isolation requires authentication (see the package isolation contract
// and README).
func (s *Store) getWritable(ctx context.Context, id string, subj Subject, action authz.Action) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	owner, kind, ok := principalParams(subj)
	if !ok {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !s.decideRecord(ctx, owner, kind, action, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}

// OwnedOrAbsent permits a client-supplied-id write: nil if the id is absent (new
// record) or already owned by the subject (replace in place); ErrNotFound if it
// exists and is owned by a different actor or the subject is anonymous but the
// record has an owner (refuse cross-owner overwrite). Transport errors surface
// unchanged.
func (s *Store) OwnedOrAbsent(ctx context.Context, id string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.OwnedOrAbsent",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "OwnedOrAbsent", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	m, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	owner, kind, ok := principalParams(subj)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !s.decideRecord(ctx, owner, kind, authz.ActionWrite, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return nil
}

// FetchForUpdate returns the record iff it exists and is owned by the subject
// (the same gate as the internal write path); otherwise ErrNotFound. The update
// handler calls this once as the authoritative ownership gate BEFORE embedding,
// then hands the returned record to Update — so the update path performs a
// single Qdrant Get instead of two. The returned record carries current
// visibility, so a content-only Update (shared==nil) preserves it.
//
// Anonymous-bucket semantics preserved: Anonymous() matches owner=="" exactly as
// getWritable does, so ownerless records remain mutually writable when auth is
// disabled.
func (s *Store) FetchForUpdate(ctx context.Context, id string, subj Subject) (out Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.FetchForUpdate",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "FetchForUpdate", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	return s.getWritable(ctx, id, subj, authz.ActionWrite)
}

// updateAfterReadHook is a test-only seam: nil in production (a single
// nil-check branch), it is invoked by Update exactly once, inside the
// s.locker.Lock block, after the in-lock re-read and before the
// whole-payload Upsert. It exists to make the archive/update race window
// (plan 03-06, TestArchiveSurvivesConcurrentUpdate/
// TestRestoreSurvivesConcurrentUpdate) addressable deterministically from a
// test, instead of repeating an unsynchronized goroutine race and hoping to
// land in the vulnerable window. Must never be set outside a test —
// mirrors internal/store/spine.go's spineScrollBatch test-only var.
var updateAfterReadHook func()

// Update applies a content change (re-embedded via vec) to a record previously
// fetched and ownership-verified via FetchForUpdate. It does NOT re-fetch: cur
// is authoritative, so the update path gates ownership exactly once. When shared
// is non-nil it also sets visibility (true → "shared", false → ""); nil leaves
// visibility unchanged so a content edit never silently unshares. cur.IdempotencyFingerprint
// is carried through UNCHANGED (IN-02, see the field's doc comment): Update
// never recomputes it from the new content, since update_memory is a distinct
// mutation path outside the keyed-replay contract. tags follows
// the same presence-signal contract: non-nil replaces the full set (an empty
// slice clears), nil leaves the existing tags untouched. summary follows the
// same presence-signal contract: non-nil replaces the summary (empty string
// clears), nil leaves the existing summary untouched.
func (s *Store) Update(ctx context.Context, cur Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Update")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Update", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	// CR-04: this method's whole-payload Upsert below would otherwise silently
	// erase a concurrent Supersede's superseded_by/supersedes back-stamp if
	// that Supersede lands between the caller's FetchForUpdate snapshot (cur)
	// and this call — reproduced and confirmed against real Qdrant. Fix has
	// two parts, mirroring Supersede's own CR-01 lock (store.go:1815-1862):
	//   1. Serialize against a concurrent Supersede on the same target via the
	//      same per-target lock, so the two writers can't interleave.
	//   2. Re-read the record's current superseded_by/supersedes inside the
	//      lock and carry them into cur, so even a just-landed back-stamp
	//      survives this Upsert. A concurrent Delete (Get returns ErrNotFound)
	//      is left as pre-existing, out-of-scope behavior: Update never
	//      existence-checks cur, matching its behavior before this fix.
	// Plan 03-06 extends this same in-lock re-read to ArchivedAt, and extends
	// the lock itself to Store.Archive/Store.Restore (internal/store/spine.go)
	// for the identical reason: a lock-free targeted SetPayload landing
	// between this re-read and the Upsert below would otherwise be silently
	// erased, exactly like the CR-04 supersession case this comment describes.
	// Phase 02-01 extends the same in-lock refresh to SchemaVersion: D-05's
	// monotonic max must be computed against the LATEST STORED stamp, not
	// only the caller's FetchForUpdate snapshot — otherwise a version raised
	// by a concurrent writer between that snapshot and this re-read would be
	// silently lost when this method's Upsert recomputes the max from a
	// stale cur.SchemaVersion.
	unlock, err := s.locker.Lock(ctx, cur.ID)
	if err != nil {
		return err
	}
	defer unlock()
	if fresh, gerr := s.Get(ctx, cur.ID); gerr == nil {
		cur.Supersedes, cur.SupersededBy, cur.ArchivedAt, cur.SchemaVersion = fresh.Supersedes, fresh.SupersededBy, fresh.ArchivedAt, fresh.SchemaVersion
	} else if !errors.Is(gerr, ErrNotFound) {
		return gerr
	}
	// updateAfterReadHook is a test-only seam (nil in production, one
	// nil-check branch cost) that makes the archive/update race window
	// addressable from a test: it fires exactly here, inside the lock, after
	// the in-lock re-read above and before the whole-payload Upsert below —
	// the same window CR-04's comment describes for supersession. A test sets
	// it to force this window open on demand rather than repeating an
	// unsynchronized goroutine race and hoping to hit it; it must never be
	// set outside a test, mirroring spineScrollBatch's own test-only var.
	if updateAfterReadHook != nil {
		updateAfterReadHook()
	}

	cur.Content = content
	// D-04: the bump piggybacks the already-in-flight Upsert below — zero extra
	// store round-trip.
	cur.AccessCount++
	now := s.now()
	cur.LastAccessedAt = &now
	if shared != nil {
		if *shared {
			cur.Visibility = visibilityShared
		} else {
			cur.Visibility = ""
		}
	}
	if tags != nil {
		cur.Tags = *tags
	}
	if summary != nil {
		cur.Summary = *summary
		if *summary == "" {
			cur.SummarySource, cur.SummaryModel = SummarySourceNone, ""
		} else {
			cur.SummarySource, cur.SummaryModel = SummarySourceClient, ""
		}
		// A client-authored or cleared summary is no longer auto-egressed
		// provenance, so the egress stamp is invalidated.
		cur.SummaryEgressAt = time.Time{}
	}
	return s.Upsert(ctx, cur, vec)
}

// provenanceKeys are the auto-summary provenance payload keys UpdatePayload
// deletes (never blank-writes) when a client summary replaces or clears an
// existing auto-summary. Shared by UpdatePayload and its tests.
var provenanceKeys = []string{"summary_model", "summary_egress_at"}

// UpdatePayload applies a shared/summary-only change to a record previously
// fetched and ownership-verified via FetchForUpdate — it mirrors Update's cur
// contract exactly: it does NOT re-fetch (cur is authoritative) and it does
// NOT re-gate ownership (the FetchForUpdate gate already ran). Unlike Update,
// it never touches content, tags, or the vector: it persists via a TARGETED
// SetPayload that writes ONLY the keys this method owns (visibility, summary,
// summary_source, plus the access_count/last_accessed_at usage-signal keys),
// so the record's existing vector is preserved untouched — no re-embed, no
// vector argument.
//
// This is deliberately NOT a whole-payload OverwritePayload(payload(cur)): a
// whole-payload write against a stale FetchForUpdate snapshot is not a
// compare-and-swap, and under ordinary concurrency it can revert a concurrent
// content update's content/tags while that write's new re-embedded vector
// survives — a durable content/vector desync that corrupts recall (round-7
// HIGH). It matches this package's own SetVisibility, which likewise writes
// only the mutated key rather than rewriting the whole payload from a
// snapshot. Do not reintroduce a whole-payload write here unless a real
// optimistic-concurrency/version (CAS) mechanism is added first.
//
// shared/summary follow store.Update's exact presence-signal contract: nil
// leaves the field unchanged, non-nil sets it (empty-string summary clears).
// Like Update, the usage signal is bumped unconditionally: cur.AccessCount++
// and cur.LastAccessedAt is stamped to now, mirroring Update's own bump
// (store.go:1379-1384) so a shared/summary-only update still counts as an
// update (review finding 6).
//
// When the summary write is a CLIENT summary (set OR cleared — summary
// non-nil either way), stale auto-summary provenance (summary_model,
// summary_egress_at) is cleared via a SEPARATE, targeted Qdrant DeletePayload
// op on those two keys — never a blank/zero-time write, which would re-parse
// non-absent at the payload decoder (fromPayload, store.go:433-439) and
// misclassify the client summary as auto-generated.
//
// Partial-failure contract (round-8 — documented, not "fixed": the two-op
// SetPayload+DeletePayload sequence is deliberately NOT atomic, matching
// SetVisibility's own non-transactional single SetPayload). If the
// DeletePayload provenance-clear fails AFTER the SetPayload has already
// committed, the caller sees TWO consequences, both accepted and recoverable:
//  1. Partial success: this method returns that error even though the
//     primary mutation (visibility/summary/access-count bump) already
//     landed — a caller-visible "mutation committed but RPC failed" outcome.
//     A retry re-applies the mutation and re-increments AccessCount again
//     (the usage counter is not idempotent across retries, same as any bump).
//  2. Soft last-writer-wins RMW: AccessCount+1 is computed from the
//     FetchForUpdate snapshot with no compare-and-swap, so concurrent updates
//     on the same record can lose/regress increments — exactly the same
//     accepted tradeoff as IncrementAccess (store.go:1445-1453).
//
// The record is NEVER left with corrupted content or vector: only the
// summary_model/summary_egress_at provenance keys are potentially stale,
// which mislabels a summary's source until a later write reconciles it.
func (s *Store) UpdatePayload(ctx context.Context, cur Memory, shared *bool, summary *string) (err error) {
	ctx, span := tracer.Start(ctx, "store.UpdatePayload")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "UpdatePayload", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	clearProvenance := false
	if shared != nil {
		if *shared {
			cur.Visibility = visibilityShared
		} else {
			cur.Visibility = ""
		}
	}
	if summary != nil {
		cur.Summary = *summary
		if *summary == "" {
			cur.SummarySource, cur.SummaryModel = SummarySourceNone, ""
		} else {
			cur.SummarySource, cur.SummaryModel = SummarySourceClient, ""
		}
		// A client-authored or cleared summary is no longer auto-egressed
		// provenance; the in-memory struct is cleared to zero here (mirroring
		// Update), and the persisted keys are DELETED below — never
		// blank-written (round-5 MED, grok).
		cur.SummaryEgressAt = time.Time{}
		clearProvenance = true
	}
	cur.AccessCount++
	now := s.now()
	cur.LastAccessedAt = &now

	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{
			"visibility":       cur.Visibility,
			"summary":          cur.Summary,
			"summary_source":   string(cur.SummarySource),
			"access_count":     cur.AccessCount,
			"last_accessed_at": now.Format(time.RFC3339),
		}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(cur.ID)}),
	}); err != nil {
		return err
	}
	if !clearProvenance {
		return nil
	}
	del := s.deletePayloadKeys
	if del == nil {
		del = s.defaultDeletePayloadKeys
	}
	return del(ctx, cur.ID, provenanceKeys)
}

// defaultDeletePayloadKeys is the production deletePayloadKeys implementation:
// a targeted Qdrant DeletePayload op removing the given keys from one point.
func (s *Store) defaultDeletePayloadKeys(ctx context.Context, id string, keys []string) error {
	_, err := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Keys:           keys,
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// defaultSetPayloadKeys is the production setPayloadKeys implementation: one
// multi-ID Qdrant SetPayload op merging kv onto every point in ids.
func (s *Store) defaultSetPayloadKeys(ctx context.Context, ids []string, kv map[string]any) error {
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewID(id)
	}
	_, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(kv),
		PointsSelector: qdrant.NewPointsSelectorIDs(pointIDs),
	})
	return err
}

// defaultDeletePoint is the production deletePoint implementation: a raw,
// UNGATED Qdrant point removal for one id — no getWritable, no ActionDelete
// authz decision. Used only by reconcileSupersedeFailure, scoped to the
// survivor id minted inside the same Supersede call.
func (s *Store) defaultDeletePoint(ctx context.Context, id string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}

// SetVisibility flips a record's shared flag without re-embedding (uses
// SetPayload, preserving the vector), only if owned by subj.
//
// TOCTOU note: if the record is deleted between the getWritable ownership gate
// and the SetPayload call, Qdrant's point-ID-selector SetPayload returns a
// NotFound gRPC error (verified against v1.18.2). That error propagates
// unchanged, so SetVisibility is fail-closed with respect to concurrent
// deletion — no additional re-fetch is required.
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
	ctx, span := tracer.Start(ctx, "store.SetVisibility",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SetVisibility", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if _, err := s.getWritable(ctx, id, subj, authz.ActionShare); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// lockTargets clones, sorts, and dedupes targets, then acquires
// s.locker.Lock for each id in sorted order, accumulating unlock funcs
// (D-10). Sorted acquisition means two concurrent Supersede calls over
// overlapping-but-different target sets can never hold each other's next
// lock — deadlock-free without changing the TargetLocker interface, so a
// future distributed locker still drops in unmodified; the ordering
// discipline lives entirely at this call site.
//
// Dedup MUST happen BEFORE acquisition, not merely for tidiness:
// inProcessTargetLocker.Lock wraps a plain, non-reentrant sync.Mutex, so
// acquiring the SAME target id twice on one goroutine self-deadlocks
// permanently — no panic, no timeout, and Go's runtime deadlock detector
// does not catch a goroutine blocking on a lock it already holds.
//
// On a mid-loop acquisition error, everything already acquired is released
// (in reverse order) before returning a nil unlock and a nil sorted slice;
// on success the returned unlock func releases everything in reverse sorted
// order — callers must always invoke it, typically via defer.
func (s *Store) lockTargets(ctx context.Context, targets []string) (unlock func(), sorted []string, err error) {
	sorted = slices.Clone(targets)
	sort.Strings(sorted)
	sorted = slices.Compact(sorted)

	unlocks := make([]func(), 0, len(sorted))
	for _, id := range sorted {
		u, lerr := s.locker.Lock(ctx, id)
		if lerr != nil {
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
			return nil, nil, lerr
		}
		unlocks = append(unlocks, u)
	}
	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}, sorted, nil
}

// Supersede stores newMem (a normal, caller-owned create — same shape as
// Upsert) and back-stamps every target's superseded_by link to newMem's id
// (D-01: promoted from a single target to a set — a one-element targets
// slice behaves exactly as the pre-phase single-target call).
// newMem.Supersedes MUST already be set to the resolved target id list by
// the caller before this is invoked (mirrors OwnedOrAbsent's contract:
// callers thread resolved ids in, store methods do not resolve short ids).
//
// Ordering is load-bearing: locks for every (deduped, sorted) target are
// acquired FIRST (lockTargets); then getWritable/ActionWrite and the
// already-superseded check run for EVERY target, both completed before any
// write (D-05 — a set is never accepted because one member passed); then
// the new record is created; then ONE multi-ID back-stamp covers the whole
// target set, routed through the setPayloadKeys seam. This is only
// atomic-intent, not atomic — same non-atomicity SetVisibility's own
// two-step SetPayload+DeletePayload sequence accepts. If the back-stamp
// fails after the survivor's create succeeds, reconcileSupersedeFailure
// removes the survivor and clears superseded_by from every target still
// pointing at it, so no committed state can leave a dangling link
// (T-03.1-06).
//
// Compensation and reconciliation are BEST-EFFORT, not a guarantee: they
// talk to the same Qdrant that just failed, so they degrade in exactly the
// outage that triggers them — a clean no-op is the usual outcome, not
// something this method promises. What IS provable is scoped to the
// TERMINAL state: once this call has returned and reconcileSupersedeFailure
// has succeeded, no target carries a superseded_by pointing at the removed
// survivor. That scope matters because lockTargets' locks serialize
// WRITERS only — Store.Get and every recall query (List/Search/
// SearchDiscovery/ListScheduled) are lock-free and take no lock at all. A
// concurrent reader CAN therefore observe a predecessor soft-hidden inside
// the window between Qdrant's partial write and the reconciliation pass; a
// partially-applied merge being briefly observable mid-flight is NOT
// something this method closes, and no comment or test in this package may
// claim otherwise.
//
// TOCTOU note (identical in spirit to SetVisibility's, store.go:1688-1692):
// if a target is deleted between its getWritable ownership gate and the
// back-stamp SetPayload, Qdrant's point-ID-selector SetPayload returns a
// NotFound gRPC error that propagates unchanged — fail-closed, no re-fetch
// needed (D-02). At the pinned server (qdrant/qdrant:v1.18.2,
// lib/shard/src/update.rs) a multi-ID SetPayload chunks point ids (in
// batches of qdrantPayloadOpBatchSize) and mutates every point it finds
// BEFORE raising a missing-id error for one it doesn't, so an error from
// this call means possibly-partial, never nothing-written — the reason
// reconcileSupersedeFailure re-reads the FULL target set, across every
// chunk, rather than trusting the error to mean nothing landed.
//
// Concurrency note (CR-01): the already-superseded check through the
// back-stamp is classic check-then-act with no Qdrant-side compare-and-swap.
// Two concurrent Supersede calls sharing a target could otherwise both
// observe "not yet superseded" and both succeed, silently forking the
// correction chain. lockTargets' sorted per-target locks, held through the
// back-stamp, make the check-then-act atomic in-process per target (D-10
// additionally guarantees overlapping concurrent merges never deadlock each
// other). See TargetLocker's doc comment: this only guarantees
// single-instance atomicity — a future distributed lock (swapped in via
// WithTargetLocker) would be required for multi-replica atomicity.
//
// Concurrency note (03.1 cycle-4 review CR-01): the per-target lock above
// only ever covered `targets` — never newMem.ID, the survivor's own id. For
// a keyed call that id is deterministic (idempotencyPointID(owner, scope,
// key), computed independent of the target set), so two concurrent keyed
// calls sharing a key but naming DISJOINT target sets contended on no lock
// at all: both could observe "not found" at that id (the caller's own
// pre-lock checkIdempotentMergeReplay Get), both pass their own per-target
// preflight, and both Upsert the same survivor id — the second silently
// whole-payload-replacing the first's content and target set, orphaning the
// first's back-stamped target with a superseded_by link to a record whose
// Supersedes omits it. This is fixed by locking the UNION of targets and
// newMem.ID (when keyed) below, then re-deciding the same
// same-key-different-operation question under that lock before ever
// Upserting. newMem.ID is locked but deliberately never joins the STAMP set
// (`sorted`, built separately from `targets` alone) — folding it into the
// lock's dedupe/sort step would make the survivor back-stamp itself as one
// of its own targets, corruption strictly worse than the race being closed.
func (s *Store) Supersede(ctx context.Context, newMem Memory, vec []float32, targets []string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.Supersede",
		trace.WithAttributes(
			attribute.String("engram.owner", ownerOf(subj)),
			attribute.Int("engram.supersede.target_count", len(targets)),
		))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Supersede", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	// 0. Dedupe + sort + acquire a per-target lock for every target, PLUS the
	//    survivor's own id (newMem.ID) when this is a keyed call, held
	//    through the back-stamp (CR-01/D-10, and 03.1 cycle-4 CR-01 above).
	//    lockTargets dedupes+sorts the UNION it is given, so a newMem.ID that
	//    happens to coincide with a target id is never locked twice (which
	//    would self-deadlock the non-reentrant per-id mutex).
	lockIDs := targets
	if newMem.ID != "" {
		lockIDs = append(slices.Clone(targets), newMem.ID)
	}
	unlock, _, err := s.lockTargets(ctx, lockIDs)
	if err != nil {
		return err
	}
	defer unlock()

	// The STAMP set is deliberately computed independently of the LOCK set
	// above: only the caller's actual targets are ever getWritable-checked or
	// back-stamped. newMem.ID must never appear here (see the doc comment
	// above Supersede).
	sorted := slices.Clone(targets)
	sort.Strings(sorted)
	sorted = slices.Compact(sorted)
	// Surfaced for operators, not branched on: whether the deduped set
	// exceeds one Qdrant payload-op chunk, so a partial-application incident
	// can be correlated with cross-chunk vs. within-chunk (D-15).
	span.SetAttributes(attribute.Bool("engram.supersede.multi_chunk", len(sorted) > qdrantPayloadOpBatchSize))

	// 0.5. Race-free keyed-survivor-collision re-check (03.1 cycle-4 review
	// CR-01). checkIdempotentMergeReplay's own Get (tools.go) runs BEFORE any
	// lock is held, so it cannot itself close the race described above — but
	// newMem.ID is now locked (for keyed calls) by step 0, so re-deciding the
	// identical same-key-different-operation question here IS race-free.
	// A matching fingerprint is a true replay: some other in-flight or
	// already-committed call already wrote this exact (key, content, target
	// set) — return success without writing again, mirroring
	// checkIdempotentMergeReplay's own replay branch. A mismatched
	// fingerprint is exactly the race this fix closes: reject it the same
	// way a genuine idempotency-key reuse conflict is rejected, rather than
	// letting a second blind Upsert silently replace the winner.
	if newMem.ID != "" && newMem.IdempotencyFingerprint != "" {
		existing, gerr := s.Get(ctx, newMem.ID)
		switch {
		case gerr == nil && existing.IdempotencyFingerprint == newMem.IdempotencyFingerprint:
			return nil
		case gerr == nil:
			return fmt.Errorf("idempotency key reused with a different target set or content: %w", ErrIdempotencyConflict)
		case !errors.Is(gerr, ErrNotFound):
			return gerr
		}
	}

	// 1. Owner-only write gate on EVERY target (not the new record — that's
	//    a normal create, already write-gated by construction) — completed
	//    before any write (D-05).
	recs := make(map[string]Memory, len(sorted))
	for _, target := range sorted {
		rec, gerr := s.getWritable(ctx, target, subj, authz.ActionWrite)
		if gerr != nil {
			return gerr // ErrNotFound: not owner, or doesn't exist — fail-closed, no leak.
		}
		recs[target] = rec
	}
	// 2. Single-hop / cycle rejection (D-05/D-06) over EVERY target: reject
	//    if any target is already a non-head record. Collects ALL offenders
	//    (not just the first) so the typed error's IDs field is complete
	//    (PD-10) — unlike the ownership gate above, this class must name
	//    every offender at once.
	var alreadySuperseded []string
	for _, target := range sorted {
		if rec := recs[target]; rec.SupersededBy != nil && *rec.SupersededBy != "" {
			alreadySuperseded = append(alreadySuperseded, target)
		}
	}
	if len(alreadySuperseded) > 0 {
		return &MultiTargetError{Err: ErrAlreadySuperseded, IDs: alreadySuperseded}
	}
	// 3. Store the new record (normal Upsert — fresh id, no concurrent-writer
	//    risk since nothing else references it yet).
	if err = s.Upsert(ctx, newMem, vec); err != nil {
		return err
	}
	// 4. Back-stamp every target: one multi-ID SetPayload, vector-preserving
	//    — mirrors SetVisibility exactly, action swapped Share->Write, key
	//    swapped visibility->superseded_by, a single id swapped for the
	//    whole deduped sorted set. Routed through the setPayloadKeys seam
	//    (nil-guarded via a local fallback, never invoked off the receiver —
	//    New populates no function field, so a receiver-qualified call would
	//    panic in production; store.go:1953-1958 is the same idiom).
	setKeys := s.setPayloadKeys
	if setKeys == nil {
		setKeys = s.defaultSetPayloadKeys
	}
	if err = setKeys(ctx, sorted, map[string]any{"superseded_by": newMem.ID}); err != nil {
		// D-07/D-08/D-15 (cycle-1 review MEDIUM, plan 03.1-02): a failed
		// multi-ID back-stamp may be possibly-partial per the doc comment
		// above, and leaving this bare would commit a merge that
		// permanently soft-hides live records behind the four
		// IsEmpty("superseded_by") recall gates. reconcileSupersedeFailure
		// owns BOTH halves — removing the survivor and reconciling every
		// target — and is the single call site for compensation; no removal
		// happens here.
		result := s.reconcileSupersedeFailure(ctx, newMem.ID, sorted)
		if result.RemoveErr != nil || len(result.Dangling) > 0 {
			// Orphan ids cross to the operator log, never back into the
			// returned error (D-08) — the caller receives the original
			// single-fault envelope only, so this log line can never be
			// used to probe for record existence.
			slog.ErrorContext(ctx, "supersede compensation incomplete",
				"survivor_id", newMem.ID,
				"remove_err", errString(result.RemoveErr),
				"read_failures", result.ReadFailures,
				"clear_failures", result.ClearFailures,
				"dangling", result.Dangling,
			)
		}
		// Return the ORIGINAL back-stamp error, unchanged.
		return err
	}
	return nil
}

// errString renders err as a string for a structured log field, "" for nil —
// slog.ErrorContext's Any-typed fields would otherwise serialize a non-nil
// error inconsistently across handlers; this pins one shape for this call
// site.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// supersedeReconcileResult classifies what reconcileSupersedeFailure
// observed while compensating a failed back-stamp. RemoveErr is the
// survivor-removal outcome (nil on success); ReadFailures and
// ClearFailures record, respectively, which target reads and which target
// clears errored (both are also folded into Dangling, since either failure
// mode leaves the reconciler unable to certify that target clean); Dangling
// is every target the caller cannot be certain was reconciled — an operator
// worklist, never returned to the caller (D-08).
type supersedeReconcileResult struct {
	RemoveErr     error
	ReadFailures  []string
	ClearFailures []string
	Dangling      []string
}

// reconcileSupersedeFailure runs after Store.Supersede's multi-ID back-stamp
// has errored, still holding every per-target lock lockTargets acquired.
// It owns BOTH halves of compensation and populates every field of the
// returned result — the caller performs no removal of its own:
//
//  1. Remove the survivor via the deletePoint seam (nil-guarded to
//     defaultDeletePoint below), the internal UNGATED point-removal
//     primitive scoped to newMem.ID — a record minted inside this same
//     Supersede call, never linked to anything, never recallable by another
//     caller. D-09's "no delete_memory in the merge path" governs the
//     caller-facing verb, not this internal cleanup: removing a record that
//     was never linked destroys zero history. Routing through this
//     primitive rather than Store.Delete is deliberate (cycle-1 review
//     MEDIUM): Store.Delete re-runs getWritable with authz.ActionDelete, a
//     SECOND authorization decision during rollback that a policy
//     permitting write but denying delete could block, stranding the
//     orphan. RemoveErr carries whatever this call returns; a removal
//     failure does NOT abort reconciliation below — an orphaned survivor
//     and a dangling link are independent problems and the operator needs
//     both reported.
//  2. Re-read the FULL requested target set (every id in targets, not one
//     qdrantPayloadOpBatchSize chunk and not only the ids the failed call is
//     believed to have reached) via s.Get and classify each:
//     - read succeeded, SupersededBy nil/empty/points elsewhere: nothing to
//     do, not reported anywhere.
//     - read succeeded, SupersededBy == survivorID: clear the key via the
//     clearKeys local (nil-guarded to defaultDeletePayloadKeys), DELETING
//     it rather than writing "" (an empty value still satisfies a value
//     check, so writing one would leave the record hidden forever —
//     mirrors UpdatePayload's provenance-clear mechanism, store.go:1954-
//     1958). On clear error: append to BOTH ClearFailures and Dangling.
//     - read failed with ErrNotFound: RESOLVED, not dangling — the target
//     is gone, so nothing points anywhere. Not reported.
//     - read failed with any other error (a transport/protocol failure):
//     the state is UNKNOWN, and unknown must be reported as
//     possibly-dangling rather than assumed clean. Append to BOTH
//     ReadFailures and Dangling.
func (s *Store) reconcileSupersedeFailure(ctx context.Context, survivorID string, targets []string) supersedeReconcileResult {
	delPoint := s.deletePoint
	if delPoint == nil {
		delPoint = s.defaultDeletePoint
	}
	result := supersedeReconcileResult{RemoveErr: delPoint(ctx, survivorID)}

	clearKeys := s.deletePayloadKeys
	if clearKeys == nil {
		clearKeys = s.defaultDeletePayloadKeys
	}
	for _, target := range targets {
		fresh, gerr := s.Get(ctx, target)
		if gerr != nil {
			if errors.Is(gerr, ErrNotFound) {
				continue // resolved: the target is gone, nothing points anywhere.
			}
			result.ReadFailures = append(result.ReadFailures, target)
			result.Dangling = append(result.Dangling, target)
			continue
		}
		if fresh.SupersededBy == nil || *fresh.SupersededBy != survivorID {
			continue // nothing to do: never stamped, or points elsewhere.
		}
		if cerr := clearKeys(ctx, target, []string{"superseded_by"}); cerr != nil {
			result.ClearFailures = append(result.ClearFailures, target)
			result.Dangling = append(result.Dangling, target)
		}
	}
	return result
}

// IncrementAccess bumps a record's access_count by 1 and stamps
// last_accessed_at, without re-embedding (uses SetPayload, preserving the
// vector). It is a store-layer primitive fired only by handler-boundary
// callers that already gated ownership (D-01) — it deliberately does NOT
// re-run getWritable/GetReadable; the internal Get here is purely a
// read-modify-write of the counter value, not a re-authorization. The RMW is
// last-writer-wins: concurrent bumps on the same record may lose an
// increment, which is an accepted tradeoff for a soft curation signal (D-05,
// no mutex/singleflight/optimistic-retry).
func (s *Store) IncrementAccess(ctx context.Context, id string) (err error) {
	ctx, span := tracer.Start(ctx, "store.IncrementAccess")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "IncrementAccess", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	cur, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{
			"access_count":     cur.AccessCount + 1,
			"last_accessed_at": s.now().Format(time.RFC3339),
		}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// Delete removes the memory with the given id, only if owned by subj.
func (s *Store) Delete(ctx context.Context, id string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.Delete",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Delete", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if _, err := s.getWritable(ctx, id, subj, authz.ActionDelete); err != nil {
		return err
	}
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}

// DeleteAll removes the subject's OWN records in scope (never another owner's,
// and never another owner's shared records). A nil/unknown Subject is rejected
// without deleting anything — fail closed. The own-bucket decision is
// PDP-derived (s.decideBucket, ActionDelete/BucketOwn), matching how the read
// filter builders route their bucket decisions, so a future per-category or
// per-tenant delete restriction applies here too, not just to id-addressed
// Delete.
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) (err error) {
	ctx, span := tracer.Start(ctx, "store.DeleteAll", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "DeleteAll", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	owner, kind, ok := principalParams(subj)
	if !ok {
		return fmt.Errorf("%w: nil subject", ErrNotFound)
	}
	if !s.decideBucket(ctx, owner, kind, authz.ActionDelete, authz.BucketOwn).Allow {
		return nil
	}
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", owner),
	}}
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}

// PruneExpired deletes every record whose not_after is strictly before the given
// instant — an operator/admin sweep run from the CLI across the WHOLE collection
// (no subject authz; it is not on behalf of a caller). Records without a
// not_after key are never matched. Returns a BEST-EFFORT deleted count: it is the
// Count taken just before the filter-Delete (Qdrant's delete response carries no
// count), so concurrent writes between the two RPCs can make the reported number
// drift from the exact number removed. The delete filter itself is exact; only
// the reported tally is approximate. Treat it as a sweep summary, not an audit.
func (s *Store) PruneExpired(ctx context.Context, before time.Time) (deleted uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.PruneExpired",
		trace.WithAttributes(attribute.Int64("engram.before", before.Unix())))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PruneExpired", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(deleted)))
		}
	}()

	// The Count half IS CountExpired (spine.go) — not a second, independently
	// built Count — so the preview number cmd/engram's prune-expired --output
	// reports without --apply and the number reported here are the same
	// function called twice, never two constructions that merely happen to
	// agree on a fixture.
	n, err := s.CountExpired(ctx, before)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(expiredFilter(before)),
	}); err != nil {
		return 0, err
	}
	deleted = n
	return deleted, nil
}

// ownerlessFilter matches pre-isolation records — those written before the owner
// key existed. NewIsEmpty matches a missing, null, or empty-array "owner" payload
// but NOT an explicit empty string, so auth-disabled records (which carry an
// explicit owner=="") are intentionally excluded.
func ownerlessFilter() *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty("owner")}}
}

// CountOwnerless returns the number of pre-isolation (owner-less) records. These
// are invisible to every owner-scoped read until migrate-set-owner stamps them;
// the server bootstrap uses this to warn the operator. See ownerlessFilter.
func (s *Store) CountOwnerless(ctx context.Context) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.CountOwnerless")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CountOwnerless", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	return s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: ownerlessFilter(), Exact: qdrant.PtrOf(true),
	})
}

// MintShortID returns a short id not currently present on any record, retrying
// on the (astronomically unlikely at 50 bits) global collision. When seen is
// non-nil, ids it returns are recorded there and candidates already in it are
// skipped — a defensive same-run guard for batch callers (backfill), so that
// intra-run uniqueness never depends on the backend's read-after-write
// visibility between a mint and its subsequent write. The global Count is
// authoritative.
func (s *Store) MintShortID(ctx context.Context, seen map[string]struct{}) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.MintShortID")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MintShortID", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	gen := s.mintCandidate
	if gen == nil {
		gen = shortid.New
	}
	for attempts, spins := 0, 0; attempts < maxMintAttempts; spins++ {
		// Absolute termination guarantee: seen-map skips don't consume the
		// maxMintAttempts budget (D-05), so a degenerate generator returning
		// only already-seen candidates could otherwise loop forever. The
		// separate spin cap makes termination unconditional without changing
		// the real-collision-check budget.
		if spins >= maxMintSpins {
			err = fmt.Errorf("%w: generator degenerate after %d spins (only seen candidates)", ErrShortIDExhausted, spins)
			return "", err
		}
		cand, genErr := gen()
		if genErr != nil {
			err = genErr
			return "", err
		}
		if seen != nil {
			if _, dup := seen[cand]; dup {
				continue // D-05: seen-map hits don't consume the real-collision-check budget
			}
		}
		attempts++
		n, countErr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", cand)}},
			Exact:          qdrant.PtrOf(true),
		})
		if countErr != nil {
			err = countErr
			return "", err
		}
		if n == 0 {
			if seen != nil {
				seen[cand] = struct{}{}
			}
			id = cand
			return id, nil
		}
	}
	err = fmt.Errorf("%w: after %d attempts", ErrShortIDExhausted, maxMintAttempts)
	return "", err
}

// CountAnonymousBucket returns the number of records in the auth-disabled
// anonymous bucket (an explicit owner==""). Distinct from CountOwnerless, which
// matches pre-isolation records with NO owner key (NewIsEmpty). The server
// bootstrap warns when this is non-empty: those records are readable by any
// anonymous caller, so an operator who once ran auth-disabled should know they
// exist before enabling a network surface.
func (s *Store) CountAnonymousBucket(ctx context.Context) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.CountAnonymousBucket")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CountAnonymousBucket", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	return s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}},
		Exact:          qdrant.PtrOf(true),
	})
}

// MigrateSetOwner backfills owner onto every pre-isolation record (one that lacks
// an owner key). Idempotent: records that already carry an owner — including the
// auth-disabled owner=="" bucket — are not matched (see ownerlessFilter).
//
// Returns the number of owner-less records counted immediately before the stamp.
// The count and the SetPayload are two separate operations, not one atomic
// transaction, and Qdrant's SetPayload reports no affected-point count to
// reconcile against — so under concurrent writes the returned figure can drift
// from the rows actually stamped (a record added after the count is still
// stamped by the filter but not counted; one deleted in the window is counted
// but not stamped). This is acceptable for the intended use: a one-time, offline
// admin backfill on a single-user deployment with no concurrent writers. Run it
// that way and the count is exact.
func (s *Store) MigrateSetOwner(ctx context.Context, owner string) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.MigrateSetOwner")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MigrateSetOwner", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if owner == "" {
		return 0, fmt.Errorf("owner must be non-empty")
	}
	missing := ownerlessFilter()
	// Snapshot count taken just before the stamp; see the non-atomicity caveat in
	// the doc comment. Exact:true so the offline single-user count is precise.
	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: missing, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, nil
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": owner}),
		PointsSelector: qdrant.NewPointsSelectorFilter(missing),
	})
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

// OwnerRemapSource selects which records RemapOwner re-stamps. It is a sealed
// sum, like Subject: exactly RemapMissing() (owner-less, pre-isolation records
// via IsEmpty), RemapAnon() (the explicit anonymous bucket, owner==""), or
// RemapFrom(sub) (a specific current owner value, a sub or an email). Missing
// and Anon are distinct because IsEmpty("owner") and Match("owner","") target
// different record sets. Sealing (the unexported marker method) makes "which
// source" a compile-time-exhaustive choice instead of a runtime field-count
// check. Like Subject's enforcement gates, dispatch happens via a single
// exhaustive type switch in RemapOwner rather than per-variant interface
// methods, so an unhandled/nil source is nil-safe by construction — a type
// switch's default arm matches a nil interface value without panicking.
// OwnerRemapSource has no Subject-style accessor method (Subject's Owner())
// because nothing outside RemapOwner's own switch needs a value out of the
// source; ValidateOwnerRemap gets what it needs (remapFrom.from) the same way.
type OwnerRemapSource interface {
	isOwnerRemapSource()
}

type remapMissing struct{}

func (remapMissing) isOwnerRemapSource() {}

type remapAnon struct{}

func (remapAnon) isOwnerRemapSource() {}

type remapFrom struct{ from string }

func (remapFrom) isOwnerRemapSource() {}

// RemapMissing selects owner-less (pre-isolation) records.
func RemapMissing() OwnerRemapSource { return remapMissing{} }

// RemapAnon selects the explicit anonymous bucket (owner=="").
func RemapAnon() OwnerRemapSource { return remapAnon{} }

// RemapFrom selects records currently stamped with the given owner value (a
// sub or an email). It panics on an empty value, consistent with
// Authenticated's empty-owner panic: an empty From must never silently
// collapse into a different source.
func RemapFrom(from string) OwnerRemapSource {
	if from == "" {
		panic("store.RemapFrom: from value must be non-empty")
	}
	return remapFrom{from: from}
}

// ValidateOwnerRemap reports whether src and to are well-formed for a remap:
// src must be non-nil, to must be non-empty, and (for a RemapFrom source)
// remapping a value onto itself is a no-op the caller almost certainly didn't
// intend. It is a free function, not an OwnerRemapSource method, because `to`
// is not a property of the source — shared by RemapOwner and the CLI's
// buildRemapSource so this check lives in one place.
func ValidateOwnerRemap(src OwnerRemapSource, to string) error {
	if src == nil {
		return errors.New("owner remap source is required")
	}
	if to == "" {
		return errors.New("to must be non-empty")
	}
	if f, ok := src.(remapFrom); ok && f.from == to {
		return fmt.Errorf("from and to are identical (%q)", to)
	}
	return nil
}

// RemapOwner re-stamps owner=<selected source> -> owner=to across the WHOLE
// collection (operator sweep; no subject authz, like PruneExpired). dryRun
// returns the matched count without writing. Validation runs before any Qdrant
// call. Non-transactional: the reported count is a best-effort snapshot taken
// just before the filtered SetPayload (which is itself exact). Idempotent.
func (s *Store) RemapOwner(ctx context.Context, src OwnerRemapSource, to string, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.RemapOwner")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "RemapOwner", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if err = ValidateOwnerRemap(src, to); err != nil {
		return 0, err
	}
	var filter *qdrant.Filter
	switch sj := src.(type) {
	case remapMissing:
		filter = ownerlessFilter()
	case remapAnon:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}}
	case remapFrom:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", sj.from)}}
	default:
		// ValidateOwnerRemap already rejected nil above, so reaching here
		// means a non-nil src of a sealed variant this switch doesn't
		// recognize — a future OwnerRemapSource variant added without a
		// corresponding case, not a caller-input problem.
		return 0, fmt.Errorf("owner remap: unrecognized source type %T", sj)
	}

	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 || dryRun {
		return cnt, nil
	}
	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": to}),
		PointsSelector: qdrant.NewPointsSelectorFilter(filter),
	}); err != nil {
		return 0, err
	}
	return cnt, nil
}

// EmbedFunc re-embeds a memory's content into a vector. Reindex takes it as a
// callback so the store stays embedder-agnostic (every other write path already
// receives a precomputed vector); the caller supplies the currently-configured
// embedder.
type EmbedFunc func(ctx context.Context, content string) ([]float32, error)

// ReindexOptions parameterizes Reindex. Target and Dim are required; Target must
// differ from the source collection. Batch is the scroll page size (0 → a sane
// default). DryRun scans and counts without creating the target or writing.
type ReindexOptions struct {
	Target string
	// Source overrides the collection Reindex reads from. Empty (the default)
	// means the store's configured collection (s.collection, i.e.
	// ENGRAM_QDRANT_COLLECTION) — so operators get the env default for free and
	// only set Source to reindex an arbitrary collection without mutating env.
	Source string
	Dim    uint64
	Batch  uint32
	DryRun bool
	// Progress, when non-nil, is called once after each scanned batch with the
	// running ReindexResult totals, so a long reindex can surface incremental
	// feedback. It is a callback (not store-side I/O) to keep the store
	// embedder- and output-agnostic — the caller owns rendering, matching how
	// EmbedFunc keeps embedding out of the store.
	Progress func(ReindexResult)
	// Resume makes an interrupted reindex cheap to restart: before embedding a
	// source point, the target is checked for that id, and a point already
	// present with IDENTICAL content AND a matching embedder_identity (or no
	// Identity configured) is skipped (counted Unchanged) instead of
	// re-embedded. A target whose content matches but whose stamped identity
	// is absent or differs from Identity is NOT skipped — it is re-embedded so
	// the stamp below rewrites it, keeping the resume path from silently
	// leaving a record untraceable to its embedder config (Phase 13 SC3). No
	// separate content hash is persisted; the target payload stays verbatim
	// aside from the identity key. Ignored on a dry run (nothing is written).
	// When false, every point is re-embedded (the prior idempotent-overwrite
	// behavior).
	Resume bool
	// Identity is the embedder-config-identity (config.EmbedderIdentity) to
	// stamp onto every reindexed record's payload under embedderIdentityKey,
	// via a guarded additive raw-map write — see the Reindex doc comment.
	// Empty means no stamp is written (e.g. an unstamped source or a caller
	// that has not computed one).
	Identity string
}

// Validate checks the options that depend only on the options themselves,
// including the target-differs-from-source guard for an EXPLICIT Source (both
// fields are then in opts). The default-source case (Source=="" → s.collection)
// and the non-nil-embed precondition are checked by Reindex, which alone has the
// store's collection and the embed callback.
func (o ReindexOptions) Validate() error {
	if o.Target == "" {
		return errors.New("reindex: target collection is required")
	}
	if o.Dim == 0 {
		return errors.New("reindex: target dimension must be > 0")
	}
	if o.Source != "" && o.Target == o.Source {
		return errors.New("reindex: target collection must differ from source")
	}
	return nil
}

// ReindexResult reports what Reindex did: points scanned from the source,
// points re-embedded and upserted into the target (0 on a dry run), points
// skipped because they carried no content to embed, and (resume only) points
// left unchanged because the target already held them with identical content
// and tags. WouldUpsert is populated only under DryRun: the count of points
// the same skip predicate would NOT have skipped, had this been a real run
// (D-14). On a successful real run, Scanned == Upserted + Skipped + Unchanged.
// On a dry run, Scanned == WouldUpsert + Skipped + Unchanged, and Upserted
// stays 0 — a dry run never writes.
type ReindexResult struct {
	Scanned     uint64
	Upserted    uint64
	Skipped     uint64
	Unchanged   uint64
	WouldUpsert uint64
}

// reindexBatch is the default scroll page size when ReindexOptions.Batch is 0.
const reindexBatch = 256

// Reindex re-embeds every point in the source collection (opts.Source, or
// s.collection when unset) into a
// new Target collection, enabling a migration to an embedder with a different
// output dimension (Qdrant vector size is immutable, so a new collection is the
// only path). It scrolls the source for (id, payload), re-embeds the payload's
// content with embed, and upserts (same id, new vector, payload preserved
// VERBATIM) into the target. The source is never mutated, so the operator can
// verify the target before cutting ENGRAM_QDRANT_COLLECTION over.
//
// The payload is carried as the raw Qdrant map rather than round-tripped through
// Memory: that preserves keys the Memory model does not know (forward/backward
// schema drift) and, critically, keeps a pre-isolation record's owner key ABSENT.
// An absent owner key and an explicit owner=="" are distinct states: an absent
// key marks a needs-backfill record invisible to every read until
// `engram migrate-set-owner` sets it, whereas owner=="" is the anonymous bucket.
// A Memory round-trip would synthesize an explicit owner=="" and silently
// relocate the record into that bucket.
//
// The one intentional exception to "payload preserved VERBATIM": when
// opts.Identity is non-empty, the single embedderIdentityKey is added/
// overwritten on the payload map immediately before the upsert (Phase 13
// SC3). This is a guarded additive raw-map write, not a Memory/payload()
// round-trip, so it does not touch the owner key or any other field.
//
// Reindex's per-point write (the raw client.Upsert call below, Payload:
// p.Payload) is the intentional whole-write exception to "every full write
// routes through payload()" — it never calls payload() at all. This has a
// real consequence for schema_version specifically (correcting an earlier
// assumption that Reindex "inherits" the D-05 monotonic stamp like every
// other full write): because the payload map is copied verbatim off the
// source point rather than rebuilt from a decoded Memory, Reindex preserves
// schema_version byte-for-byte, present or absent, exactly as the source
// carried it. Reindexing does NOT advance a record's version — converging
// a version toward current is the future migration sweep's job (Phase 3),
// not Reindex's.
//
// Fail-closed: an embed error aborts immediately and is returned wrapped; no
// zero/garbage vector is ever written. A point carrying no content is skipped
// (counted in ReindexResult.Skipped) rather than embedded as an empty string.
// The operation is bounded and cancellable via ctx.
//
// No rollback: Reindex is NOT transactional. A scroll, embed, or upsert error
// part-way through leaves the target partially populated (ReindexResult reports
// how many landed). Because upsert is keyed by point id, re-running Reindex with
// the same target is idempotent and safe — it overwrites and completes the set.
// opts.Resume makes that re-run cheap: points already present in the target with
// identical content are skipped (counted Unchanged) instead of re-embedded.
func (s *Store) Reindex(ctx context.Context, opts ReindexOptions, embed EmbedFunc) (res ReindexResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Reindex",
		trace.WithAttributes(attribute.String("engram.target", opts.Target)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Reindex", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int64("engram.scanned", int64(res.Scanned)),
				attribute.Int64("engram.upserted", int64(res.Upserted)),
				attribute.Int64("engram.skipped", int64(res.Skipped)),
				attribute.Int64("engram.unchanged", int64(res.Unchanged)),
			)
		}
	}()

	if err = opts.Validate(); err != nil {
		return res, err
	}
	// The read source is the override when set, else the store's configured
	// collection (engram-orve). All source reads and the target-differs guard
	// below key off this effective source, not s.collection directly.
	source := opts.Source
	if source == "" {
		source = s.collection
	}
	// Record the effective source (may differ from the store's collection under
	// --source) so a reindex trace names both endpoints.
	span.SetAttributes(attribute.String("engram.source", source))
	if opts.Target == source {
		return res, fmt.Errorf("reindex: target collection %q must differ from source", opts.Target)
	}
	if embed == nil {
		return res, fmt.Errorf("reindex: embed function is required")
	}
	batch := opts.Batch
	if batch == 0 {
		batch = reindexBatch
	}

	// Require the source to already exist. Without this, a typo'd source name
	// (or a not-yet-created collection) would scroll zero points and report a
	// misleading success — especially since the caller's StoreFromEnv may have
	// just created an empty source at the wrong dimension.
	srcExists, err := s.client.CollectionExists(ctx, source)
	if err != nil {
		return res, fmt.Errorf("reindex: check source %q: %w", source, err)
	}
	if !srcExists {
		return res, fmt.Errorf("reindex: source collection %q does not exist", source)
	}

	if !opts.DryRun {
		if err = s.ensureCollection(ctx, opts.Target, opts.Dim); err != nil {
			return res, fmt.Errorf("reindex: ensure target %q: %w", opts.Target, err)
		}
	}

	var offset *qdrant.PointId
	for {
		var pts []*qdrant.RetrievedPoint
		var next *qdrant.PointId
		pts, next, err = s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: source,
			Limit:          qdrant.PtrOf(batch),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(false),
		})
		if err != nil {
			return res, fmt.Errorf("reindex: scroll source: %w", err)
		}
		// Resume: fetch this batch's ids from the target once so a point already
		// embedded with identical content and tags (AND a matching stamped
		// identity) can be skipped (engram-irhg; identity-awareness per Phase 13
		// SC3 review; tag-awareness per #345, D-07..D-12). One Get per page keeps
		// the lookup O(pages), not O(points). A single per-point skip predicate
		// backs BOTH the dry-run and the real arm below — duplicating it into a
		// dry-run-only copy is the failure mode this plan designs out (two
		// predicates drift, which is exactly the defect class being closed).
		var targetInfo map[string]reindexTarget
		if opts.Resume {
			// Under a real run the target is guaranteed to exist (ensureCollection
			// already ran above). Under a dry run it may not — CollectionExists
			// guards the lookup, mirroring the source-existence check's shape, so a
			// missing target under --dry-run --resume yields an empty map (every
			// record counted as would-re-embed) rather than an error. Never call
			// ensureCollection under DryRun; that would cross the write boundary a
			// dry run promises not to cross.
			lookup := true
			if opts.DryRun {
				lookup, err = s.client.CollectionExists(ctx, opts.Target)
				if err != nil {
					return res, fmt.Errorf("reindex: check target %q: %w", opts.Target, err)
				}
			}
			if lookup {
				targetInfo, err = s.reindexTargetContents(ctx, opts.Target, pts)
				if err != nil {
					return res, fmt.Errorf("reindex: resume lookup in %q: %w", opts.Target, err)
				}
			}
		}
		for _, p := range pts {
			res.Scanned++
			m := fromPayload(p.Id.GetUuid(), p.Payload)
			content := m.Content
			if content == "" {
				// Nothing to embed — skip rather than write a meaningless vector
				// for an empty string. Surfaced via ReindexResult.Skipped.
				res.Skipped++
				continue
			}
			if ti, ok := targetInfo[p.Id.GetUuid()]; ok && ti.content == content &&
				tagsEqual(ti.tags, m.Tags) &&
				(opts.Identity == "" || ti.identity == opts.Identity) {
				// ti is a SNAPSHOT of what the target held at its last write; content
				// and m.Tags are the source's CURRENT values. The two are
				// independently mutable — a tags-only edit on the source leaves ti
				// stale while content still matches, so this is a genuine three-part
				// equality check, not one part implying another (D-11). EmbedText
				// folds tags into the embedded text, so a real tags change without a
				// content change still requires a re-embed to avoid serving a stale
				// vector (the #345 defect this conjunct closes).
				//
				// tagsEqual is deliberately order-independent (D-09): a record whose
				// tags were only reordered embeds to different text under EmbedText
				// yet is intentionally treated as unchanged here. Normalizing tag
				// order at write time would be the root fix and is out of scope.
				//
				// Only when no Identity is being enforced, or the target already
				// carries the matching stamp: a content+tags match with an
				// absent/stale identity falls through and gets re-embedded+restamped
				// below, so resume never leaves a record untraceable to its embedder
				// config.
				res.Unchanged++
				continue
			}
			if opts.DryRun {
				// Would be re-embedded on a real run — count it, but never call
				// embed or write. Upserted stays 0 on every dry run (EDGE 5).
				res.WouldUpsert++
				continue
			}
			var vec []float32
			// Embed content + tags (EmbedText) so a re-embed folds curated tags
			// into the vector exactly as the store path does.
			vec, err = embed(ctx, EmbedText(m.Content, m.Tags))
			if err != nil {
				return res, fmt.Errorf("reindex: embed point %s: %w", p.Id.GetUuid(), err)
			}
			if opts.Identity != "" {
				// The one intentional additive exception to the verbatim-payload
				// invariant: a single guarded raw-map key write, never a
				// Memory/payload() round-trip (see the Reindex doc comment).
				p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)
			}
			if _, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
				CollectionName: opts.Target,
				Wait:           qdrant.PtrOf(true),
				Points: []*qdrant.PointStruct{{
					Id:      p.Id,
					Vectors: qdrant.NewVectors(vec...),
					Payload: p.Payload,
				}},
			}); err != nil {
				return res, fmt.Errorf("reindex: upsert point %s into %q: %w", p.Id.GetUuid(), opts.Target, err)
			}
			res.Upserted++
		}
		// Surface running totals after each scanned page (engram-xddn).
		if opts.Progress != nil {
			opts.Progress(res)
		}
		if next == nil {
			break
		}
		offset = next
	}
	return res, nil
}

// tagsFromPayload decodes the "tags" key out of a raw Qdrant payload map. It is
// the single tag decoder in this package: fromPayload calls it for the source
// side and reindexTargetContents calls it for the target side. Both reads MUST
// go through this one function — an encoding asymmetry between the two sides
// would surface as permanent false re-embeds, since reindex --resume would
// never converge (D-08). An absent key and a present-but-empty list both yield
// the nil zero value; do not pre-initialize the return slice, or every
// untagged record would re-embed forever under nil-vs-empty comparison.
func tagsFromPayload(p map[string]*qdrant.Value) []string {
	var tags []string
	if v, ok := p["tags"]; ok {
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				tags = append(tags, item.GetStringValue())
			}
		}
	}
	return tags
}

// supersedesFromPayload decodes the "supersedes" payload key tolerantly: a
// list-shaped value (every record this phase writes, D-01/D-03) or a legacy
// bare-string value (every record written before this phase, D-04 — no
// backfill command, a legacy scalar loses nothing functionally under this
// tolerant read). It exists to close a silent-empty-string failure mode: the
// protobuf oneof accessor Value.GetStringValue() returns "" — never an error
// — when the stored Kind is actually a list, so an unconditional string read
// of this key would silently decode to empty for every record this phase
// writes, with every existing test on the pre-phase scalar shape staying
// green. Kind is checked explicitly (mirroring tagsFromPayload's
// GetListValue-first shape) to close that seam; this is the ONLY decode call
// site for this key.
func supersedesFromPayload(p map[string]*qdrant.Value) []string {
	v, ok := p["supersedes"]
	if !ok {
		return nil
	}
	if lv := v.GetListValue(); lv != nil {
		ids := make([]string, 0, len(lv.GetValues()))
		for _, item := range lv.GetValues() {
			ids = append(ids, item.GetStringValue())
		}
		return ids
	}
	if _, isString := v.GetKind().(*qdrant.Value_StringValue); isString {
		return []string{v.GetStringValue()}
	}
	return nil
}

// tagsEqual reports whether a and b hold the same tags, order-independent but
// multiplicity-preserving: reordering doesn't matter (D-09's deliberate
// residual — EmbedText joins tags in slice order, so a purely reordered
// record embeds to different text yet compares equal here), but a tag
// appearing twice in one slice and once in the other is NOT equal. A set-
// based comparison would silently collapse duplicates, which content
// equality's own byte-exact semantics never would. nil and an empty slice
// compare equal by construction (D-10) — both have length 0.
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := slices.Clone(a), slices.Clone(b)
	sort.Strings(as)
	sort.Strings(bs)
	return slices.Equal(as, bs)
}

// reindexTarget is the per-id resume lookup shape: the target point's content
// and tags (for the content/tags-equality skip check) and its stamped
// embedder_identity (a missing key reads as the zero-value "", per
// fromPayload's convention).
type reindexTarget struct {
	content  string
	tags     []string
	identity string
}

// reindexTargetContents fetches the content, tags, and stamped
// embedder_identity of the given source points' ids from the target
// collection, returning id→reindexTarget only for ids that already exist
// there. It backs Reindex's resume skip: a fresh or partially-populated
// target simply yields fewer (or no) entries, so a first run skips nothing.
// Tags are read via tagsFromPayload — the same decoder the source side uses
// via fromPayload — so the resume tag-equality conjunct can never diverge
// between the two sides (D-08). The identity is read so the resume skip
// predicate can be identity-aware (Phase 13 SC3): a content match with a
// missing/mismatched identity must NOT be treated as unchanged.
func (s *Store) reindexTargetContents(ctx context.Context, target string, pts []*qdrant.RetrievedPoint) (map[string]reindexTarget, error) {
	if len(pts) == 0 {
		return nil, nil
	}
	ids := make([]*qdrant.PointId, 0, len(pts))
	for _, p := range pts {
		ids = append(ids, p.Id)
	}
	got, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: target,
		Ids:            ids,
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]reindexTarget, len(got))
	for _, p := range got {
		out[p.Id.GetUuid()] = reindexTarget{
			content:  p.Payload["content"].GetStringValue(),
			tags:     tagsFromPayload(p.Payload),
			identity: p.Payload[embedderIdentityKey].GetStringValue(),
		}
	}
	return out, nil
}
