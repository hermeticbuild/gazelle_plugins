// Top-level app: pulls in packages from across the monorepo + npm packages.
import React from 'react';
import { useQuery } from '@tanstack/react-query';
import type { User } from '#packages/core/index.js';
import { userLabel } from '#packages/utils/string/format.js';
import { statsForUsers } from '#packages/utils/math/deep/stats.js';

export function App() {
  const { data } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: () => fetch('/api/users').then((r) => r.json()),
  });

  const users = data ?? [];
  const stats = statsForUsers(users);

  return React.createElement(
    'div',
    null,
    `${stats.count} users; first: ${users[0] ? userLabel(users[0]) : 'none'}`,
  );
}
