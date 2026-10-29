import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';

import type { WirelessBindingInput } from '@/graphql/types';

import { AppHeader, AppHeaderContent, AppHeaderTitle } from '@/components/layouts/app/app-header';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Card, CardContent } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { FlowForm, type FlowFormValues } from '@/features/flows/flow-form';
import { leaseIsFresh } from '@/features/wireless/types';
import { WirelessFlowFields } from '@/features/wireless/wireless-flow-fields';
import { routes } from '@/lib/routes';
import { useFlows } from '@/providers/flows-provider';
import { useProviders } from '@/providers/providers-provider';
import { useSystemSettings } from '@/providers/system-settings-provider';
import { useWireless } from '@/providers/wireless-provider';

function NewFlow() {
    const navigate = useNavigate();
    const [params, setParams] = useSearchParams();
    const scenario = params.get('scenario') === 'wireless' ? 'wireless' : 'general';
    const { error: wirelessError, state: wirelessState } = useWireless();
    const [wirelessBinding, setWirelessBinding] = useState<null | WirelessBindingInput>(null);
    const initialContext = wirelessState?.contexts.find((item) => item.id === params.get('context'));
    const initialEndpoint = wirelessState?.endpoints.find(
        (item) => item.id === params.get('endpoint') && item.contextId === initialContext?.id,
    );

    if (!wirelessBinding && initialContext) {
        setWirelessBinding({
            contextId: initialContext.id,
            endpointId: initialEndpoint?.id,
            endpointRevision: initialEndpoint?.revision,
            epoch: initialContext.epoch,
        });
    }

    const boundContext = wirelessState?.contexts.find((item) => item.id === wirelessBinding?.contextId);
    const boundEndpoint = wirelessState?.endpoints.find((item) => item.id === wirelessBinding?.endpointId);
    const bindingValid =
        !!wirelessBinding &&
        !wirelessError &&
        boundContext?.status === 'ready' &&
        boundContext.epoch === wirelessBinding.epoch &&
        leaseIsFresh(boundContext.expiresAt) &&
        (!wirelessBinding.endpointId ||
            (boundEndpoint?.status === 'verified' &&
                boundEndpoint.revision === wirelessBinding.endpointRevision &&
                leaseIsFresh(boundEndpoint.expiresAt)));

    const { selectedProvider } = useProviders();
    const { createFlow, createFlowWithAssistant } = useFlows();
    const { settings } = useSystemSettings();

    const [isLoading, setIsLoading] = useState(false);
    const [flowType, setFlowType] = useState<'assistant' | 'automation'>('automation');

    const shouldUseAgents = useMemo(() => {
        return settings?.assistantUseAgents ?? false;
    }, [settings?.assistantUseAgents]);

    const handleSubmit = async (values: FlowFormValues) => {
        if (isLoading || (scenario === 'wireless' && !bindingValid)) {
            return false;
        }

        setIsLoading(true);

        try {
            const wireless = scenario === 'wireless' && wirelessBinding ? wirelessBinding : undefined;
            const flowId =
                flowType === 'automation'
                    ? await createFlow(values, wireless)
                    : await createFlowWithAssistant(values, wireless);

            if (!flowId) {
                return false;
            }

            navigate(routes.flow(flowId, { tab: flowType }));

            return true;
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <>
            <AppHeader>
                <AppHeaderContent>
                    <AppHeaderTitle>New flow</AppHeaderTitle>
                </AppHeaderContent>
            </AppHeader>
            <div className="flex min-h-[calc(100dvh-3rem)] items-center justify-center p-4">
                <Card className="w-full max-w-2xl">
                    <CardContent className="flex flex-col gap-4 pt-6">
                        <div className="flex flex-col gap-2 text-center">
                            <h2 className="text-2xl font-semibold">Create a new flow</h2>
                            <p className="text-muted-foreground">Describe what you would like PentAGI to test</p>
                        </div>
                        <Tabs
                            onValueChange={(value) =>
                                setParams((previous) => {
                                    const next = new URLSearchParams(previous);
                                    next.set('scenario', value);

                                    return next;
                                })
                            }
                            value={scenario}
                        >
                            <TabsList className="grid w-full grid-cols-2">
                                <TabsTrigger value="general">General network</TabsTrigger>
                                <TabsTrigger value="wireless">Wireless network</TabsTrigger>
                            </TabsList>
                        </Tabs>
                        {scenario === 'wireless' && (
                            <WirelessFlowFields
                                binding={wirelessBinding}
                                onChange={setWirelessBinding}
                            />
                        )}
                        {scenario === 'wireless' && wirelessBinding && !bindingValid && (
                            <Alert variant="destructive">
                                <AlertTitle>Binding needs review</AlertTitle>
                                <AlertDescription>
                                    Select a current network or client to accept its latest identity and revision.
                                </AlertDescription>
                            </Alert>
                        )}
                        <Tabs
                            onValueChange={(value) => setFlowType(value as 'assistant' | 'automation')}
                            value={flowType}
                        >
                            <TabsList className="grid w-full grid-cols-2">
                                <TabsTrigger
                                    disabled={isLoading}
                                    value="automation"
                                >
                                    Automation
                                </TabsTrigger>
                                <TabsTrigger
                                    disabled={isLoading}
                                    value="assistant"
                                >
                                    Assistant
                                </TabsTrigger>
                            </TabsList>
                        </Tabs>
                        <FlowForm
                            defaultValues={{
                                providerName: selectedProvider?.name ?? '',
                                useAgents: shouldUseAgents,
                            }}
                            isDisabled={scenario === 'wireless' && !bindingValid}
                            isSubmitting={isLoading}
                            onSubmit={handleSubmit}
                            placeholder={
                                !isLoading
                                    ? flowType === 'automation'
                                        ? 'Describe what you would like PentAGI to test...'
                                        : 'What would you like me to help you with?'
                                    : 'Creating a new flow...'
                            }
                            type={flowType}
                        />
                    </CardContent>
                </Card>
            </div>
        </>
    );
}

export default NewFlow;
