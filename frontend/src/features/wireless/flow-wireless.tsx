import { Link } from 'react-router-dom';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { leaseIsFresh } from '@/features/wireless/types';
import { WirelessTable } from '@/features/wireless/wireless-table';
import { routes } from '@/lib/routes';
import { useFlow } from '@/providers/flow-provider';
import { useWireless } from '@/providers/wireless-provider';

export function FlowWireless() {
    const { error, state } = useWireless();
    const flow = useWirelessFlowBinding();
    const context = state?.contexts.find((item) => item.id === flow?.binding.contextId);
    const endpoint = state?.endpoints.find((item) => item.id === flow?.binding.endpointId);

    if (!flow || !context) {
        return (
            <Alert>
                <AlertTitle>No wireless binding</AlertTitle>
                <AlertDescription>{error ?? 'This flow has no wireless environment.'}</AlertDescription>
            </Alert>
        );
    }

    const blocked =
        !!error ||
        context.status !== 'ready' ||
        context.epoch !== flow.binding.epoch ||
        !leaseIsFresh(context.expiresAt) ||
        (flow.binding.endpointId &&
            (!endpoint ||
                endpoint.status !== 'verified' ||
                endpoint.revision !== flow.binding.endpointRevision ||
                !leaseIsFresh(endpoint.expiresAt)));

    return (
        <div className="space-y-4">
            {flow.evidence?.endpoint && (
                <p className="text-muted-foreground text-sm">
                    Target at creation: {flow.evidence.endpoint.mac} / {flow.evidence.endpoint.ip} · revision{' '}
                    {flow.binding.endpointRevision}
                </p>
            )}
            <Alert variant={blocked ? 'destructive' : 'default'}>
                <AlertTitle>
                    {blocked ? 'Environment changed — execution blocked' : 'Wireless environment bound'}
                </AlertTitle>
                <AlertDescription>
                    {blocked
                        ? context.reason || 'Revalidate the association and client identity, then create a new flow.'
                        : `${context.bssid} · epoch ${flow.binding.epoch}${endpoint ? ` · ${endpoint.mac} / ${endpoint.ip}` : ''}`}
                </AlertDescription>
            </Alert>
            <Button
                asChild
                variant="outline"
            >
                <Link to={routes.wirelessArea(context.areaId)}>Open wireless area</Link>
            </Button>
            <WirelessTable
                data={[context]}
                fields={[
                    ['bssid', 'BSSID'],
                    ['epoch', 'Current epoch'],
                    ['status', 'Status'],
                    ['address', 'Address'],
                    ['expiresAt', 'Lease expiry'],
                ]}
                label="Bound network"
                tableKey="flowNetwork"
            />
            {endpoint && (
                <WirelessTable
                    data={[endpoint]}
                    fields={[
                        ['mac', 'Client MAC'],
                        ['ip', 'Current IP'],
                        ['revision', 'Current revision'],
                        ['status', 'Identity'],
                    ]}
                    label="Bound client"
                    tableKey="flowClient"
                />
            )}
        </div>
    );
}

export function useWirelessFlowBinding() {
    const { flowId } = useFlow();
    const { state } = useWireless();

    return state?.flows.find((flow) => String(flow.flowId) === flowId) ?? null;
}
