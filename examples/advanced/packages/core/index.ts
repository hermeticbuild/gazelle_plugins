// Core package — no internal deps, just builtins.
import * as path from 'node:path';

export interface User {
  id: string;
  email: string;
}

export function formatPath(p: string): string {
  return path.normalize(p);
}

export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}
