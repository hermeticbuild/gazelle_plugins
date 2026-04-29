// Library code: imports a runtime npm dep AND a local helper that's also
// pulled in by vite.config.ts. The helper landing in lib srcs (not in the
// vite_config target) is what the "helpers stay in lib" rule from the RFC
// produces — and what makes vite_config end up with `:lib` in its deps.
import { groupBy } from 'lodash-es';
import { withBanner } from './viteHelpers.js';

export interface Event {
  id: string;
  type: string;
}

export function summarize(events: Event[]): string {
  const grouped = groupBy(events, 'type');
  return withBanner(`${Object.keys(grouped).length} types`);
}
