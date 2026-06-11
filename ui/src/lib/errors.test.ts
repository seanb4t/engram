import { describe, it, expect } from 'vitest';
import { Code, ConnectError } from '@connectrpc/connect';
import { describeError } from './errors';

describe('describeError', () => {
  it('returns the ConnectError message', () => {
    expect(describeError(new ConnectError('boom', Code.Internal))).toContain('boom');
  });
  it('returns a plain Error message', () => {
    expect(describeError(new Error('nope'))).toBe('nope');
  });
  it('stringifies unknown values', () => {
    expect(describeError('weird')).toBe('weird');
  });
});
