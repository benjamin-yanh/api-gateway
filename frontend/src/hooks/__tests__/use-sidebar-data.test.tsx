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
Object.defineProperty(globalThis, 'window', {
  configurable: true,
  value: domWindow,
})
Object.defineProperty(globalThis, 'document', {
  configurable: true,
  value: domWindow.document,
})
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: domWindow.navigator,
})

const { act, createElement } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { useSidebarData } = await import('../use-sidebar-data')

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

test('personal sidebar exposes separate recharge and card redemption entries', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  function SidebarProbe() {
    const personal = useSidebarData().navGroups.find(
      (group) => group.id === 'personal'
    )
    return createElement('pre', null, JSON.stringify(personal?.items))
  }

  await act(async () => {
    root.render(
      createElement(
        I18nextProvider,
        { i18n },
        createElement(SidebarProbe)
      )
    )
  })

  const items = JSON.parse(container.textContent || '[]') as Array<{
    title: string
    url: string
  }>
  assert.deepEqual(
    items.map(({ title, url }) => ({ title, url })),
    [
      { title: 'Recharge Center', url: '/wallet' },
      { title: 'Redeem Card', url: '/wallet#redemption-code' },
      { title: 'Profile', url: '/profile' },
    ]
  )

  await act(async () => root.unmount())
  container.remove()
})
