// shared-read: a shared record is readable by any authenticated (non-empty
// owner) caller. READ ONLY (DEC-kyz) — write/delete/share/schedule on a
// shared record must fall through to deny. The empty-owner guard here keeps
// the anonymous caller from gaining shared-read; own-records already grants
// the anonymous caller its own owner=="" bucket independently.
permit (
  principal,
  action == Action::"read",
  resource
)
when {
  resource.visibility == "shared" &&
  principal.owner != ""
};
