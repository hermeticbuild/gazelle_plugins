// Vitest config — peeled out by
// `# gazelle:ts_bundler_config_pattern vitest.config.* vitest_config`.
// `vitest/config` is build-time tooling, not a runtime dep of the library.
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    include: ['**/*.test.ts'],
  },
});
