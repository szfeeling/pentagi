import type { ReactNode } from 'react';

import { useMutation, useQuery, useSubscription } from '@apollo/client/react';
import { createContext, use, useCallback, useEffect, useMemo } from 'react';
import { toast } from 'sonner';
import { z } from 'zod';

import { type WirelessState, wirelessStateSchema } from '@/features/wireless/types';
import { WirelessCommandDocument, WirelessStateDocument, WirelessUpdatedDocument } from '@/graphql/types';
import { useUser } from '@/providers/user-provider';

interface WirelessContextValue {
    canEdit: boolean;
    canView: boolean;
    command: (command: Record<string, unknown>) => Promise<null | { id: string; token?: string }>;
    error: null | string;
    isLoading: boolean;
    isSubmitting: boolean;
    refetch: () => Promise<unknown>;
    state: null | WirelessState;
}

const Context = createContext<null | WirelessContextValue>(null);
const resultSchema = z.object({ id: z.string(), token: z.string().optional() });

export function useWireless() {
    const context = use(Context);

    if (!context) {
        throw new Error('WirelessProvider is required');
    }

    return context;
}

export function WirelessProvider({ children }: { children: ReactNode }) {
    const { authInfo } = useUser();
    const permissions = authInfo?.privileges ?? [];
    const canEdit = permissions.includes('wireless.edit') || permissions.includes('wireless.admin');
    const canView = permissions.includes('wireless.view') || permissions.includes('wireless.admin');
    const query = useQuery(WirelessStateDocument, {
        fetchPolicy: 'cache-and-network',
        pollInterval: 10000,
        skip: !canView,
    });
    const [mutate, mutation] = useMutation(WirelessCommandDocument);
    const subscription = useSubscription(WirelessUpdatedDocument, {
        onData: ({ client, data }) => {
            const incoming = wirelessStateSchema.safeParse(data.data?.wirelessUpdated);
            const current = wirelessStateSchema.safeParse(
                client.readQuery({ query: WirelessStateDocument })?.wirelessState,
            );

            if (incoming.success && (!current.success || incoming.data.revision >= current.data.revision)) {
                client.writeQuery({
                    data: { wirelessState: data.data?.wirelessUpdated },
                    query: WirelessStateDocument,
                });
            }
        },
        skip: !canView,
    });
    const parsed = useMemo(() => wirelessStateSchema.safeParse(query.data?.wirelessState), [query.data]);
    const state = parsed.success ? parsed.data : null;
    const refetchQuery = query.refetch;
    const refetch = useCallback(() => refetchQuery(), [refetchQuery]);

    useEffect(() => {
        if (!canView) {
            return;
        }

        const catchUp = () => {
            void refetch().catch(() => undefined);
        };

        window.addEventListener('ws:connected', catchUp);

        return () => window.removeEventListener('ws:connected', catchUp);
    }, [canView, refetch]);

    const command = useCallback(
        async (payload: Record<string, unknown>) => {
            if (!canEdit || mutation.loading) {
                return null;
            }

            try {
                const response = await mutate({
                    variables: { command: { ...payload, expectedRevision: state?.revision } },
                });
                const result = resultSchema.parse(response.data?.wirelessCommand);
                await refetch().catch(() => undefined);

                return result;
            } catch (error) {
                toast.error('Wireless operation failed', {
                    description: error instanceof Error ? error.message : 'Refresh and try again',
                });

                return null;
            }
        },
        [canEdit, mutate, mutation.loading, refetch, state],
    );

    const error =
        query.error?.message ??
        subscription.error?.message ??
        (query.data && !parsed.success ? 'Invalid wireless response' : null);

    return (
        <Context
            value={{
                canEdit,
                canView,
                command,
                error,
                isLoading: query.loading && !state,
                isSubmitting: mutation.loading,
                refetch,
                state,
            }}
        >
            {children}
        </Context>
    );
}
