import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, ArchiveOutcome, type ArchiveResult, type Memory } from '$lib/gen/engram_pb';
import {
  applyArchiveResultsOptimistic,
  changedIds,
  buildSupersedeRequest,
  applySupersedeOptimistic,
  type SupersedeDraft
} from './curation';

function memory(overrides: { id?: string; archivedAt?: ReturnType<typeof timestampFromDate> } = {}): Memory {
  return create(MemorySchema, { id: 'm1', content: 'c', scope: 's', visibility: 'private', tags: [], ...overrides });
}

function result(overrides: Partial<ArchiveResult> = {}): ArchiveResult {
  return { requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED, ...overrides } as ArchiveResult;
}

describe('applyArchiveResultsOptimistic', () => {
  it('ARCHIVED stamps archivedAt on the matching record in a cached searchMemories page, a cached offset listMemories page and getMemory, keeping every record in place', () => {
    const qc = new QueryClient();
    const searchKey = ['searchMemories', 'q', ''];
    const listKey = ['listMemories', 's', [], '', 50, 0];
    qc.setQueryData(searchKey, { memories: [memory({ id: 'm1' }), memory({ id: 'm2' })] });
    qc.setQueryData(listKey, { memories: [memory({ id: 'm1' }), memory({ id: 'm2' })], total: 2n });
    qc.setQueryData(['getMemory', 'm1'], { memory: memory({ id: 'm1' }) });

    applyArchiveResultsOptimistic(qc, [result({ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED })], new Date());

    const search = qc.getQueryData(searchKey) as { memories: Memory[] };
    expect(search.memories).toHaveLength(2);
    expect(search.memories[0].archivedAt).toBeDefined();
    expect(search.memories[1].archivedAt).toBeUndefined();

    const list = qc.getQueryData(listKey) as { memories: Memory[]; total: bigint };
    expect(list.memories).toHaveLength(2);
    expect(list.memories[0].archivedAt).toBeDefined();
    expect(list.total).toBe(2n);

    const got = qc.getQueryData(['getMemory', 'm1']) as { memory: Memory };
    expect(got.memory.archivedAt).toBeDefined();
  });

  it('RESTORED clears archivedAt', () => {
    const qc = new QueryClient();
    qc.setQueryData(['getMemory', 'm1'], { memory: memory({ id: 'm1', archivedAt: timestampFromDate(new Date()) }) });

    applyArchiveResultsOptimistic(qc, [result({ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.RESTORED })], new Date());

    const got = qc.getQueryData(['getMemory', 'm1']) as { memory: Memory };
    expect(got.memory.archivedAt).toBeUndefined();
  });

  it('ALREADY_ARCHIVED / NOT_ARCHIVED / NOT_FOUND change nothing', () => {
    const qc = new QueryClient();
    const original = memory({ id: 'm1' });
    qc.setQueryData(['getMemory', 'm1'], { memory: original });

    applyArchiveResultsOptimistic(
      qc,
      [
        result({ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ALREADY_ARCHIVED }),
        result({ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.NOT_ARCHIVED }),
        result({ requested: 'm9', id: '', outcome: ArchiveOutcome.NOT_FOUND })
      ],
      new Date()
    );

    const got = qc.getQueryData(['getMemory', 'm1']) as { memory: Memory };
    expect(got.memory).toEqual(original);
  });
});

describe('changedIds', () => {
  it('returns the ARCHIVED+RESTORED ids in result order', () => {
    const results: ArchiveResult[] = [
      result({ requested: 'a', id: 'a', outcome: ArchiveOutcome.ARCHIVED }),
      result({ requested: 'b', id: 'b', outcome: ArchiveOutcome.ALREADY_ARCHIVED }),
      result({ requested: 'c', id: 'c', outcome: ArchiveOutcome.RESTORED }),
      result({ requested: 'd', id: '', outcome: ArchiveOutcome.NOT_FOUND }),
      result({ requested: 'e', id: 'e', outcome: ArchiveOutcome.NOT_ARCHIVED })
    ];
    expect(changedIds(results)).toEqual(['a', 'c']);
  });
});

function draft(overrides: Partial<SupersedeDraft> = {}): SupersedeDraft {
  return {
    targets: ['m1', 'm2'],
    fields: { summary: 'sum', content: 'body', category: 'convention', scope: 'repo:test', tags: ['a', 'b'] },
    idempotencyKey: 'idem-key-1',
    ...overrides
  };
}

describe('buildSupersedeRequest', () => {
  it('sets source to console and preserves target order', () => {
    const req = buildSupersedeRequest(draft(), { validateOnly: true });
    expect(req.source).toBe('console');
    expect(req.supersedes).toEqual(['m1', 'm2']);
    expect(req.content).toBe('body');
    expect(req.scope).toBe('repo:test');
    expect(req.category).toBe('convention');
    expect(req.tags).toEqual(['a', 'b']);
    expect(req.summary).toBe('sum');
  });

  it('omits the idempotency key on a validate_only preview', () => {
    const req = buildSupersedeRequest(draft(), { validateOnly: true });
    expect(req.validateOnly).toBe(true);
    expect(req.idempotencyKey).toBe('');
  });

  it('carries the idempotency key on a real commit', () => {
    const req = buildSupersedeRequest(draft(), { validateOnly: false });
    expect(req.validateOnly).toBe(false);
    expect(req.idempotencyKey).toBe('idem-key-1');
  });
});

describe('applySupersedeOptimistic', () => {
  it('sets supersededBy on the matching cached search row and keeps the list length', () => {
    const qc = new QueryClient();
    const searchKey = ['searchMemories', 'q', ''];
    qc.setQueryData(searchKey, { memories: [memory({ id: 'm1' }), memory({ id: 'm2' })] });

    applySupersedeOptimistic(qc, ['m1'], 'n1');

    const search = qc.getQueryData(searchKey) as { memories: Memory[] };
    expect(search.memories).toHaveLength(2);
    expect(search.memories[0].supersededBy).toBe('n1');
    expect(search.memories[1].supersededBy).toBeUndefined();
  });
});
