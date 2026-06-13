import { describe, it, expect } from 'vitest';
import { stripCategoryPrefix } from './summary';

describe('stripCategoryPrefix', () => {
  it('strips a leading ALLCAPS category token matching the category', () => {
    expect(stripCategoryPrefix('GOTCHA (agentgateway path) must match', 'gotcha')).toBe('agentgateway path) must match');
  });
  it('leaves content alone when there is no redundant prefix', () => {
    expect(stripCategoryPrefix('Uptime Kuma monitors as IaC', 'convention')).toBe('Uptime Kuma monitors as IaC');
  });
});
