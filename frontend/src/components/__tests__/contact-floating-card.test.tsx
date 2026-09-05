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
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
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
const { ContactFloatingCard } = await import('../contact-floating-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Contact us': 'Contact us',
        'Customer service QQ': 'Customer service QQ',
        'Telegram group': 'Telegram group',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  domWindow.close()
})

test('shows the customer service QQ and Telegram group', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ContactFloatingCard />
      </I18nextProvider>
    )
  })

  assert.match(container.textContent ?? '', /3138763753/)
  const telegramLink = container.querySelector('a')
  assert.equal(telegramLink?.href, 'https://t.me/gtongxue')
  assert.equal(telegramLink?.target, '_blank')
  assert.equal(telegramLink?.rel, 'noopener noreferrer')

  await act(async () => root.unmount())
  container.remove()
})

test('minimizing hides contact details and the contact button restores them without losing focus', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  try {
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ContactFloatingCard />
        </I18nextProvider>
      )
    })

    const toggle = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Minimize"]'
    )
    assert.ok(toggle, 'The contact window must offer a minimize button')
    assert.equal(toggle.getAttribute('aria-expanded'), 'true')
    toggle.focus()

    await act(async () => toggle.click())

    assert.doesNotMatch(container.textContent ?? '', /3138763753/)
    assert.equal(container.querySelector('a'), null)
    assert.equal(toggle.getAttribute('aria-expanded'), 'false')
    assert.equal(toggle.getAttribute('aria-label'), 'Contact us')
    assert.equal(document.activeElement, toggle)

    await act(async () => toggle.click())

    assert.match(container.textContent ?? '', /3138763753/)
    assert.equal(container.querySelector('a')?.href, 'https://t.me/gtongxue')
    assert.equal(toggle.getAttribute('aria-expanded'), 'true')
    assert.equal(document.activeElement, toggle)
  } finally {
    await act(async () => root.unmount())
    container.remove()
  }
})
