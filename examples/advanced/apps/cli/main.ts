// CLI app: imports from a deeply nested package, a Node builtin, and a
// synthetic Bazel-generated package wired via `# gazelle:ts_generated_package`.
import { argv } from 'node:process';
import { readFileSync } from 'node:fs';
import { greeting } from '@myrepo_generated/synthetic';
import { statsForUsers } from '#packages/utils/math/deep/stats.js';
import { userSchema } from '#packages/core/index.js';

function main(): void {
  const file = argv[2] ?? '/dev/stdin';
  const raw = JSON.parse(readFileSync(file, 'utf8'));
  const users = raw.map((u: unknown) => userSchema.parse(u));
  const stats = statsForUsers(users);
  console.log(greeting('user'));
  console.log(JSON.stringify(stats, null, 2));
}

main();
