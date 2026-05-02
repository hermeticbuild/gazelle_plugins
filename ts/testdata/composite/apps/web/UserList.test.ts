// Smoke test running directly via Node 22's --experimental-transform-types.
import assert from 'node:assert';

assert.equal('hello'.length, 5);
console.log('ok');
