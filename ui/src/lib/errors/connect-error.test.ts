import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { parseConnectError, fixRowsFor } from './connect-error';

describe('parseConnectError', () => {
  it('classifies a field=/hint= envelope as rejected, splitting fields on comma', () => {
    const err = new ConnectError(
      'field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true',
      Code.FailedPrecondition
    );
    expect(parseConnectError(err)).toEqual({
      kind: 'rejected',
      fields: ['scope', 'cross_spine'],
      hint: 'conditional_required',
      detail: 'scope is required unless cross_spine is true',
      code: Code.FailedPrecondition
    });
  });

  it('classifies an ambiguous short_id BEFORE the envelope check, despite sharing FailedPrecondition', () => {
    const err = new ConnectError('ambiguous short id: k3m9p2qr7a', Code.FailedPrecondition);
    expect(parseConnectError(err)).toEqual({ kind: 'ambiguous-short-id', shortId: 'k3m9p2qr7a' });
  });

  it('classifies Code.NotFound as not-found', () => {
    const err = new ConnectError('no memory with that id', Code.NotFound);
    expect(parseConnectError(err)).toEqual({ kind: 'not-found' });
  });

  it('keeps the detail verbatim, including embedded colons', () => {
    const err = new ConnectError(
      'field=response hint=response_too_large: the result would exceed 64MiB: really',
      Code.ResourceExhausted
    );
    const parsed = parseConnectError(err);
    expect(parsed).toEqual({
      kind: 'rejected',
      fields: ['response'],
      hint: 'response_too_large',
      detail: 'the result would exceed 64MiB: really',
      code: Code.ResourceExhausted
    });
  });

  it('classifies a ConnectError with an unrecognized message shape as opaque, naming its code', () => {
    const err = new ConnectError('backend unavailable', Code.Unavailable);
    expect(parseConnectError(err)).toEqual({
      kind: 'opaque',
      code: Code.Unavailable,
      codeName: 'Unavailable',
      detail: 'backend unavailable'
    });
  });

  it('classifies a plain Error as opaque', () => {
    const parsed = parseConnectError(new Error('boom'));
    expect(parsed.kind).toBe('opaque');
    expect((parsed as { detail: string }).detail).toContain('boom');
  });
});

describe('fixRowsFor', () => {
  it('offers enable-cross-spine and pick-scope for a conditional_required cross_spine rejection', () => {
    const parsed = parseConnectError(
      new ConnectError(
        'field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true',
        Code.FailedPrecondition
      )
    );
    expect(fixRowsFor(parsed).map((r) => r.id)).toEqual(['enable-cross-spine', 'pick-scope', 'retry']);
  });

  it('offers lower-k for an out_of_range k rejection', () => {
    const parsed = parseConnectError(
      new ConnectError('field=k hint=out_of_range: k must be at most 1000', Code.InvalidArgument)
    );
    expect(fixRowsFor(parsed).map((r) => r.id)).toEqual(['lower-k', 'retry']);
  });

  it('offers lower-k and without-full for response_too_large', () => {
    const parsed = parseConnectError(
      new ConnectError('field=response hint=response_too_large: too big', Code.ResourceExhausted)
    );
    expect(fixRowsFor(parsed).map((r) => r.id)).toEqual(['lower-k', 'without-full', 'retry']);
  });

  it('offers clear-created for a format rejection on created_after', () => {
    const parsed = parseConnectError(
      new ConnectError('field=created_after hint=format: not RFC3339', Code.InvalidArgument)
    );
    expect(fixRowsFor(parsed).map((r) => r.id)).toEqual(['clear-created', 'retry']);
  });

  it('offers only retry for an opaque error', () => {
    const parsed = parseConnectError(new ConnectError('backend unavailable', Code.Unavailable));
    expect(fixRowsFor(parsed).map((r) => r.id)).toEqual(['retry']);
  });
});
