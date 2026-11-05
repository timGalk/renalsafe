# React clinician workspace

React/TypeScript with Vite, React Router and generated OpenAPI types. The locked versions are recorded in package-lock.json; TypeScript 5.9.3 is pinned to the API generator's supported peer range.

```sh
# From the repository root:
npm ci --prefix web
npm run check:types --prefix web
npm run build --prefix web
```

The build writes generated assets to `core/internal/httpapi/web/dist/`, which Go embeds. Build before running all Go packages or the database integration runner. Dependencies and generated bundles are ignored by Git; commit source and the lockfile with implementation changes. Docker performs a clean npm install, API-type check and frontend build before compiling Go, so it does not depend on host build artifacts.

After changing Go HTTP payloads/routes, regenerate the contract and client:

```sh
cd core
go run ./cmd/renalsafe-openapi > ../api/openapi.json
cd ..
npm run types --prefix web
npm run build --prefix web
```

`npm run check:types` compares generated types against the checked-in contract without modifying source. CI builds the frontend before vet/integration tests. The Go contract generation equality check remains separate.

For development, start the core service at loopback port 8080, then run `npm run dev --prefix web`. Vite proxies `/api`, `/healthz` and `/readyz` to the local core. Production serves browser assets and API from the same Go origin; no Node runtime or external CDN is needed after build.

Components are separated into session/API/draft infrastructure, patient/history screens, assessment/result screens, source/review/document screens and staff/audit/status screens. React owns forms and rendering; the previous imperative browser implementation is no longer served. No credentials or clinical inputs are persisted in browser localStorage/sessionStorage. Backend authorization remains authoritative.

Architecture references consulted 2026-11-05: [React's build-from-scratch guidance](https://react.dev/learn/build-a-react-app-from-scratch), [Vite production build](https://vite.dev/guide/build), [React Router navigation blocking](https://reactrouter.com/api/hooks/useBlocker). The same-origin static build is a project choice to preserve the existing Go deployment.
