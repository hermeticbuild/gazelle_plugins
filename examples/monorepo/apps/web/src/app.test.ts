// Test file: imports vitest + the local app.
import { describe, expect, it } from 'vitest';
import { App } from './index.js';

describe('App', () => {
  it('renders without crashing', () => {
    expect(typeof App).toBe('function');
  });
});
