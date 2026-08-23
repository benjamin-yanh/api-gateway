/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { RechargeCenterHero } = await import('../recharge-center-hero')
const { RechargeFormCard } = await import('../recharge-form-card')
const { RedemptionHistoryCard } = await import('../redemption-history-card')
const { redemptionCardHistoryQueryKey } = await import('../../query-keys')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  domWindow.close()
})

test('purchase button opens the configured card store in a separate tab', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <RechargeCenterHero user={null} loading={false} />
      </I18nextProvider>
    )
  })

  const link = container.querySelector<HTMLAnchorElement>(
    'a[href="https://www.kufaka.com/shop/GBCRMEYE"]'
  )
  assert.ok(link)
  assert.equal(link.target, '_blank')
  assert.match(link.rel, /noopener/)

  await act(async () => root.unmount())
  container.remove()
})

test('redemption history renders only the 10 records returned by the API', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(redemptionCardHistoryQueryKey, {
    success: true,
    data: Array.from({ length: 10 }, (_, index) => ({
      id: index + 1,
      group: index === 0 ? '3_RMB_CARD' : '10_RMB_CARD',
      amount_rmb: index === 0 ? 3 : 10,
      quota: 1000,
      redeemed_time: 1_700_000_000 + index,
    })),
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RedemptionHistoryCard />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  assert.equal(container.querySelectorAll('tbody tr').length, 10)
  assert.equal(
    container.querySelector('tbody tr')?.textContent?.includes('¥3'),
    true
  )

  await act(async () => root.unmount())
  container.remove()
  queryClient.clear()
})

test('redemption becomes the primary card when online topup is unavailable', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <RechargeFormCard
          topupInfo={{
            enable_online_topup: false,
            enable_stripe_topup: false,
            pay_methods: [],
            min_topup: 1,
            stripe_min_topup: 1,
            amount_options: [],
            discount: {},
            enable_redemption: true,
          }}
          presetAmounts={[]}
          selectedPreset={null}
          onSelectPreset={() => undefined}
          topupAmount={0}
          onTopupAmountChange={() => undefined}
          paymentAmount={0}
          calculating={false}
          onPaymentMethodSelect={() => undefined}
          paymentLoading={null}
          redemptionCode=''
          onRedemptionCodeChange={() => undefined}
          onRedeem={() => undefined}
          redeeming={false}
        />
      </I18nextProvider>
    )
  })

  assert.equal(container.textContent?.includes('Add Funds'), false)
  assert.equal(
    container.textContent?.includes('Online topup is not enabled.'),
    false
  )
  assert.equal(container.textContent?.includes('Have a Code?'), true)
  assert.ok(
    container.querySelector<HTMLInputElement>(
      'input[placeholder="Enter your redemption code"]'
    )
  )

  await act(async () => root.unmount())
  container.remove()
})
