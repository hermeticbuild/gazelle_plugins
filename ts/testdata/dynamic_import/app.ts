export async function load() {
  const m = await import("lodash");
  return m;
}
