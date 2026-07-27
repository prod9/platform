import { defineConfig } from 'astro/config';

// https://astro.build/config
// No outDir: the testbed takes Astro's own default, so the smoke build exercises what a
// real Astro project actually hands platform. Bending it to match platform's default
// would test the two against nothing.
export default defineConfig({});
