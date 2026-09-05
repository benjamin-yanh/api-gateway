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
import { test } from 'node:test'

import { createInstance } from 'i18next'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import { useAuthStore } from '@/stores/auth-store'

import { ApiPricingTable } from '../api-pricing-table'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

test('cashback column is hidden for guests, visible to normal users, and removed on logout with cached rules', async () => {
  const i18n = createInstance()
  await i18n
    .use(initReactI18next)
    .init({ lng: 'en', resources: { en: { translation: {} } } })
  useAuthStore.getState().auth.reset()
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ApiPricingTable
            models={[
              {
                id: 1,
                model_name: 'text-model',
                quota_type: 0,
                model_ratio: 1,
                completion_ratio: 2,
                enable_groups: [],
              },
            ]}
            priceRate={1}
            usdExchangeRate={7}
            cashbackRules={{
              version: 1,
              enabled: true,
              max_ratio: '0.2',
              models: {
                'text-model': {
                  enabled: true,
                  input_per_million: '1',
                  output_per_million: '2',
                },
              },
              supported_models: {
                'text-model': { supported: true, reason: '' },
              },
            }}
          />
        </I18nextProvider>
      )
    )
    assert.equal(container.querySelectorAll('th').length, 5)
    assert.equal(container.querySelectorAll('tbody td').length, 5)
    assert.ok(!container.textContent?.includes('Usage cashback'))

    await act(async () =>
      useAuthStore.getState().auth.setUser({ id: 2, username: 'user', role: 1 })
    )
    assert.equal(container.querySelectorAll('th').length, 6)
    assert.equal(container.querySelectorAll('tbody td').length, 6)
    assert.ok(
      container
        .querySelector('th:last-child')
        ?.textContent?.includes('Usage cashback')
    )
    assert.ok(
      container.textContent?.includes('Uncached input: 1 CNY / 1M tokens')
    )

    await act(async () => useAuthStore.getState().auth.reset())
    assert.equal(container.querySelectorAll('th').length, 5)
    assert.equal(container.querySelectorAll('tbody td').length, 5)
    assert.ok(!container.textContent?.includes('Usage cashback'))
    assert.ok(!container.textContent?.includes('Uncached input:'))
  } finally {
    await act(async () => root.unmount())
    useAuthStore.getState().auth.reset()
    container.remove()
  }
})
