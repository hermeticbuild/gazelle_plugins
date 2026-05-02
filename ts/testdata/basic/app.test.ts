// Smoke test running directly via Node 22's --experimental-transform-types
// (configured in .bazelrc).
import assert from 'node:assert';

assert.equal(2 + 2, 4);
console.log('ok');
