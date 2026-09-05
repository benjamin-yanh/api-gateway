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

const domWindow =
  (globalThis.window as unknown as Window | undefined) ?? new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider, notifyManager } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ModelCashbackSettings } =
  await import('../components/model-cashback-settings')
const { CashbackRule } = await import('../components/cashback-rule')
const { CashbackRecords, CashbackRecordDetails } =
  await import('../components/cashback-records')
const { api } = await import('@/lib/api')
const { useAuthStore } = await import('@/stores/auth-store')
const { AxiosError } = await import('axios')
const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
;(
  globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true
notifyManager.setScheduler(queueMicrotask)
after(() => domWindow.close())

const settings = {
  version: 2,
  enabled: true,
  max_ratio: '0.123456',
  models: {
    'text-model': {
      enabled: true,
      input_per_million: '1.00000001',
      output_per_million: '0',
    },
    'other-model': {
      enabled: true,
      input_per_million: '2',
      output_per_million: '3',
    },
  },
  supported_models: { 'text-model': { supported: true, reason: '' } },
}
const record = {
  id: '7289b877-a44e-4dad-b615-c54ff394a0cf',
  request_id: 'usage-123',
  model_name: 'text-model',
  status: 'credited',
  reason: '',
  capped: false,
  actual_quota: 100,
  original_quota: 1,
  credited_quota: 1,
  cancelled_quota: 0,
  recovered_quota: 0,
  refunded_quota: 0,
  input_tokens: 200,
  output_tokens: 30,
  created_time: 100,
  updated_time: 100,
}

async function renderCashbackUI(element: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'root', email: '', role: 100 })
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>{element}</I18nextProvider>
      </QueryClientProvider>
    )
  )
  return {
    container,
    async cleanup() {
      await act(async () => root.unmount())
      client.clear()
      useAuthStore.getState().auth.setUser(null)
      container.remove()
    },
  }
}

function button(label: string): HTMLButtonElement {
  const found = [
    ...document.querySelectorAll<HTMLButtonElement>('button'),
  ].find((item) => item.textContent === label)
  assert.ok(found, label)
  return found
}

test('saving model cashback preserves other models and decimal strings with the current version', async () => {
  const previous = api.defaults.adapter
  let payload: unknown
  api.defaults.adapter = async (config) => {
    if (config.method === 'put') payload = JSON.parse(config.data)
    return {
      config,
      data: { success: true, data: settings },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  const view = await renderCashbackUI(
    <ModelCashbackSettings modelName='text-model' tokenPriced />
  )
  try {
    assert.ok(view.container.textContent?.includes('12.3456%'))
    await act(async () => button('Save cashback rules').click())
    assert.deepEqual(payload, settings)
  } finally {
    await view.cleanup()
    api.defaults.adapter = previous
  }
})

test('settings conflict prevents overwriting newer rules until an explicit reload', async () => {
  const previous = api.defaults.adapter
  let puts = 0
  api.defaults.adapter = async (config) => {
    if (config.method === 'put') {
      puts++
      throw new AxiosError('Conflict', 'ERR_BAD_REQUEST', config, undefined, {
        config,
        data: { code: 'cashback_settings_conflict' },
        status: 409,
        statusText: 'Conflict',
        headers: {},
      })
    }
    return {
      config,
      data: { success: true, data: settings },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  const view = await renderCashbackUI(
    <ModelCashbackSettings modelName='text-model' tokenPriced />
  )
  try {
    await act(async () => button('Save cashback rules').click())
    assert.match(
      view.container.querySelector('[role="alert"]')?.textContent ?? '',
      /Reload the current rules/
    )
    assert.equal(button('Save cashback rules').disabled, true)
    assert.equal(puts, 1)
    assert.ok(button('Reload current cashback rules'))
  } finally {
    await view.cleanup()
    api.defaults.adapter = previous
  }
})

test('unverified models cannot be enabled and disabled amount fields retain zero', async () => {
  const previous = api.defaults.adapter
  api.defaults.adapter = async (config) => ({
    config,
    data: { success: true, data: settings },
    status: 200,
    statusText: 'OK',
    headers: {},
  })
  const view = await renderCashbackUI(
    <ModelCashbackSettings modelName='unsupported-model' tokenPriced />
  )
  try {
    const toggle = view.container.querySelector<HTMLElement>('[role="switch"]')
    assert.ok(toggle)
    assert.equal(toggle.getAttribute('aria-checked'), 'false')
    assert.equal(toggle.getAttribute('aria-disabled'), 'true')
    const amount = view.container.querySelector<HTMLInputElement>(
      'input[inputmode="decimal"]'
    )
    assert.equal(amount?.disabled, true)
    assert.equal(amount?.value, '0')
  } finally {
    await view.cleanup()
    api.defaults.adapter = previous
  }
})

test('public active rules disclose caps and exclusions and inactive rules do not advertise rates', async () => {
  const active = await renderCashbackUI(
    <CashbackRule modelName='text-model' rules={settings} />
  )
  try {
    assert.ok(
      active.container.textContent?.includes('1.00000001 CNY / 1M tokens')
    )
    assert.ok(active.container.textContent?.includes('12.3456%'))
    assert.ok(
      active.container.textContent?.includes('estimated or unknown usage')
    )
    const summary = active.container.querySelector('summary')
    assert.ok(summary)
    await act(async () => summary.click())
    assert.equal(active.container.querySelector('details')?.open, true)
  } finally {
    await active.cleanup()
  }
  const inactive = await renderCashbackUI(
    <CashbackRule
      modelName='text-model'
      rules={{ ...settings, enabled: false }}
    />
  )
  try {
    assert.equal(inactive.container.textContent, 'Cashback is not active')
  } finally {
    await inactive.cleanup()
  }
})

test('pending review does not present an unconfirmed entitlement as zero', async () => {
  const view = await renderCashbackUI(
    <CashbackRecordDetails
      record={{
        ...record,
        status: 'pending_review',
        original_quota: 0,
        credited_quota: 0,
        reason: 'invalid_usage',
      }}
    />
  )
  try {
    assert.ok(view.container.textContent?.includes('Cashback under review'))
    assert.ok(view.container.textContent?.includes('Not confirmed'))
    assert.ok(
      view.container.textContent?.includes(
        'final amount has not been confirmed'
      )
    )
  } finally {
    await view.cleanup()
  }
})

test('small cashback and cancellation amounts remain visible in exact internal units', async () => {
  const view = await renderCashbackUI(
    <CashbackRecordDetails
      record={{ ...record, cancelled_quota: 2, recovered_quota: 3 }}
    />
  )
  try {
    for (const text of [
      '1 quota units',
      'Cancelled before credit',
      '2 quota units',
      'Recovered after credit',
      '3 quota units',
    ]) {
      assert.ok(view.container.textContent?.includes(text), text)
    }
  } finally {
    await view.cleanup()
  }
})

test('request-linked records use the exact filter and keep missing records distinct from ineligibility', async () => {
  const previous = api.defaults.adapter
  let params: unknown
  api.defaults.adapter = async (config) => {
    params = config.params
    assert.equal(config.url, '/api/user/cashback/records')
    return {
      config,
      data: { success: true, data: { items: [], total: 0 } },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  const view = await renderCashbackUI(<CashbackRecords requestId='usage-123' />)
  try {
    assert.deepEqual(params, { p: 1, page_size: 10, request_id: 'usage-123' })
    assert.ok(view.container.textContent?.includes('No cashback records'))
    assert.ok(
      view.container.textContent?.includes('does not confirm eligibility')
    )
  } finally {
    await view.cleanup()
    api.defaults.adapter = previous
  }
})

test('refund expansion uses the admin detail endpoint and explains the net refund', async () => {
  const previous = api.defaults.adapter
  api.defaults.adapter = async (config) => {
    assert.equal(
      config.url,
      '/api/cashback/records/7289b877-a44e-4dad-b615-c54ff394a0cf'
    )
    return {
      config,
      data: {
        success: true,
        data: {
          record,
          refunds: [
            {
              id: 'refund-1',
              quota: 100,
              cancelled_quota: 0,
              recovered_quota: 10,
              cashback_debited: 4,
              refund_withheld: 6,
              wallet_credited: 94,
              created_time: 200,
            },
          ],
        },
      },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  const view = await renderCashbackUI(
    <CashbackRecordDetails record={{ ...record, refunded_quota: 100 }} admin />
  )
  try {
    const expand = button('Refund breakdown')
    assert.equal(expand.getAttribute('aria-expanded'), 'false')
    await act(async () => expand.click())
    assert.equal(expand.getAttribute('aria-expanded'), 'true')
    for (const text of [
      'Withheld from refund',
      '6 quota units',
      'Net refund to API balance',
      '94 quota units',
    ]) {
      assert.ok(view.container.textContent?.includes(text), text)
    }
  } finally {
    await view.cleanup()
    api.defaults.adapter = previous
  }
})

test('uncapped active rules show the rates and explicitly disclose no percentage cap', async () => {
  const view = await renderCashbackUI(
    <CashbackRule
      modelName='text-model'
      rules={{ ...settings, max_ratio: '' }}
    />
  )
  try {
    assert.ok(
      view.container.textContent?.includes('1.00000001 CNY / 1M tokens')
    )
    assert.ok(view.container.textContent?.includes('No percentage cap.'))
    assert.equal(view.container.textContent?.includes('capped at'), false)
  } finally {
    await view.cleanup()
  }
})
