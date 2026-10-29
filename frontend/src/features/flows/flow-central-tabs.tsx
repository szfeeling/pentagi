import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import FlowDashboard from '@/features/flows/dashboard/flow-dashboard';
import FlowAssistantMessages from '@/features/flows/messages/flow-assistant-messages';
import FlowAutomationMessages from '@/features/flows/messages/flow-automation-messages';
import { FlowWireless, useWirelessFlowBinding } from '@/features/wireless/flow-wireless';
import { CENTRAL_TAB_VALUES, useFlowTabDetection, WIRELESS_CENTRAL_TAB_VALUES } from '@/hooks/use-flow-tab-detection';

function FlowCentralTabs() {
    const binding = useWirelessFlowBinding();
    const { handleTabChange, resolvedTab } = useFlowTabDetection(
        binding ? WIRELESS_CENTRAL_TAB_VALUES : CENTRAL_TAB_VALUES,
    );

    return (
        <Tabs
            activationMode="manual"
            className="flex size-full flex-col"
            onValueChange={handleTabChange}
            value={resolvedTab}
        >
            <div className="max-w-full">
                <ScrollArea className="w-full pb-3">
                    <TabsList className="flex w-fit">
                        <TabsTrigger value="automation">Automation</TabsTrigger>
                        <TabsTrigger value="assistant">Assistant</TabsTrigger>
                        <TabsTrigger value="dashboard">Dashboard</TabsTrigger>
                        {binding && <TabsTrigger value="wireless">Wireless</TabsTrigger>}
                    </TabsList>
                    <ScrollBar orientation="horizontal" />
                </ScrollArea>
            </div>

            {binding && (
                <TabsContent
                    className="mt-1 flex-1 overflow-auto pr-4"
                    value="wireless"
                >
                    <FlowWireless />
                </TabsContent>
            )}
            <TabsContent
                className="mt-1 flex-1 overflow-auto pr-4"
                value="automation"
            >
                <FlowAutomationMessages />
            </TabsContent>
            <TabsContent
                className="mt-1 flex-1 overflow-auto pr-4"
                value="assistant"
            >
                <FlowAssistantMessages />
            </TabsContent>
            <TabsContent
                className="mt-1 flex-1 overflow-auto pr-4"
                value="dashboard"
            >
                <FlowDashboard />
            </TabsContent>
        </Tabs>
    );
}

export default FlowCentralTabs;
