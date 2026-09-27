// STUB for RED evidence (Task 2, tdd="true") -- replaced with the real
// implementation once the failing tests are confirmed RED for the right
// reason (assertion mismatches, not import/module errors).
export type SupersedeRejection =
  | { kind: 'targets'; issue: 'not-found' | 'rule' | 'already-superseded'; inputs: string[] }
  | { kind: 'field'; fields: string[]; hint: string; detail: string }
  | { kind: 'reauth' }
  | { kind: 'opaque'; codeName: string; detail: string };

export function parseSupersedeRejection(_err: unknown): SupersedeRejection {
  return { kind: 'opaque', codeName: 'unknown', detail: '' };
}
