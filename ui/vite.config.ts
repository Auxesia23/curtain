import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// Go API target for `vite dev`. Production builds are served by the Go binary.
const apiTarget = process.env.CURTAIN_API ?? 'http://localhost:8080';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// Render every route in the browser without SSR or prerendering.
			// The Go handler (see ui.go) uses `fallback: 'index.html'` for all
			// non-asset and non-API URLs.
			adapter: adapter({ fallback: 'index.html' }),

			// Resolve assets from the root ("/_app/..."). The SPA fallback may be
			// served from a nested URL such as /projects/1.
			paths: { relative: false }
		})
	],
	server: {
		proxy: {
			'/api': { target: apiTarget, changeOrigin: false }
		}
	}
});
