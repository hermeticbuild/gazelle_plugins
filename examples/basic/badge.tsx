// A small TSX component using react-intl. Demonstrates that .tsx files
// flow through the same gazelle pipeline as .ts files.
import React from 'react';
import { FormattedNumber } from 'react-intl';

export interface BadgeProps {
  count: number;
}

export function Badge({ count }: BadgeProps): React.ReactElement {
  return (
    <span className="badge">
      <FormattedNumber value={count} />
    </span>
  );
}
