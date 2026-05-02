// Vitest discovers .test.ts files and runs each describe/it block. The
// gazelle plugin map_kinds js_test → vitest_test so the wrapper is the
// rules_js auto-generated bin macro for vitest itself (see //tools:ts.bzl).
import { describe, expect, it } from 'vitest';

describe('smoke', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2);
  });
});
