// Core package — no internal deps, just npm + builtins.
import { z } from 'zod';
import * as path from 'node:path';

export const userSchema = z.object({
  id: z.string(),
  email: z.string().email(),
});

export type User = z.infer<typeof userSchema>;

export function formatPath(p: string): string {
  return path.normalize(p);
}
