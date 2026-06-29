// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

// Subject is the verified caller identity used for authorization. It is a sealed
// sum: exactly Anonymous (auth disabled — the owner=="" bucket) or Authenticated
// (a verified, non-empty resolved owner-claim value, default email — not
// necessarily the OIDC sub). The concrete variants are unexported, so the union
// cannot be extended or constructed outside this package; callers use the
// Anonymous()/Authenticated() constructors. The zero value is nil (not
// Anonymous): a discarded extraction error yields nil, which fails closed at the
// store default arm rather than silently granting the anonymous bucket.
type Subject interface {
	isSubject()
	// Owner is the persistence/stamping accessor: the owner string this subject
	// writes onto Memory.Owner ("" for anonymous, sub for authenticated). It is
	// NOT an enforcement accessor — read filters and write gates use the
	// exhaustive type switch (with its default-deny arm), never Owner(). Calling
	// Owner() on a nil Subject panics (loud), never a silent anonymous grant.
	Owner() string
}

type anonymous struct{}

func (anonymous) isSubject()    {}
func (anonymous) Owner() string { return "" }

type authenticated struct{ sub string }

func (authenticated) isSubject()      {}
func (a authenticated) Owner() string { return a.sub }

// Anonymous is the caller when auth is disabled (the owner=="" bucket).
func Anonymous() Subject { return anonymous{} }

// Authenticated wraps the caller's resolved owner-claim value (default email),
// the authorization key written onto Memory.Owner. It panics on an empty value:
// an authenticated subject must never collapse into the owner=="" anonymous
// bucket by an empty-string slip. SubjectFromTokenInfo already guards this at
// the extraction boundary; the panic makes the invariant loud for any other
// in-package caller or test, consistent with Owner()-on-nil panicking.
func Authenticated(sub string) Subject {
	if sub == "" {
		panic("store.Authenticated: owner value must be non-empty")
	}
	return authenticated{sub: sub}
}
