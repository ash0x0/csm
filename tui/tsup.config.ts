import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/index.tsx'],
  format: ['esm'],
  outDir: 'dist',
  bundle: true,
  minify: false,
  sourcemap: false,
  target: 'node20',
  platform: 'node',
  noExternal: [/.*/],
  banner: {
    js: '#!/usr/bin/env node\nimport{createRequire}from"module";const require=createRequire(import.meta.url);',
  },
  esbuildOptions(options) {
    options.external = [...(options.external ?? []), 'react-devtools-core'];
  },
});
