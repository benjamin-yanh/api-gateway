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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
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
  'PointerEvent',
  'MouseEvent',
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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { ModelsProvider } = await import('../models-provider')
const { VendorsPage } = await import('../vendors-page')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiGet = (
  url: string,
  config?: unknown
) => Promise<{ data: Record<string, unknown> }>
type MockableApi = { get: ApiGet }

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
let renderedRoot: ReturnType<typeof createRoot> | null = null
let renderedHost: HTMLDivElement | null = null

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  if (condition()) return

  await new Promise<void>((resolve, reject) => {
    const intervalId = setInterval(() => {
      if (!condition()) return
      clearTimeout(timeoutId)
      clearInterval(intervalId)
      resolve()
    }, 10)
    const timeoutId = setTimeout(() => {
      clearInterval(intervalId)
      if (condition()) {
        resolve()
        return
      }
      reject(new Error(`${failureMessage}: ${document.body.textContent}`))
    }, 1500)
  })
}

describe('vendor management page', () => {
  afterEach(async () => {
    if (renderedRoot) await act(async () => renderedRoot?.unmount())
    renderedHost?.remove()
    renderedRoot = null
    renderedHost = null
    apiClient.get = originalGet
  })

  after(() => {
    domWindow.close()
  })

  test('lists vendors and filters them by the search field', async () => {
    apiClient.get = async (url) => {
      assert.equal(url, '/api/vendors/')
      return {
        data: {
          success: true,
          data: {
            items: [
              {
                id: 1,
                name: 'OpenAI',
                description: 'General models',
                icon: 'OpenAI',
                status: 1,
                created_time: 1,
                updated_time: 1,
              },
              {
                id: 2,
                name: 'Anthropic',
                description: 'Claude models',
                icon: 'Claude',
                status: 1,
                created_time: 1,
                updated_time: 1,
              },
            ],
            total: 2,
            page: 1,
            page_size: 1000,
          },
        },
      }
    }

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderedHost = document.createElement('div')
    document.body.append(renderedHost)
    renderedRoot = createRoot(renderedHost)

    await act(async () =>
      renderedRoot?.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelsProvider>
              <VendorsPage />
            </ModelsProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    )

    await act(async () =>
      waitForCondition(
        () => renderedHost?.textContent?.includes('Anthropic') === true,
        'Vendor rows did not load'
      )
    )
    assert.match(renderedHost.textContent || '', /OpenAI/)
    const table = renderedHost.querySelector('table')
    assert.ok(table)
    assert.ok(table.closest('.overflow-auto'))

    const searchInput = renderedHost.querySelector<HTMLInputElement>(
      'input[aria-label="Search vendors..."]'
    )
    assert.ok(searchInput)
    await act(async () => {
      const valueSetter = Object.getOwnPropertyDescriptor(
        domWindow.HTMLInputElement.prototype,
        'value'
      )?.set
      assert.ok(valueSetter)
      valueSetter.call(searchInput, 'claude')
      searchInput.dispatchEvent(
        new domWindow.Event('input', { bubbles: true }) as unknown as Event
      )
    })

    assert.match(renderedHost.textContent || '', /Anthropic/)
    assert.doesNotMatch(renderedHost.textContent || '', /OpenAI/)
  })
})
