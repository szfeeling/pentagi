# Wireless extension validation

Validated on 2026-10-20 with Go 1.26.5, Node 24.17.0 and an isolated PostgreSQL 16 instance. Tests used controlled observations and a local HTTPS node-protocol fixture. No physical Wi-Fi adapter, real access point, client penetration test or paid LLM run was used.

## Completed checks

- Backend build for `cmd/pentagi` and `cmd/wireless-node`; Linux amd64 collector cross-compilation.
- Race checks for wireless domain, collector, tool execution, flow controller, GraphQL and configuration packages. The broader relevant package run also passed providers, server packages and Docker tests.
- PostgreSQL migration up/down, restart persistence, credential lookup, per-user isolation, concurrent writes, transaction rollback and active-flow filtering by owner and status.
- Domain scenarios for expired evidence, duplicate reports, node restart and old-boot replay, node revocation, operation idempotency, cancellation receipts, unknown outcomes, channel/address/runtime changes, stable AP identity across observers, client IP changes, ambiguous MAC/IP claims and cached client evidence that cannot be renewed by a heartbeat.
- Node-protocol scenarios for periodic passive observations, BSS cache-age preservation, cancellation acknowledged only after local cleanup, and leased target verification through the public collector loop. Reply parsing preserves the actual responding MAC and confirmation expiry; expired, foreign-context and out-of-subnet targets are excluded.
- Tool execution checks for refusal before a stale call and cancellation of an in-flight call after environment invalidation.
- Frontend production build and 200 relevant Vitest checks, including Apollo snapshot ordering, existing flow forms, tab routing, DataTable and route-manifest coverage.
- Three browser scenarios: same-SSID APs remain distinct and disappearance disables connection; a verified client opens the shared flow form; a changed client revision requires explicit acceptance. The repository's mock-tier Playwright fixtures ran against the production bundle using installed Chrome with video disabled because the pinned Playwright browser and ffmpeg were absent. The test result file reported `passed`.
- Go domain tests under America/Los_Angeles, Asia/Shanghai and Pacific/Kiritimati; Apollo tests under Pacific/Kiritimati with a Chinese locale.
- Relevant ESLint, Prettier, gofmt and whitespace checks.

Nineteen deliberate mutations were detected in temporary copies or compiler overlays: endpoint revision checks, report replay checks, credential redaction, stable asset identity, scan frequency parsing, channel invalidation, container cleanup, SQL commit, tool execution guards, idle observation scheduling, ambiguous client claims, Apollo revision ordering, cached client expiry, scan cache age, active-flow targeting, SQL flow-status filtering, ARP reply identity, ARP confirmation expiry and collector dispatch wiring. Production source files were not mutated.

## Generated artifacts

gqlgen, Swagger and frontend GraphQL artifacts were regenerated with the pinned repository dependencies. SQLC 1.27.0 generated the affected database model from the migration using its built-in analyzer. Its model diff consists only of the new `WirelessState` struct and its `time` import; existing models were otherwise unchanged. Full SQLC query analysis requires a migrated pgvector database. The existing knowledge queries require that live analyzer and were excluded from the isolated model generation; those query sources and generated implementations were not changed.

## Remaining deployment validation

Validate the physical interface and driver, saved NetworkManager profiles, exact BSSID association, runtime subnet and packet routing, ipvlan/macvlan compatibility, client isolation, neighbor freshness, DHCP reassignment, AP disappearance and channel changes. Also exercise control-link loss, process restart, daemon failure and cleanup failure on the deployment hardware. A ready context establishes the reported association and runtime identity; it does not prove every client is reachable or provide cryptographic device attribution.

See [wireless node operations](../../examples/guides/wireless-node/README.md) for setup, limits and the current implementation boundary.
