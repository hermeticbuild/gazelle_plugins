// CLI app: imports from a deeply nested package and a Node builtin.
import { argv } from 'node:process';
import { readFileSync } from 'node:fs';
import { statsForUsers } from '#packages/utils/math/deep/src/stats.js';
import { userSchema } from '#packages/core/src/index.js';

function main(): void {
  const file = argv[2] ?? '/dev/stdin';
  const raw = JSON.parse(readFileSync(file, 'utf8'));
  const users = raw.map((u: unknown) => userSchema.parse(u));
  const stats = statsForUsers(users);
  console.log(JSON.stringify(stats, null, 2));
}

main();
