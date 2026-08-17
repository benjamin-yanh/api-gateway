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

import type { PricingModel } from '../../types'

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
  'customElements',
  'MutationObserver',
  'ResizeObserver',
  'matchMedia',
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

Object.defineProperty(globalThis, 'React', {
  configurable: true,
  value: await import('react'),
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ApiPricingTable } = await import('../api-pricing-table')

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

test('renders a horizontally scrollable semantic price table', async () => {
  const model: PricingModel = {
    id: 1,
    model_name: 'gpt-example',
    vendor_name: 'Example AI',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: 0.5,
    enable_groups: [],
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ApiPricingTable models={[model]} priceRate={1} usdExchangeRate={7} />
      </I18nextProvider>
    )
  })

  const table = container.querySelector('table[aria-label="API pricing table"]')
  assert.ok(table)
  assert.ok(table.closest('[data-slot="table-container"]'))
  assert.equal(table.querySelectorAll('thead th').length, 5)
  assert.match(table.textContent || '', /gpt-example/)
  assert.match(table.textContent || '', /per 1M tokens/)

  await act(async () => root.unmount())
  container.remove()
})
