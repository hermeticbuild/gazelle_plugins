// Smoke test running directly via Node 22's --experimental-transform-types
// (configured in .bazelrc). Demonstrates that the gazelle plugin emits
// js_test targets and that Bazel can build+run them. Real consumers wire a
// proper TS-aware test runner (vitest_test, jest_test, …) via map_kind.
import assert from 'node:assert';

assert.equal(1 + 1, 2);
console.log('ok');
