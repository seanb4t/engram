// The one URL/request codec for /scheduled (D-11, D-13) — one parse
// function, one encode function, so the two cannot drift (mirrors params.ts's
// contract for /search).

export type ScheduledState = 'scheduled' | 'expired' | 'all';

const STATES: readonly ScheduledState[] = ['scheduled', 'expired', 'all'];

export interface ScheduledParams {
  state: ScheduledState;
  sel: string;
}

export function defaultScheduledParams(): ScheduledParams {
  return { state: 'scheduled', sel: '' };
}

// An unknown/missing `state` value parses to 'scheduled' — the tab default,
// never an error.
export function parseScheduledParams(sp: URLSearchParams): ScheduledParams {
  const raw = sp.get('state');
  const state = (STATES as readonly string[]).includes(raw ?? '') ? (raw as ScheduledState) : 'scheduled';
  return {
    state,
    sel: sp.get('sel') ?? ''
  };
}

// Defaults are never written back: encode(default()) round-trips to ''.
export function encodeScheduledParams(p: ScheduledParams): string {
  const sp = new URLSearchParams();
  if (p.state !== 'scheduled') sp.set('state', p.state);
  if (p.sel) sp.set('sel', p.sel);
  return sp.toString();
}
