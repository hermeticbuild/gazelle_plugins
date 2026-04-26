// Mid-level package — uses #packages/core via the package.json subpath import.
import { formatPath, type User } from '#packages/core/src/index.js';
import debounce from 'lodash/debounce';

export function userLabel(u: User): string {
  return `${u.id} <${u.email}>`;
}

export function formatPaths(paths: string[]): string[] {
  return paths.map(formatPath);
}

export const debouncedFormat = debounce(userLabel, 100);
