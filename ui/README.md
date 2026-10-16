# Curtain Web UI

This directory contains Curtain's SvelteKit single-page application. It handles
the web experience, including account setup, project and environment management,
client-side encryption, and the local key vault.

## Development

From the repository root, start the API and UI in separate terminals:

```bash
make dev-api
make dev-ui
```

The API defaults to `http://localhost:8080`. Vite proxies `/api` requests there.
To run the UI directly from this directory, use `npm run dev`.

## Build

Build the static SPA from this directory:

```bash
npm run build
```

The repository's `make build` command builds the UI and embeds its output in the
Go server binary. Rebuild the server after changing UI code so the binary serves
the latest assets.

## Project layout

- `src/routes/` contains the SvelteKit pages and route loaders.
- `src/lib/api/` contains API clients and shared request/response types.
- `src/lib/crypto/` contains browser-side key derivation and encryption.
- `src/lib/state/` contains session and vault state.
- `scripts/e2e.mjs` tests the API and crypto flow.
- `scripts/ui-e2e.mjs` tests the browser flow with Playwright.

## Checks and tests

```bash
npm run check
npm run e2e
npm run e2e:ui
```

The E2E scripts need a running server. They default to `http://localhost:8080`;
set `BASE` to target another test server. Install Playwright Chromium once with
`npx playwright install chromium` before running `npm run e2e:ui`.
