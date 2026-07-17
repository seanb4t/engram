// tenant-isolate: has-guarded, vacuous this phase (D-06). Neither
// Principal.tenant nor Memory.tenant is ever set by this phase's entity
// converter, so `has tenant` is false for every request this phase
// constructs — the policy can never match, by construction, until a later
// milestone's converter starts populating `tenant`.
permit (
  principal,
  action,
  resource
)
when {
  principal has tenant &&
  resource has tenant &&
  principal.tenant == resource.tenant
};
