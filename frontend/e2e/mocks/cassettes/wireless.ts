import type { WirelessState } from '@/features/wireless/types';

import type { Cassette } from '../cassette.ts';

import { baseQueries, baseRest } from './base.ts';

export const wirelessSnapshot = (): WirelessState => ({
    areas: [{ bssids: ['02:00:00:00:00:01'], cidrs: ['192.168.50.0/24'], id: 'lab', name: 'E2E Wireless Lab' }],
    assets: [
        {
            areaId: 'lab',
            bssid: '02:00:00:00:00:01',
            frequency: 2412,
            id: 'ap-1',
            inScope: true,
            kind: 'ap',
            lastSeen: '2026-01-15T12:00:00Z',
            mac: '02:00:00:00:00:01',
            nodeId: 'sensor',
            security: 'SAE',
            signal: -45,
            ssid: 'E2E SharedSSID',
            status: 'visible',
        },
        {
            areaId: 'lab',
            bssid: '02:00:00:00:00:02',
            frequency: 5180,
            id: 'ap-2',
            inScope: false,
            kind: 'ap',
            lastSeen: '2026-01-15T12:00:00Z',
            mac: '02:00:00:00:00:02',
            nodeId: 'sensor',
            security: 'SAE',
            signal: -62,
            ssid: 'E2E SharedSSID',
            status: 'visible',
        },
    ],
    contexts: [
        {
            address: '192.168.50.10/24',
            areaId: 'lab',
            bssid: '02:00:00:00:00:01',
            daemonId: 'daemon-1',
            epoch: 1,
            expiresAt: '2026-01-15T12:00:30Z',
            id: 'context-1',
            interface: 'wlan1',
            network: 'lab-runtime',
            networkId: 'network-1',
            nodeId: 'sensor',
            reason: '',
            status: 'ready',
        },
    ],
    endpoints: [
        {
            contextId: 'context-1',
            expiresAt: '2026-01-15T12:00:30Z',
            id: 'client-1',
            ip: '192.168.50.20',
            mac: '02:00:00:00:00:03',
            revision: 1,
            status: 'verified',
        },
    ],
    events: [],
    flows: [],
    jobs: [],
    nodes: [
        {
            areaId: 'lab',
            capabilities: ['scan', 'connect', 'disconnect'],
            id: 'sensor',
            lastSeen: '2026-01-15T12:00:00Z',
            name: 'E2E Sensor',
            status: 'online',
        },
    ],
    observations: [],
    revision: 1,
});

export const wirelessCassette = (): Cassette => {
    const state = wirelessSnapshot();
    const lost = { ...state, assets: state.assets.map((asset) => ({ ...asset, status: 'lost' })), revision: 2 };

    return {
        mutations: {
            wirelessCommand: [
                {
                    data: { wirelessCommand: { id: 'job-1' } },
                    variables: {
                        command: {
                            bssid: '02:00:00:00:00:01',
                            expectedRevision: 1,
                            jobKind: 'connect',
                            kind: 'job.start',
                            nodeId: 'sensor',
                        },
                    },
                },
            ],
        },
        queries: {
            ...baseQueries(),
            wirelessState: [
                { data: { wirelessState: state } },
                { data: { wirelessState: lost }, whenFlag: 'radioLost' },
            ],
        },
        rest: baseRest(),
        subscriptions: {
            wirelessUpdated: [{ frames: [{ payload: { data: { wirelessUpdated: lost } }, whenFlag: 'radioLost' }] }],
        },
    };
};
