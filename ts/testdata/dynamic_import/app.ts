export async function load() {
  const m = await import("lodash");
  return m;
}

export async function loadSingular() {
  return import("singular-sdk");
}
