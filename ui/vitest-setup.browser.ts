// Browser-tier setup: jest-dom matchers attach to vitest's expect, usable on
// locators via expect.element(...). The node tier's localStorage stub is NOT
// needed here — a real browser provides Storage.
import '@testing-library/jest-dom/vitest';
