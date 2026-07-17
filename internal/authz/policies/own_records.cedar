// own-records: the principal may perform any action on a resource it owns.
// resource.owner == principal.owner naturally grants the anonymous bucket to
// the anonymous caller too, since "" == "" — this is intentional (D-11): the
// owner=="" bucket must stay reachable for auth-disabled deployments.
permit (
  principal,
  action,
  resource
)
when {
  resource.owner == principal.owner
};
