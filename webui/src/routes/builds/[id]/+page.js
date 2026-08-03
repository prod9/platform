// A build id cannot be enumerated at build time, so this route rides the SPA fallback —
// srv serves it at the status the record deserves (docs/spec/platform-server.md).
export const prerender = false;
