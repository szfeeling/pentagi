# Wireless areas and Linux collection nodes

The wireless extension adds a control plane and a Linux collector to existing PentAGI flows. Pentester, Coder, Installer, terminal tools, resources, memory and reports remain the execution path for network and client assessments. The collector performs passive Wi-Fi discovery and connects operator-provisioned NetworkManager profiles. Credentials stay on the node.

This implementation has automated state-machine, database and node-protocol tests. Physical radio, driver, access-point and client interoperability require validation on your equipment.

## Deploy the control plane

Build the backend with Go 1.26.5 and the frontend with the Node version in `frontend/.nvmrc`. The backend applies `20261020_120000_wireless_state.sql` during its normal migration step. Restart the backend and sign in again to refresh the new wireless privileges in the UI.

Open **Wireless**, create an area, enter the authorized BSSIDs and client CIDRs, and open that area to enroll a node. Empty scope permits observation only. Enrollment returns a token once; save it in a root-readable file on the node. Node tokens cannot use the regular application API. Creating or revoking a node requires an interactive user session.

The first release uses one configured Docker daemon for both ordinary and wireless flow containers. Configure the backend's existing `DOCKER_HOST`, `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH` for the Linux daemon that hosts the wireless interface. Use the existing authenticated Docker transport; the extension does not expose an additional Docker API. Set `DOCKER_INSIDE=false`. A context reporting a different daemon is refused. Arbitrary selection among multiple remote Docker daemons is not implemented.

## Prepare the Linux node

Use a dedicated Wi-Fi interface and a separate management link. Required programs are `iw`, `ip`, iputils `arping`, `nmcli` and the Docker CLI. NetworkManager must manage the dedicated interface. The node process needs permission to scan and configure that interface, read neighboring devices, send selected-target ARP probes, and manage its flow containers. Keep the host clock synchronized with the control plane.

Create saved NetworkManager connection profiles locally with the required authentication settings. Pin each profile's `802-11-wireless.bssid` to the exact authorized BSSID. The node accepts connection UUIDs, checks the pin, and refuses SSID-only association.

Create a Docker ipvlan or macvlan network on the dedicated interface, using the associated network's subnet and a container address pool reserved outside DHCP. The node checks its parent interface and subnet; the backend checks its immutable network ID and daemon ID. Typical Wi-Fi station drivers cannot carry additional macvlan MAC addresses, so verify the driver's behavior and the network's routing before using the runtime. IPvlan and macvlan interoperability is equipment-dependent. The application does not create this network, alter DHCP or install egress firewall rules.

Example profile mapping, with locally configured connection UUIDs:

```json
{
  "02:00:00:00:00:01": "11111111-2222-4333-8444-555555555555"
}
```

Build the collector on Linux, or cross-compile it from the backend directory:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wireless-node ./cmd/wireless-node
```

Run it with the node ID and token obtained from the UI:

```sh
./wireless-node \
  -server https://pentagi.example.com \
  -node NODE_UUID \
  -token-file /etc/pentagi-wireless/token \
  -interface wlan1 \
  -network wireless-lab \
  -profiles /etc/pentagi-wireless/profiles.json \
  -state /var/lib/pentagi-wireless/state.json \
  -scan-interval 15s
```

Use a trusted HTTPS certificate. Run one collector process per interface and state file. Keep the state directory across restarts: its journal is written before association and drives cleanup on startup. A zero scan interval disables automatic observation. While idle, the collector scans passively with the scan-cache flush flag at the configured interval; an active association suspends scanning so a shared radio does not leave its channel. Manual observation jobs use the same serialized radio slot.

## Run an assessment

1. Check the node heartbeat and its reported capabilities. Passive discoveries appear in the area's asset table; the **Observe** action requests another scan.
2. Select a fresh, authorized AP and choose **Connect using saved node profile**. Follow the operation status until an association and prepared runtime produce a network context. A ready context records association and runtime identity; it does not establish that every client is reachable.
3. Open **Networks & clients**. The collector reports reachable IPv4 neighbor entries with recent confirmation statistics as MAC/IP candidates. Client expiry is anchored to that confirmation, so repeated heartbeats cannot renew cached evidence. Only clients in the configured CIDRs can be selected. Sleeping, isolated or otherwise absent clients are not invented or assigned an old IP. An active client flow authorizes periodic ARP verification of its selected address. The actual responding MAC is reported; a changed identity blocks the old binding. Finished and failed flows do not request verification. Waiting flows keep their selected target under verification until finished, revoked or disconnected.
4. Start **New flow** for the network, or **Test client** for a current binding. The existing form supports Automation and Assistant, provider selection, templates and resources. It pins the network epoch and optional client revision and supplies the captured environment evidence to the existing agents.
5. Use the existing tasks, terminal, agents, memory and report interfaces. The flow's **Wireless** tab shows the bound context and current client identity. Use a separate interface or disconnect before running another radio operation.

## Dynamic behavior

| Change | Result |
| --- | --- |
| No recent AP observation | Asset becomes stale after 30 seconds and lost after 5 minutes; the row and recent history remain available |
| Same BSSID observed by another node in one area | Asset ID stays stable; each observation retains its node and timestamp |
| Same SSID on different BSSIDs | Separate assets |
| BSSID, channel, local address, runtime network or node boot changes | Context becomes invalid and its epoch advances; a heartbeat cannot revive it |
| Client IP changes, conflicts or expires | Old client revision is refused; a new flow requires explicit selection of the current binding |
| Control-plane or database checks fail | Tools stop proceeding; running calls are cancelled and the flow container is stopped |
| Node loses the control plane | Local lease expires after 20 seconds; the collector cancels operations, stops labeled flow containers and disconnects the dedicated interface |
| Cancel requested | Operation remains cancelling until cleanup is acknowledged; lease expiry yields unknown |
| Cleanup fails or an operation's outcome is unknown | Reconciliation runs cleanup; the original active operation is not replayed |
| Context invalidated by the operator | Execution is refused and the node receives an explicit context revocation |

Each heartbeat verifies at most eight selected client targets, rotating across active bindings and restricting probes to the current context and subnet. Replies expire 29 seconds after the probe; failed probes cannot extend cached evidence. Keep the active target count small enough to refresh before expiry; excess load fails closed rather than extending evidence.

The control-plane lease is 30 seconds. Node heartbeats run every 5 seconds. Tool and idle-runtime checks run every 2 seconds, with bounded database and container cleanup deadlines. These are scheduling bounds, not physical-disconnection guarantees when an operating system or Docker daemon is unresponsive.

## Data, APIs and operating limits

State is isolated by user and deployment schema in PostgreSQL. Writes serialize under a row lock and advance a revision. GraphQL `wirelessState`, `wirelessUpdated` and `wirelessCommand` use the existing authentication and Apollo WebSocket connection. The JSON payload is validated by the backend command decoder and the frontend Zod schema. Reconnection and periodic snapshots catch up state across server instances. Nodes use the separate token-authenticated `POST /api/v1/wireless/node/report` ingress, with boot IDs, monotonic sequences, leases and operation receipts.

The initial store retains at most 100 areas, 100 nodes, 5,000 assets, 2,000 observations, 1,000 jobs, 1,000 contexts, 5,000 endpoint bindings, 1,000 flow bindings and 500 events per user. Observation and event histories are rolling windows. Other limits reject new objects explicitly; archival and normalized high-volume storage are future work. Flow bindings capture immutable environment evidence and the same evidence is included in the existing flow input for reporting.

Node and runtime checks establish the execution environment, while BSSID and CIDR scope restrict selection and accompany the agent's instructions. They are not an egress firewall or cryptographic proof of a device's identity. Apply the deployment's network isolation and validate client attribution independently where required.

Kismet ingestion, coverage heatmaps, historical replay mode, distributed radio scheduling, automatic credential acquisition, arbitrary remote runtime selection, normalized retention management and hardware-specific attack adapters are not implemented in this release. Existing general-purpose agents and terminal tools remain available after a valid connection.

## Verification

From `backend/`:

```sh
go test -race ./pkg/wireless/... ./pkg/tools ./pkg/controller ./pkg/graph
PENTAGI_TEST_DSN='postgres://USER:PASSWORD@HOST/DATABASE?sslmode=disable' \
  go test -race -tags postgres ./pkg/wireless/...
go build ./cmd/pentagi ./cmd/wireless-node
```

From `frontend/`, after installing the pinned dependencies and Playwright browser:

```sh
pnpm run build
pnpm exec vitest run src/pages/flows/new-flow.test.tsx src/features/flows/flow-central-tabs.test.tsx src/hooks/use-flow-tab-detection.test.tsx src/components/ui/data-table.test.tsx e2e/route-manifest.unit.test.ts
pnpm exec playwright test --config e2e/playwright.config.ts e2e/specs/wireless.spec.ts --project=mock-chromium --workers=1
```

Before deploying against physical equipment, verify passive scan support, saved-profile association, container addressing and routing, client isolation, AP disappearance, channel changes, DHCP reassignment, control-link loss, process restart and cleanup failure. Test fixtures validate protocol behavior and UI state; they do not validate radio interoperability.

The scan flush flag and BSS ages follow the [iw implementation](https://kernel.googlesource.com/pub/scm/linux/kernel/git/jberg/iw/+/refs/heads/master/scan.c). Neighbor confirmation ages are read in seconds, matching [iproute2](https://github.com/iproute2/iproute2/blob/main/ip/ipneigh.c). Missing or expired confirmation statistics do not produce a verified client binding.

ARP reply parsing follows [iputils arping](https://github.com/iputils/iputils/blob/master/arping.c), with `LC_ALL=C` set for collector commands. This verifies a recent layer-2 response, not cryptographic device identity.
