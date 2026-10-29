import { Radio, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Link, useLocation, useParams, useSearchParams } from 'react-router-dom';

import { AppHeader, AppHeaderContent, AppHeaderTitle } from '@/components/layouts/app/app-header';
import ConfirmationDialog from '@/components/shared/confirmation-dialog';
import { ErrorState } from '@/components/shared/error-state';
import { LoadingState } from '@/components/shared/loading-state';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { leaseIsFresh, type WirelessAsset } from '@/features/wireless/types';
import { WirelessSetupForm } from '@/features/wireless/wireless-setup-form';
import { WirelessTable } from '@/features/wireless/wireless-table';
import { routes } from '@/lib/routes';
import { useWireless } from '@/providers/wireless-provider';

function Wireless() {
    const { areaId } = useParams();
    const location = useLocation();
    const [params, setParams] = useSearchParams();
    const { canEdit, canView, command, error, isLoading, isSubmitting, refetch, state } = useWireless();
    const [setup, setSetup] = useState<'area' | 'node' | false>(false);
    const [now, setNow] = useState(() => Date.now());
    useEffect(() => {
        const timer = window.setInterval(() => setNow(Date.now()), 1000);

        return () => window.clearInterval(timer);
    }, []);
    const [revokingNode, setRevokingNode] = useState<null | { id: string; name: string }>(null);
    const selected = state?.assets.find((asset) => asset.id === params.get('asset'));
    const view = params.get('view') ?? (location.pathname === routes.wirelessNodes ? 'nodes' : 'assets');
    const area = state?.areas.find((item) => item.id === areaId);
    const chooseAsset = (asset?: WirelessAsset) =>
        setParams((previous) => {
            const next = new URLSearchParams(previous);

            if (asset) {
                next.set('asset', asset.id);
            } else {
                next.delete('asset');
            }

            return next;
        });
    const start = (nodeId: string, jobKind: string, bssid = '') =>
        void command({ bssid, jobKind, kind: 'job.start', nodeId, requestId: crypto.randomUUID() });

    if (!canView) {
        return (
            <ErrorState
                message="Your account needs wireless.view permission."
                title="Wireless access unavailable"
            />
        );
    }

    if (isLoading) {
        return <LoadingState title="Loading wireless environment" />;
    }

    if (!state) {
        return (
            <ErrorState
                message={error}
                onRetry={refetch}
                title="Unable to load wireless environment"
            />
        );
    }

    if (areaId && !area) {
        return <ErrorState title="Wireless area not found" />;
    }

    const nodes = state.nodes.filter((node) => !areaId || node.areaId === areaId);
    const assets = state.assets.filter((asset) => !areaId || asset.areaId === areaId);
    const contexts = state.contexts.filter((context) => !areaId || context.areaId === areaId);
    const jobs = state.jobs.filter((job) => !areaId || job.areaId === areaId);
    const disabled = !canEdit || isSubmitting || !!error;

    return (
        <>
            <AppHeader>
                <AppHeaderContent>
                    <AppHeaderTitle>{area?.name ?? 'Wireless environment'}</AppHeaderTitle>
                </AppHeaderContent>
            </AppHeader>
            <div className="space-y-6 p-4 md:p-6">
                <div className="flex flex-wrap items-center gap-2">
                    <Button
                        asChild
                        variant="outline"
                    >
                        <Link to={routes.wireless}>
                            <Radio />
                            All areas
                        </Link>
                    </Button>
                    <Button
                        asChild
                        variant="outline"
                    >
                        <Link to={routes.wirelessNodes}>Nodes</Link>
                    </Button>
                    <Button
                        aria-label="Refresh wireless state"
                        onClick={() => void refetch()}
                        variant="outline"
                    >
                        <RefreshCw />
                    </Button>
                    <Button
                        disabled={disabled}
                        onClick={() => setSetup('area')}
                    >
                        New area
                    </Button>
                    {areaId && (
                        <Button
                            disabled={disabled}
                            onClick={() => setSetup('node')}
                        >
                            Enroll node
                        </Button>
                    )}
                </div>
                {error && (
                    <Alert variant="destructive">
                        <AlertTitle>Live state unavailable</AlertTitle>
                        <AlertDescription>{error} Actions are disabled until state is refreshed.</AlertDescription>
                    </Alert>
                )}
                <div className="grid gap-4 sm:grid-cols-3">
                    {[
                        ['Visible assets', assets.filter((asset) => asset.status === 'visible').length],
                        ['Online nodes', nodes.filter((node) => node.status === 'online').length],
                        [
                            'Ready networks',
                            contexts.filter(
                                (context) => context.status === 'ready' && leaseIsFresh(context.expiresAt, now),
                            ).length,
                        ],
                    ].map(([label, value]) => (
                        <Card key={label}>
                            <CardHeader>
                                <CardTitle className="text-sm">{label}</CardTitle>
                            </CardHeader>
                            <CardContent className="text-3xl font-semibold">{value}</CardContent>
                        </Card>
                    ))}
                </div>
                {!areaId && location.pathname !== routes.wirelessNodes && (
                    <WirelessTable
                        actions={(item) => (
                            <Button
                                asChild
                                size="sm"
                                variant="outline"
                            >
                                <Link to={routes.wirelessArea(item.id)}>Open area</Link>
                            </Button>
                        )}
                        data={state.areas}
                        fields={[
                            ['name', 'Area'],
                            ['bssids', 'Authorized BSSIDs'],
                            ['cidrs', 'Client scope'],
                        ]}
                        label="Areas"
                        tableKey="areas"
                    />
                )}
                {!state.areas.length && (
                    <Alert>
                        <AlertTitle>Start with an area</AlertTitle>
                        <AlertDescription>
                            Create an area and open it to enroll a Linux collection node. Live observations appear after
                            the node connects.
                        </AlertDescription>
                    </Alert>
                )}
                <Tabs
                    onValueChange={(value) =>
                        setParams((previous) => {
                            const next = new URLSearchParams(previous);
                            next.set('view', value);

                            return next;
                        })
                    }
                    value={view}
                >
                    <TabsList className="h-auto flex-wrap">
                        <TabsTrigger value="assets">Assets</TabsTrigger>
                        <TabsTrigger value="nodes">Nodes</TabsTrigger>
                        <TabsTrigger value="contexts">Networks & clients</TabsTrigger>
                        <TabsTrigger value="jobs">Operations</TabsTrigger>
                        <TabsTrigger value="events">Timeline</TabsTrigger>
                    </TabsList>
                </Tabs>
                {view === 'assets' && (
                    <WirelessTable
                        data={assets}
                        fields={[
                            ['ssid', 'SSID'],
                            ['mac', 'MAC'],
                            ['kind', 'Type'],
                            ['frequency', 'MHz'],
                            ['signal', 'dBm'],
                            ['status', 'Presence'],
                            ['inScope', 'Scope'],
                        ]}
                        label="Wireless assets"
                        onRowClick={chooseAsset}
                        tableKey="assets"
                    />
                )}
                {view === 'nodes' && (
                    <WirelessTable
                        actions={(node) => (
                            <div className="flex gap-2">
                                <Button
                                    disabled={
                                        disabled || node.status !== 'online' || !node.capabilities.includes('scan')
                                    }
                                    onClick={() => start(node.id, 'scan')}
                                    size="sm"
                                >
                                    Observe
                                </Button>
                                <Button
                                    disabled={
                                        disabled ||
                                        node.status !== 'online' ||
                                        !node.capabilities.includes('disconnect')
                                    }
                                    onClick={() => start(node.id, 'disconnect')}
                                    size="sm"
                                    variant="outline"
                                >
                                    Disconnect
                                </Button>
                                <Button
                                    disabled={disabled || node.status === 'revoked'}
                                    onClick={() => setRevokingNode(node)}
                                    size="sm"
                                    variant="outline"
                                >
                                    Revoke
                                </Button>
                            </div>
                        )}
                        data={nodes}
                        fields={[
                            ['name', 'Node'],
                            ['status', 'Status'],
                            ['capabilities', 'Capabilities'],
                            ['lastSeen', 'Last heartbeat'],
                        ]}
                        label="Nodes"
                        tableKey="nodes"
                    />
                )}
                {view === 'contexts' && (
                    <div className="space-y-6">
                        <WirelessTable
                            actions={(context) => (
                                <div className="flex gap-2">
                                    <Button
                                        asChild
                                        disabled={
                                            disabled ||
                                            context.status !== 'ready' ||
                                            !leaseIsFresh(context.expiresAt, now)
                                        }
                                        size="sm"
                                    >
                                        <Link
                                            aria-disabled={
                                                disabled ||
                                                context.status !== 'ready' ||
                                                !leaseIsFresh(context.expiresAt, now)
                                            }
                                            onClick={(event) => {
                                                if (
                                                    disabled ||
                                                    context.status !== 'ready' ||
                                                    !leaseIsFresh(context.expiresAt, now)
                                                ) {
                                                    event.preventDefault();
                                                }
                                            }}
                                            to={routes.newWirelessFlow(context.id)}
                                        >
                                            New flow
                                        </Link>
                                    </Button>
                                    <Button
                                        disabled={disabled || context.status !== 'ready'}
                                        onClick={() => void command({ id: context.id, kind: 'context.invalidate' })}
                                        size="sm"
                                        variant="outline"
                                    >
                                        Invalidate
                                    </Button>
                                </div>
                            )}
                            data={contexts}
                            fields={[
                                ['bssid', 'BSSID'],
                                ['address', 'Address'],
                                ['epoch', 'Epoch'],
                                ['status', 'Status'],
                                ['reason', 'Reason'],
                                ['expiresAt', 'Lease expiry'],
                            ]}
                            label="Network contexts"
                            tableKey="contexts"
                        />
                        <WirelessTable
                            actions={(endpoint) => {
                                const context = contexts.find((item) => item.id === endpoint.contextId);
                                const allowed =
                                    !disabled &&
                                    endpoint.status === 'verified' &&
                                    leaseIsFresh(endpoint.expiresAt, now) &&
                                    context?.status === 'ready' &&
                                    leaseIsFresh(context.expiresAt, now);

                                return allowed ? (
                                    <Button
                                        asChild
                                        size="sm"
                                    >
                                        <Link to={routes.newWirelessFlow(endpoint.contextId, endpoint.id)}>
                                            Test client
                                        </Link>
                                    </Button>
                                ) : (
                                    <Badge variant="outline">Revalidation required</Badge>
                                );
                            }}
                            data={state.endpoints.filter((endpoint) =>
                                contexts.some((context) => context.id === endpoint.contextId),
                            )}
                            fields={[
                                ['mac', 'Client MAC'],
                                ['ip', 'Current IP'],
                                ['revision', 'Binding revision'],
                                ['status', 'Identity'],
                                ['expiresAt', 'Lease expiry'],
                            ]}
                            label="Client bindings"
                            tableKey="clients"
                        />
                    </div>
                )}
                {view === 'jobs' && (
                    <WirelessTable
                        actions={(job) => (
                            <Button
                                disabled={disabled || !['queued', 'running', 'unknown'].includes(job.status)}
                                onClick={() => void command({ id: job.id, kind: 'job.cancel' })}
                                size="sm"
                                variant="outline"
                            >
                                Cancel & reconcile
                            </Button>
                        )}
                        data={jobs}
                        fields={[
                            ['kind', 'Operation'],
                            ['bssid', 'BSSID'],
                            ['status', 'Status'],
                            ['message', 'Result'],
                            ['createdAt', 'Created'],
                        ]}
                        label="Operations"
                        tableKey="jobs"
                    />
                )}
                {view === 'events' && (
                    <WirelessTable
                        data={[...state.events].reverse()}
                        fields={[
                            ['kind', 'Event'],
                            ['subjectId', 'Subject'],
                            ['message', 'Detail'],
                            ['createdAt', 'Time'],
                        ]}
                        label="Events"
                        tableKey="events"
                    />
                )}
            </div>
            <ConfirmationDialog
                confirmText="Revoke"
                description="Revoke this node token and invalidate its wireless contexts. Existing flow runtimes will be stopped."
                handleConfirm={async () => {
                    if (revokingNode) {
                        await command({ id: revokingNode.id, kind: 'node.revoke' });
                    }
                }}
                handleOpenChange={(open) => {
                    if (!open) {
                        setRevokingNode(null);
                    }
                }}
                isOpen={!!revokingNode}
                itemName={revokingNode?.name}
                itemType="node"
            />
            <Sheet
                onOpenChange={(open) => {
                    if (!open) {
                        setSetup(false);
                    }
                }}
                open={!!setup}
            >
                <SheetContent className="overflow-auto">
                    <SheetHeader>
                        <SheetTitle>{setup === 'node' ? 'Enroll collection node' : 'Create wireless area'}</SheetTitle>
                        <SheetDescription>
                            {setup === 'node'
                                ? 'Use a dedicated Linux wireless interface.'
                                : 'Define the authorized test scope.'}
                        </SheetDescription>
                    </SheetHeader>
                    <div className="p-4">
                        {setup && (
                            <WirelessSetupForm
                                areaId={setup === 'node' ? areaId : undefined}
                                key={setup}
                                onComplete={() => setSetup(false)}
                            />
                        )}
                    </div>
                </SheetContent>
            </Sheet>
            <Sheet
                onOpenChange={(open) => {
                    if (!open) {
                        chooseAsset();
                    }
                }}
                open={!!selected}
            >
                <SheetContent className="overflow-auto">
                    <SheetHeader>
                        <SheetTitle>{selected?.ssid || selected?.mac || 'Wireless asset'}</SheetTitle>
                        <SheetDescription>
                            Observations describe where and when an asset was seen. A shared SSID does not establish
                            identity.
                        </SheetDescription>
                    </SheetHeader>
                    {selected && (
                        <div className="space-y-4 p-4">
                            <dl className="space-y-2 text-sm">
                                {Object.entries(selected).map(([key, value]) => (
                                    <div
                                        className="grid grid-cols-2 gap-2"
                                        key={key}
                                    >
                                        <dt className="text-muted-foreground">{key}</dt>
                                        <dd className="break-all">{String(value)}</dd>
                                    </div>
                                ))}
                            </dl>
                            {selected.kind === 'ap' && (
                                <Button
                                    disabled={
                                        disabled ||
                                        !selected.inScope ||
                                        selected.status !== 'visible' ||
                                        Date.parse(selected.lastSeen) + 30000 <= now ||
                                        !nodes.some(
                                            (node) =>
                                                node.id === selected.nodeId &&
                                                node.status === 'online' &&
                                                node.capabilities.includes('connect'),
                                        )
                                    }
                                    onClick={() => start(selected.nodeId, 'connect', selected.mac)}
                                >
                                    Connect using saved node profile
                                </Button>
                            )}
                            <h3 className="font-medium">Recent observations</h3>
                            {state.observations
                                .filter(
                                    (item) =>
                                        item.areaId === selected.areaId &&
                                        item.mac === selected.mac &&
                                        item.kind === selected.kind,
                                )
                                .slice(-20)
                                .reverse()
                                .map((item) => (
                                    <p
                                        className="text-sm"
                                        key={item.id}
                                    >
                                        {state.nodes.find((node) => node.id === item.nodeId)?.name ?? item.nodeId} ·{' '}
                                        {item.observedAt} · {item.frequency} MHz · {item.signal} dBm
                                    </p>
                                ))}
                        </div>
                    )}
                </SheetContent>
            </Sheet>
        </>
    );
}

export default Wireless;
