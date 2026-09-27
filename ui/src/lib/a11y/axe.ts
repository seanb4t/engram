// Test-only WCAG 2.2 AA audit helper. Imported ONLY from *.browser.test.ts —
// never from a component the SPA bundles (DSYS-03, T-04-08).
import axe from 'axe-core';

/** WCAG 2.0/2.1/2.2 A and AA rule tags — the D-17 gate's rule set. */
export const AA_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] as const;

export type AxeViolation = axe.Result;

/**
 * Runs axe-core over a rendered subtree with the WCAG 2.2 AA rule tags and
 * returns the violations. Never throws, including on an empty subtree.
 */
export async function auditAA(root: Element): Promise<AxeViolation[]> {
  const results = await axe.run(root, {
    runOnly: { type: 'tag', values: [...AA_TAGS] },
    resultTypes: ['violations']
  });
  return results.violations;
}

/** One line per offending node, for actionable test failure output. */
export function formatViolations(violations: AxeViolation[]): string {
  return violations
    .flatMap((v) =>
      v.nodes.map((n) => {
        const firstLine = (n.failureSummary ?? '').split('\n')[0];
        const target = Array.isArray(n.target) ? n.target.join(' ') : String(n.target);
        return `${v.id} (${v.impact ?? 'unknown'}) ${target} — ${firstLine}`;
      })
    )
    .join('\n');
}
