// CLI app: imports from a deeply nested package, a Node builtin, a
// synthetic Bazel-generated package wired via `# gazelle:resolve_regexp`,
// and a virtual module routed via `# gazelle:resolve` at the repo root.
import { argv } from 'node:process';
import { readFileSync } from 'node:fs';
import { greeting } from '@myrepo_generated/synthetic';
import { banner } from 'mystery:banner';
import { statsForUsers } from '#packages/utils/math/deep/stats.js';
import { isValidEmail, type User } from '#packages/core/index.js';

function main(): void {
  const file = argv[2] ?? '/dev/stdin';
  const raw = JSON.parse(readFileSync(file, 'utf8')) as User[];
  const valid = raw.filter((u) => isValidEmail(u.email));
  const stats = statsForUsers(valid);
  console.log(banner);
  console.log(greeting('user'));
  console.log(JSON.stringify(stats, null, 2));
}

main();
