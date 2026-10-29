import { routes } from '@/lib/routes';

import { expect, test } from '../fixtures/test.ts';
import { wirelessCassette, wirelessSnapshot } from '../mocks/cassettes/wireless.ts';

test.use({ cassette: wirelessCassette() });

test('keeps same-SSID access points distinct and disables connection after disappearance', async ({ page, world }) => {
    await page.goto(routes.wirelessArea('lab'));
    await expect(page.getByRole('row', { name: /02:00:00:00:00:01/ })).toBeVisible();
    await expect(page.getByRole('row', { name: /02:00:00:00:00:02/ })).toBeVisible();
    await page.getByRole('row', { name: /02:00:00:00:00:01/ }).click();
    const connect = page.getByRole('button', { name: 'Connect using saved node profile' });
    await expect(connect).toBeEnabled();
    world.raiseFlag('radioLost');
    await expect(connect).toBeDisabled();
    await expect(page.getByRole('dialog')).toContainText('lost');
});

test('routes a verified client into the existing flow form with a pinned identity', async ({ page }) => {
    await page.goto(`${routes.wirelessArea('lab')}?view=contexts`);
    await page.getByRole('link', { name: 'Test client' }).click();
    await expect(page).toHaveURL(
        (url) =>
            url.pathname === routes.newFlow &&
            url.searchParams.get('scenario') === 'wireless' &&
            url.searchParams.get('context') === 'context-1' &&
            url.searchParams.get('endpoint') === 'client-1',
    );
    await expect(page.getByRole('tab', { name: 'Wireless network' })).toHaveAttribute('data-state', 'active');
    await expect(page.getByRole('combobox').filter({ hasText: '02:00:00:00:00:03' })).toContainText('revision 1');
});

test.describe('client address changes', () => {
    const initial = wirelessSnapshot();
    const changed = {
        ...initial,
        endpoints: initial.endpoints.map((endpoint) => ({ ...endpoint, ip: '192.168.50.21', revision: 2 })),
        revision: 2,
    };
    const cassette = wirelessCassette();
    test.use({
        cassette: {
            ...cassette,
            queries: {
                ...cassette.queries,
                wirelessState: [
                    { data: { wirelessState: initial } },
                    { data: { wirelessState: changed }, whenFlag: 'clientChanged' },
                ],
            },
            subscriptions: {
                wirelessUpdated: [
                    { frames: [{ payload: { data: { wirelessUpdated: changed } }, whenFlag: 'clientChanged' }] },
                ],
            },
        },
    });

    test('requires explicit acceptance of a changed client revision', async ({ page, world }) => {
        await page.goto(routes.newWirelessFlow('context-1', 'client-1'));
        await expect(page.getByRole('combobox').filter({ hasText: '02:00:00:00:00:03' })).toContainText('revision 1');
        world.raiseFlag('clientChanged');
        await expect(page.getByText('Binding needs review')).toBeVisible();
        await page.getByRole('button', { name: 'Use updated client 192.168.50.21 · revision 2' }).click();
        await expect(page.getByText('Binding needs review')).toHaveCount(0);
        await expect(page.getByRole('combobox').filter({ hasText: '02:00:00:00:00:03' })).toContainText('revision 2');
    });
});
