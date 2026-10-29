import { z } from 'zod';

const list = <T extends z.ZodType>(schema: T) =>
    z
        .array(schema)
        .nullish()
        .transform((value) => value ?? []);
const area = z.object({ bssids: list(z.string()), cidrs: list(z.string()), id: z.string(), name: z.string() });
const node = z.object({
    areaId: z.string(),
    capabilities: list(z.string()),
    id: z.string(),
    lastSeen: z.string(),
    name: z.string(),
    status: z.string(),
});
const asset = z.object({
    areaId: z.string(),
    bssid: z.string(),
    frequency: z.number(),
    id: z.string(),
    inScope: z.boolean(),
    kind: z.string(),
    lastSeen: z.string(),
    mac: z.string(),
    nodeId: z.string(),
    security: z.string(),
    signal: z.number(),
    ssid: z.string(),
    status: z.string(),
});
const network = z.object({
    address: z.string(),
    areaId: z.string(),
    bssid: z.string(),
    daemonId: z.string(),
    epoch: z.number(),
    expiresAt: z.string(),
    id: z.string(),
    interface: z.string(),
    network: z.string(),
    networkId: z.string(),
    nodeId: z.string(),
    reason: z.string(),
    status: z.string(),
});
const endpoint = z.object({
    contextId: z.string(),
    expiresAt: z.string(),
    id: z.string(),
    ip: z.string(),
    mac: z.string(),
    revision: z.number(),
    status: z.string(),
});
const binding = z.object({
    contextId: z.string(),
    endpointId: z.string().optional(),
    endpointRevision: z.number().optional(),
    epoch: z.number(),
});
const job = z.object({
    areaId: z.string(),
    bssid: z.string(),
    createdAt: z.string(),
    id: z.string(),
    kind: z.string(),
    message: z.string(),
    nodeId: z.string(),
    status: z.string(),
});
const event = z.object({
    createdAt: z.string(),
    id: z.string(),
    kind: z.string(),
    message: z.string(),
    subjectId: z.string(),
});
const observation = z.object({
    areaId: z.string(),
    bssid: z.string(),
    frequency: z.number(),
    id: z.string(),
    kind: z.string(),
    mac: z.string(),
    nodeId: z.string(),
    observedAt: z.string(),
    security: z.string(),
    signal: z.number(),
    ssid: z.string(),
});

export const wirelessStateSchema = z.object({
    areas: list(area),
    assets: list(asset),
    contexts: list(network),
    endpoints: list(endpoint),
    events: list(event),
    flows: list(
        z.object({
            binding,
            evidence: z
                .object({ area, boundAt: z.string(), context: network, endpoint: endpoint.optional() })
                .optional(),
            flowId: z.number(),
            status: z.string(),
        }),
    ),
    jobs: list(job),
    nodes: list(node),
    observations: list(observation),
    revision: z.number(),
});

export type WirelessArea = z.infer<typeof area>;
export type WirelessAsset = z.infer<typeof asset>;
export type WirelessContext = z.infer<typeof network>;
export type WirelessEndpoint = z.infer<typeof endpoint>;
export type WirelessJob = z.infer<typeof job>;
export type WirelessNode = z.infer<typeof node>;
export type WirelessState = z.infer<typeof wirelessStateSchema>;

export const leaseIsFresh = (expiresAt: string, now = Date.now()) => Date.parse(expiresAt) > now;
