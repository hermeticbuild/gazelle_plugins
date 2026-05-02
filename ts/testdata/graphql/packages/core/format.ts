// Internal package re-using the User type from the codegen output. Both
// this module and apps/web import @myrepo_generated/queries — same canonical
// types across the whole graph because the codegen step is part of the build.
import { type User } from '@myrepo_generated/queries';

export function formatUser(u: Pick<User, 'name' | 'email'>): string {
  return `${u.name} <${u.email}>`;
}
