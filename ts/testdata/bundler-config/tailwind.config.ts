// Tailwind config — peeled out by
// `# gazelle:ts_bundler_config_pattern tailwind.config.ts tailwind_config`.
// Demonstrates the directive working on a single named file rather than a
// glob.
import type { Config } from 'tailwindcss';

export default {
  content: ['./*.{ts,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [],
} satisfies Config;
