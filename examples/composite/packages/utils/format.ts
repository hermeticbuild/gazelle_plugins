// Mid-level package — depends on core via the package.json `imports` map.
import { sortBy } from 'lodash-es';
import { type User } from '#packages/core/types.js';

export function newest(users: User[]): User | undefined {
  return sortBy(users, (u) => -u.registered.getTime())[0];
}
