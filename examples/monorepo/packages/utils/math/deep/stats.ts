// Deeply nested package: packages/utils/math/deep — three levels under
// packages/. Demonstrates that gazelle handles arbitrary depth and that
// imports across deeply-nested packages resolve.
import { type User } from '#packages/core/index.js';
import { sum, mean } from 'lodash';

export interface UserStats {
  count: number;
  total: number;
  average: number;
}

export function statsForUsers(users: User[]): UserStats {
  const counts = users.map((_, i) => i);
  return {
    count: users.length,
    total: sum(counts),
    average: mean(counts) ?? 0,
  };
}
