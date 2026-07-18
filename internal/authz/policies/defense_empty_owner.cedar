// defense-empty-owner: defense-in-depth backstop for the milestone's #1 risk
// (an empty/unresolved owner claim silently landing in the anonymous bucket
// then reaching another owner's records). Scoped forbid — NOT a blanket
// `forbid ... unless principal.owner != ""` — because a blanket forbid would
// also deny the legitimate anonymous owner=="" bucket and break D-11. This
// fires only when the RESOURCE has a non-empty owner (a real, non-anonymous
// resource) and the principal's owner is empty; it never fires on the
// owner=="" anonymous bucket (resource.owner == "" there too).
forbid (
  principal,
  action,
  resource
)
when {
  resource has owner &&
  resource.owner != ""
}
unless {
  principal.owner != ""
};
