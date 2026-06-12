import { describe, it, expect } from 'vitest';
import { relativeTime, fullTimestamp } from './time';

const NOW = new Date('2026-06-12T15:00:00Z');

describe('relativeTime', () => {
  it('renders hours', () => { expect(relativeTime(new Date('2026-06-12T10:00:00Z'), NOW)).toBe('5h'); });
  it('renders days', () => { expect(relativeTime(new Date('2026-06-10T15:00:00Z'), NOW)).toBe('2d'); });
  it('renders just now under a minute', () => { expect(relativeTime(new Date('2026-06-12T14:59:40Z'), NOW)).toBe('now'); });
});
describe('fullTimestamp', () => {
  it('renders an ISO-ish minute precision', () => { expect(fullTimestamp(new Date('2026-06-12T14:03:00Z'))).toMatch(/2026-06-12 14:03/); });
});
