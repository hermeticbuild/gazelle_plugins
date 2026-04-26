// Smoke test that consumes the codegen output. Two purposes:
//  1. Forces tsc to typecheck @myrepo_generated/queries against the schema —
//     a broken codegen step (e.g. schema/query mismatch) fails the build here.
//  2. Gives `bazel test //...` at least one test target so the CI step
//     doesn't error with "No test targets were found".
//
// Note: graphql-codegen with the `typescript` + `typescript-operations` plugins
// emits only TS types (no runtime constants). The test imports them as
// type-only and asserts on a constructed value to keep the body executable.
import assert from 'node:assert';
import type {
  GetUserQuery,
  ListUsersQuery,
} from '@myrepo_generated/queries';

const sampleUser: NonNullable<GetUserQuery['user']> = {
  __typename: 'User',
  id: '1',
  email: 'a@b',
  name: 'A',
};
assert.equal(sampleUser.id, '1');

const list: ListUsersQuery['users'] = [sampleUser];
assert.equal(list.length, 1);

console.log('ok');
