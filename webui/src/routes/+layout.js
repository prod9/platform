export const ssr = false;
export const prerender = true;

// 'always' so prerender emits install/index.html (not install.html); the Go FileServer
// then 301s /install -> /install/, surviving a hard refresh mid-install.
export const trailingSlash = "always";
