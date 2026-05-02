// Shared helper used by both lib code (app.ts) and the vite config
// (vite.config.ts). Per the gazelle_ts plugin's "helpers stay in lib" rule
// for files matched by ts_bundler_config_pattern, this lands in the lib
// target's srcs and the vite_config target gets `:lib` in its deps.
export function withBanner(text: string): string {
  return `[gazelle_ts] ${text}`;
}
