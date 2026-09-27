import { page } from 'vitest/browser';
import { describe, it, expect, afterEach } from 'vitest';
import { auditAA, formatViolations } from './axe';

function mount(html: string): HTMLElement {
  const el = document.createElement('div');
  el.innerHTML = html;
  document.body.appendChild(el);
  return el;
}

describe('auditAA', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('reports color-contrast on a seeded low-contrast fixture (proves the gate can go red)', async () => {
    const el = mount('<p style="color:#9a9a9a;background:#ffffff">low contrast</p>');
    await page.screenshot({ element: el });
    const violations = await auditAA(el);
    expect(violations.map((v) => v.id)).toContain('color-contrast');
    expect(formatViolations(violations)).toContain('color-contrast');
  });

  it('reports no violations on a clean fixture', async () => {
    const el = mount('<p style="color:#1f2328;background:#ffffff">readable</p>');
    await page.screenshot({ element: el });
    const violations = await auditAA(el);
    expect(violations).toHaveLength(0);
  });

  it('never throws on an empty subtree', async () => {
    const el = mount('');
    await expect(auditAA(el)).resolves.toEqual([]);
  });
});
