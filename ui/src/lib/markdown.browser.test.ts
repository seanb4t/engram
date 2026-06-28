import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown', () => {
  it('renders benign markdown structure', () => {
    const html = renderMarkdown('# h\n\n- a\n- b\n\n`code` and **bold**');
    expect(html).toContain('<ul>');
    expect(html).toContain('<li>a</li>');
    expect(html).toContain('<code>code</code>');
    expect(html).toContain('<strong>bold</strong>');
  });

  it('keeps fenced code blocks', () => {
    const html = renderMarkdown('```\nx := 1\n```');
    expect(html).toContain('<pre>');
    expect(html).toContain('x := 1');
  });

  it('strips script tags', () => {
    expect(renderMarkdown('<script>alert(1)</script>')).not.toContain('<script');
  });

  it('strips event-handler attributes and dangerous img', () => {
    const html = renderMarkdown('<img src=x onerror="alert(1)">');
    expect(html).not.toContain('onerror');
  });

  it('drops javascript: links but keeps https links with safe rel/target', () => {
    const js = renderMarkdown('[x](javascript:alert(1))');
    expect(js).not.toContain('javascript:');
    const ok = renderMarkdown('[x](https://example.com)');
    expect(ok).toContain('href="https://example.com"');
    expect(ok).toContain('rel="noopener noreferrer"');
    expect(ok).toContain('target="_blank"');
  });

  it('returns empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('');
  });
});
