// A simple TS package: npm deps (lodash-es, react-intl) + a Node.js builtin.
import { basename } from 'node:path';
import { groupBy } from 'lodash-es';
import { createIntl, createIntlCache } from 'react-intl';

export interface Event {
  id: string;
  type: string;
  at: Date;
}

const cache = createIntlCache();
const intl = createIntl({ locale: 'en', messages: {} }, cache);

export function describe(events: Event[], file: string): string {
  const grouped = groupBy(events, 'type');
  const latest = events.reduce<Date | null>(
    (acc, e) => (acc && acc > e.at ? acc : e.at),
    null,
  );
  const stamp = latest
    ? intl.formatDate(latest, { dateStyle: 'short', timeStyle: 'short' })
    : 'no events';
  return `${basename(file)}: ${Object.keys(grouped).length} types (last: ${stamp})`;
}
