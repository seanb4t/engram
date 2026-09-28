import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { parseSupersedeRejection } from './supersede-rejection';

describe('parseSupersedeRejection', () => {
  it('maps a NotFound rejection to a targets/not-found rejection with parsed inputs', () => {
    const err = new ConnectError('not found: a, b', Code.NotFound);
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'targets', issue: 'not-found', inputs: ['a', 'b'] });
  });

  it('maps the rule-rejection sentinel to a targets/rule rejection', () => {
    const err = new ConnectError(
      'rules are always shared: r1 — delete the rule instead of superseding it',
      Code.FailedPrecondition
    );
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'targets', issue: 'rule', inputs: ['r1'] });
  });

  it('maps the already-superseded sentinel to a targets/already-superseded rejection', () => {
    const err = new ConnectError('target is already superseded: x', Code.FailedPrecondition);
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'targets', issue: 'already-superseded', inputs: ['x'] });
  });

  it('maps a field=/hint= envelope to a field rejection', () => {
    const err = new ConnectError('field=content hint=required: content is required', Code.InvalidArgument);
    expect(parseSupersedeRejection(err)).toEqual({
      kind: 'field',
      fields: ['content'],
      hint: 'required',
      detail: 'content is required'
    });
  });

  it('maps Unauthenticated to a reauth rejection', () => {
    const err = new ConnectError('session expired', Code.Unauthenticated);
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'reauth' });
  });

  it('maps PermissionDenied to a reauth rejection', () => {
    const err = new ConnectError('forbidden', Code.PermissionDenied);
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'reauth' });
  });

  it('maps anything else to an opaque rejection', () => {
    const err = new ConnectError('boom', Code.Internal);
    const result = parseSupersedeRejection(err);
    expect(result.kind).toBe('opaque');
    if (result.kind === 'opaque') {
      expect(result.codeName).toBe('Internal');
      expect(result.detail).toBe('boom');
    }
  });

  it('maps a non-ConnectError to an opaque rejection without throwing', () => {
    const result = parseSupersedeRejection(new Error('network down'));
    expect(result.kind).toBe('opaque');
  });

  it('parses multiple comma-separated inputs, trimming whitespace', () => {
    const err = new ConnectError('not found: a,  b ,c', Code.NotFound);
    expect(parseSupersedeRejection(err)).toEqual({ kind: 'targets', issue: 'not-found', inputs: ['a', 'b', 'c'] });
  });
});
