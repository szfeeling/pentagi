import { useId } from 'react';

import type { WirelessBindingInput } from '@/graphql/types';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { leaseIsFresh } from '@/features/wireless/types';
import { useWireless } from '@/providers/wireless-provider';

export function WirelessFlowFields({
    binding,
    onChange,
}: {
    binding: null | WirelessBindingInput;
    onChange: (binding: WirelessBindingInput) => void;
}) {
    const { error, state } = useWireless();
    const networkLabelId = useId();
    const targetLabelId = useId();
    const networks =
        state?.contexts.filter((context) => context.status === 'ready' && leaseIsFresh(context.expiresAt)) ?? [];
    const endpoints =
        state?.endpoints.filter(
            (endpoint) =>
                endpoint.contextId === binding?.contextId &&
                endpoint.status === 'verified' &&
                leaseIsFresh(endpoint.expiresAt),
        ) ?? [];
    const context = networks.find((network) => network.id === binding?.contextId);
    const currentEndpoint = endpoints.find((endpoint) => endpoint.id === binding?.endpointId);

    return (
        <div className="space-y-3 rounded-lg border p-4">
            <Label htmlFor={networkLabelId}>Wireless network</Label>
            <Select
                onValueChange={(id) => {
                    const network = networks.find((item) => item.id === id);

                    if (network) {
                        onChange({ contextId: id, epoch: network.epoch });
                    }
                }}
                value={binding?.contextId ?? ''}
            >
                <SelectTrigger
                    className="w-full"
                    id={networkLabelId}
                >
                    <SelectValue placeholder="Select a live network context" />
                </SelectTrigger>
                <SelectContent>
                    {networks.map((network) => (
                        <SelectItem
                            key={network.id}
                            value={network.id}
                        >
                            {network.bssid} · {network.address} · epoch {network.epoch}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            {context && (
                <>
                    <Label htmlFor={targetLabelId}>Target</Label>
                    <Select
                        onValueChange={(id) => {
                            const endpoint = endpoints.find((item) => item.id === id);
                            onChange({
                                contextId: context.id,
                                endpointId: endpoint?.id,
                                endpointRevision: endpoint?.revision,
                                epoch: context.epoch,
                            });
                        }}
                        value={binding?.endpointId || 'network'}
                    >
                        <SelectTrigger
                            className="w-full"
                            id={targetLabelId}
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="network">Network assessment</SelectItem>
                            {endpoints.map((endpoint) => (
                                <SelectItem
                                    key={endpoint.id}
                                    value={endpoint.id}
                                >
                                    {endpoint.mac} · {endpoint.ip} · revision {endpoint.revision}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </>
            )}
            {(!networks.length || error) && (
                <Alert>
                    <AlertTitle>No live wireless network</AlertTitle>
                    <AlertDescription>
                        {error ?? 'Open a wireless area, enroll a node and connect an authorized access point first.'}
                    </AlertDescription>
                </Alert>
            )}
            {context && currentEndpoint && currentEndpoint.revision !== binding?.endpointRevision && (
                <Button
                    onClick={() =>
                        onChange({
                            contextId: context.id,
                            endpointId: currentEndpoint.id,
                            endpointRevision: currentEndpoint.revision,
                            epoch: context.epoch,
                        })
                    }
                    variant="outline"
                >
                    Use updated client {currentEndpoint.ip} · revision {currentEndpoint.revision}
                </Button>
            )}
            <p className="text-muted-foreground text-sm">
                The selected epoch and client revision are pinned when you submit. An expired or changed identity blocks
                execution.
            </p>
        </div>
    );
}
