// Smoke test running directly via Node 22's --experimental-transform-types.
// We don't run the file under vitest here — vitest.config.ts exists only to
// demonstrate the directive routing it out of the library. The test rule
// itself is plain js_test.
import assert from 'node:assert';
import { summarize } from './app.js';

const events = [
  { id: '1', type: 'click' },
  { id: '2', type: 'click' },
  { id: '3', type: 'hover' },
];
assert.equal(summarize(events), '[gazelle_ts] 2 types');
console.log('ok');
