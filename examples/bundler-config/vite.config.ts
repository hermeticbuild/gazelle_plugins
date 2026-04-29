// Bundler config — peeled out of the lib by the gazelle directive
// `# gazelle:ts_bundler_config_pattern vite.config.* vite_config`. None of
// these imports (vite, @vitejs/plugin-react) appear on the lib target's
// deps; they live on the sibling :vite_config target instead.
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { withBanner } from './viteHelpers.js';

export default defineConfig({
  plugins: [react()],
  define: {
    __BANNER__: JSON.stringify(withBanner('vite-build')),
  },
});
