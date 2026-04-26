// Top-level app: imports the codegen output AND a sibling internal package.
// `@myrepo_generated/queries` is built by Bazel from schema/queries.graphql via
// graphql-codegen; the gazelle plugin routes the import through the
// `# gazelle:ts_generated_package @myrepo_generated/*=...` directive in
// //BUILD.bazel.
import React from 'react';
import { FormattedDate } from 'react-intl';
import { type GetUserQuery, type ListUsersQuery } from '@myrepo_generated/queries';
import { formatUser } from '#packages/core/format.js';

export interface UserCardProps {
  data: NonNullable<GetUserQuery['user']>;
  registered: Date;
}

export function UserCard({ data, registered }: UserCardProps): React.ReactElement {
  return (
    <div className="user-card">
      <h3>{formatUser(data)}</h3>
      <p>
        joined <FormattedDate value={registered} dateStyle="medium" />
      </p>
    </div>
  );
}

export type AllUsersData = ListUsersQuery['users'];
