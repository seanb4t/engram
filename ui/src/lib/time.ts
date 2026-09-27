import type { Memory } from '$lib/gen/engram_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';

// `now` is injectable for deterministic tests; defaults to current time at call.
export function relativeTime(d: Date, now: Date = new Date()): string {
  const s = Math.max(0, Math.floor((now.getTime() - d.getTime()) / 1000));
  if (s < 60) return 'now';
  const m = Math.floor(s / 60); if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60); if (h < 24) return `${h}h`;
  const days = Math.floor(h / 24); if (days < 30) return `${days}d`;
  const mo = Math.floor(days / 30); if (mo < 12) return `${mo}mo`;
  return `${Math.floor(mo / 12)}y`;
}

export function fullTimestamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
}

// D-13/CUR-04: a windowed record's `{not_before} → {not_after}` range, `—`
// for whichever bound is absent. The one formatter /scheduled's trailing
// cell and its tooltip both read — never re-derived at the call site.
type Windowed = Pick<Memory, 'notBefore' | 'notAfter'>;

export function windowRange(m: Windowed): string {
  const before = m.notBefore ? fullTimestamp(timestampDate(m.notBefore)) : '—';
  const after = m.notAfter ? fullTimestamp(timestampDate(m.notAfter)) : '—';
  return `${before} → ${after}`;
}

// The relative phrase for a windowed record's row (D-13): 'reveals in {unit}'
// while still scheduled, 'expired {unit} ago' once past, '' while active (no
// window, or a window whose bounds are both in the past/inapplicable).
// Precedence mirrors memorystate.ts exactly: expired (not_after <= now) is
// evaluated first and, when true, suppresses the reveal phrase even for an
// inverted not_before/not_after pair.
export function windowPhrase(m: Windowed, now: Date = new Date()): string {
  const notAfterDate = m.notAfter ? timestampDate(m.notAfter) : undefined;
  const notBeforeDate = m.notBefore ? timestampDate(m.notBefore) : undefined;

  const expired = !!notAfterDate && notAfterDate.getTime() <= now.getTime();
  if (expired) {
    const rel = relativeTime(notAfterDate!, now);
    return rel === 'now' ? 'expired just now' : `expired ${rel} ago`;
  }

  const scheduled = !!notBeforeDate && notBeforeDate.getTime() > now.getTime();
  if (scheduled) {
    // relativeTime(d, now) computes max(0, now - d) — passing `now` as `d`
    // and `notBeforeDate` as `now` yields the FUTURE delta (notBefore - now)
    // through the same bucket thresholds, without a second implementation.
    const rel = relativeTime(now, notBeforeDate!);
    return rel === 'now' ? 'reveals in under a minute' : `reveals in ${rel}`;
  }

  return '';
}
