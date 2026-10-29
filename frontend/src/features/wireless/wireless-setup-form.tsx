import { useState } from 'react';
import { z } from 'zod';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useAppForm } from '@/hooks/use-app-form';
import { copyToClipboard } from '@/lib/clipboard';
import { useWireless } from '@/providers/wireless-provider';

const schema = z.object({ bssids: z.string(), cidrs: z.string(), name: z.string().trim().min(1).max(128) });
type Values = z.infer<typeof schema>;
const split = (value: string) => value.split(/[\s,]+/).filter(Boolean);

export function WirelessSetupForm({ areaId, onComplete }: { areaId?: string; onComplete: () => void }) {
    const { command, isSubmitting } = useWireless();
    const [enrollment, setEnrollment] = useState<null | { id: string; token?: string }>(null);
    const form = useAppForm<Values>({ defaultValues: { bssids: '', cidrs: '', name: '' }, schema });

    const submit = async (values: Values) => {
        const result = await command(
            areaId
                ? { areaId, kind: 'node.create', name: values.name }
                : { bssids: split(values.bssids), cidrs: split(values.cidrs), kind: 'area.create', name: values.name },
        );

        if (!result) {
            return;
        }

        if (result.token) {
            setEnrollment(result);
        } else {
            onComplete();
        }
    };

    if (enrollment) {
        return (
            <div className="space-y-4">
                <Alert>
                    <AlertTitle>Save the enrollment token</AlertTitle>
                    <AlertDescription>The token is shown once. Store it in the Linux node token file.</AlertDescription>
                </Alert>
                <p className="text-sm break-all">Node ID: {enrollment.id}</p>
                <code className="bg-muted block rounded p-3 text-sm break-all">{enrollment.token}</code>
                <Button onClick={() => void copyToClipboard(enrollment.token ?? '')}>Copy token</Button>
                <Button
                    onClick={onComplete}
                    variant="outline"
                >
                    Done
                </Button>
            </div>
        );
    }

    return (
        <Form {...form}>
            <form
                className="space-y-4"
                onSubmit={form.handleSubmit(submit)}
            >
                <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Name</FormLabel>
                            <FormControl>
                                <Input
                                    {...field}
                                    autoComplete="off"
                                />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                {!areaId && (
                    <>
                        <FormField
                            control={form.control}
                            name="bssids"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Authorized BSSIDs</FormLabel>
                                    <FormControl>
                                        <Input
                                            {...field}
                                            placeholder="aa:bb:cc:dd:ee:02, aa:bb:cc:dd:ee:04"
                                        />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <FormField
                            control={form.control}
                            name="cidrs"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>Authorized client subnets</FormLabel>
                                    <FormControl>
                                        <Input
                                            {...field}
                                            placeholder="192.168.50.0/24"
                                        />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <p className="text-muted-foreground text-sm">
                            Empty scope allows observation only. BSSIDs define which access points may be connected;
                            subnets define which discovered clients may be selected.
                        </p>
                    </>
                )}
                <Button
                    disabled={isSubmitting}
                    type="submit"
                >
                    {isSubmitting ? 'Saving…' : areaId ? 'Enroll node' : 'Create area'}
                </Button>
            </form>
        </Form>
    );
}
