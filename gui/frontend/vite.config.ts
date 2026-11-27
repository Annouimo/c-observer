import { defineConfig } from 'vite';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';

// Record package roots of modules actually included in the frontend bundles.
// The release collector reads their upstream license files, not source text.
export default defineConfig({
  plugins: [
    svelte({ preprocess: vitePreprocess({ script: true }) }),
    {
      name: 'observer-bundled-packages',
      generateBundle(_options, bundle) {
        const packages = new Set<string>(['svelte', 'vite']);
        for (const chunk of Object.values(bundle)) {
          if (chunk.type !== 'chunk') continue;
          for (const id of Object.keys(chunk.modules)) {
            const match = id.replace(/\\/g, '/').match(/\/node_modules\/((?:@[^/]+\/)?[^/]+)\//g)?.at(-1);
            if (match) packages.add(match.slice('/node_modules/'.length, -1));
          }
        }
        this.emitFile({type:'asset', fileName:'bundled-packages.json', source:JSON.stringify([...packages].sort())});
      }
    }
  ],
  base: './'
});
