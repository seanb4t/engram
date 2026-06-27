// SPDX-License-Identifier: Apache-2.0
import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Memory content is caller-authored and `shared` records are cross-actor
// readable, so the only safe path is: marked (no HTML passthrough trust) ->
// DOMPurify tight allowlist -> {@html}. With `{ async: false }` marked's typed
// overload returns `string` (no cast needed); flipping it to async would make
// the type a Promise and fail the compile here, which is the intended guard.
marked.use({ gfm: true, breaks: true });

const ALLOWED_TAGS = [
  'p', 'br', 'strong', 'em', 'del', 'code', 'pre', 'blockquote',
  'ul', 'ol', 'li', 'a', 'h1', 'h2', 'h3', 'h4', 'hr',
  'table', 'thead', 'tbody', 'tr', 'th', 'td'
];
// Deliberate delta from spec §3.4 (['href','title']): `target`/`rel` are
// allow-listed so the afterSanitizeAttributes hook's link-hardening survives
// across all DOMPurify v3 patch levels. No security regression — marked never
// emits them from input, and they are inert attributes.
const ALLOWED_ATTR = ['href', 'title', 'target', 'rel'];
const SAFE_SCHEME = /^(https?|mailto):/i;

let hookInstalled = false;
function installLinkHook(): void {
  if (hookInstalled) return;
  hookInstalled = true;
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (!(node instanceof Element) || !node.hasAttribute('href')) return;
    const href = node.getAttribute('href') ?? '';
    if (!SAFE_SCHEME.test(href)) {
      node.removeAttribute('href');
      return;
    }
    if (node.tagName === 'A') {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noopener noreferrer');
    }
  });
}

// renderMarkdown turns caller-authored markdown into sanitized HTML safe for
// {@html}. Returns "" for empty input.
export function renderMarkdown(src: string): string {
  if (!src) return '';
  installLinkHook();
  const html = marked.parse(src, { async: false });
  return DOMPurify.sanitize(html, { ALLOWED_TAGS, ALLOWED_ATTR });
}
