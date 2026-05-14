process.env.NODE_ENV;
chrome.runtime.sendMessage("hello");
window.gapi.load("picker", () => {});
import.meta.env.VITE_FOO;
appEnv.VITE_FOO;

export const foo: Bar = {};

export type PickedFile = google.picker.DocumentObject;

export interface Env {
  BUCKET: R2Bucket;
}

export default {
  async fetch(_request: Request, env: Env, _ctx: ExecutionContext) {
    return new Response((await env.BUCKET.get("key"))?.body);
  },
} satisfies ExportedHandler<Env>;
