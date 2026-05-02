// Core package — base types, no internal deps. Exists so utils + apps can
// share the same User shape via #packages/core.
export interface User {
  id: string;
  email: string;
  registered: Date;
}

export const isAdmin = (u: User): boolean => u.email.endsWith('@admin.local');
