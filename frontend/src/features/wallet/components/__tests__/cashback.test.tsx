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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider, notifyManager } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { CashbackCard } = await import('../cashback-card')
const { api } = await import('@/lib/api')
const { AxiosError } = await import('axios')
const { formatQuota } = await import('@/lib/format')

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true
notifyManager.setScheduler(queueMicrotask)
after(() => domWindow.close())

async function renderCashback(quota: number, loading = false) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  let refreshes = 0
  function WalletFixture() {
    const [cashback, setCashback] = useState(quota)
    return (
      <CashbackCard
        quota={cashback}
        loading={loading}
        onSuccess={async () => {
          refreshes++
          setCashback(0)
        }}
      />
    )
  }
  await act(async () =>
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <WalletFixture />
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
  return {
    container,
    refreshes: () => refreshes,
    async cleanup() {
      await act(async () => root.unmount())
      queryClient.clear()
      container.remove()
    },
  }
}

function button(label: string): HTMLButtonElement {
  const result = [
    ...document.querySelectorAll<HTMLButtonElement>('button'),
  ].find((element) => element.textContent === label)
  assert.ok(result, `Expected button: ${label}`)
  return result
}

test('withdrawal lists five methods and disables all external payment methods', async () => {
  const view = await renderCashback(500_000)
  try {
    assert.ok(view.container.textContent?.includes(formatQuota(500_000)))
    await act(async () => button('Withdraw').click())
    const radios = [...document.querySelectorAll<HTMLElement>('[role="radio"]')]
    assert.equal(radios.length, 5)
    for (const method of ['bank_card', 'alipay', 'wechat', 'usdt']) {
      const radio = document.querySelector<HTMLElement>(
        `[role="radio"][aria-labelledby="cashback-${method}-label"]`
      )
      assert.ok(radio)
      assert.equal(radio.getAttribute('aria-disabled'), 'true')
      assert.match(
        document.querySelector(`label[for="cashback-${method}"]`)
          ?.textContent ?? '',
        /Not yet available/
      )
      await act(async () => radio.click())
      assert.equal(radio.getAttribute('aria-checked'), 'false')
    }
    const balance = document.querySelector<HTMLElement>(
      '[role="radio"][aria-labelledby="cashback-balance-label"]'
    )
    assert.equal(balance?.getAttribute('aria-checked'), 'true')
    balance?.focus()
    assert.equal(document.activeElement, balance)
    assert.equal(button('Confirm withdrawal').disabled, false)
  } finally {
    await view.cleanup()
  }
})

test('zero cashback keeps methods visible but prevents confirmation', async () => {
  const view = await renderCashback(0)
  try {
    await act(async () => button('Withdraw').click())
    assert.ok(
      document.body.textContent?.includes('No cashback available to withdraw')
    )
    assert.equal(button('Confirm withdrawal').disabled, true)
    assert.equal(document.querySelectorAll('[role="radio"]').length, 5)
  } finally {
    await view.cleanup()
  }
})

test('loading cashback prevents opening a withdrawal with a stale amount', async () => {
  const view = await renderCashback(500_000, true)
  try {
    assert.equal(button('Withdraw').disabled, true)
  } finally {
    await view.cleanup()
  }
})

test('successful withdrawal submits once and refreshes the displayed cashback', async () => {
  const originalAdapter = api.defaults.adapter
  let finish: (() => void) | undefined
  let requests = 0
  api.defaults.adapter = async (config) => {
    requests++
    assert.equal(config.url, '/api/user/cashback/withdraw')
    assert.deepEqual(JSON.parse(config.data), {
      method: 'balance',
      quota: 500_000,
    })
    await new Promise<void>((resolve) => {
      finish = resolve
    })
    return {
      config,
      data: { success: true },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  const view = await renderCashback(500_000)
  try {
    await act(async () => button('Withdraw').click())
    await act(async () => {
      const confirm = button('Confirm withdrawal')
      confirm.click()
      confirm.click()
    })
    assert.equal(requests, 1)
    assert.equal(button('Processing...').disabled, true)
    assert.equal(button('Cancel').disabled, true)
    assert.ok(finish)
    await act(async () => finish?.())
    assert.equal(view.refreshes(), 1)
    assert.ok(view.container.textContent?.includes(formatQuota(0)))
    assert.equal(document.querySelector('[role="dialog"]'), null)
  } finally {
    api.defaults.adapter = originalAdapter
    await view.cleanup()
  }
})

test('failed withdrawal preserves cashback and shows a recoverable error', async () => {
  const originalAdapter = api.defaults.adapter
  api.defaults.adapter = async (config) => {
    throw new AxiosError('Conflict', 'ERR_BAD_REQUEST', config, undefined, {
      config,
      data: { success: false, code: 'cashback_balance_changed' },
      status: 409,
      statusText: 'Conflict',
      headers: {},
    })
  }
  const view = await renderCashback(500_000)
  try {
    await act(async () => button('Withdraw').click())
    await act(async () => button('Confirm withdrawal').click())
    assert.match(
      document.querySelector('[role="alert"]')?.textContent ?? '',
      /Cashback balance changed/
    )
    assert.equal(view.refreshes(), 0)
    assert.ok(view.container.textContent?.includes(formatQuota(500_000)))
    assert.equal(button('Confirm withdrawal').disabled, false)
  } finally {
    api.defaults.adapter = originalAdapter
    await view.cleanup()
  }
})
