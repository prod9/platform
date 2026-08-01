export const ssr = false;
export const prerender = true;

// 'always' so a prerendered route emits <route>/index.html rather than <route>.html, which
// keeps a hard refresh mid-route resolving to a real file.
export const trailingSlash = "always";
