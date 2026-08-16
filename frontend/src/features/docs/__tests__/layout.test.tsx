/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
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

Object.defineProperty(globalThis, 'React', {
  configurable: true,
  value: await import('react'),
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { DocsSidebar, DocsTableOfContents } = await import('../docs-navigation')

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

test('renders separate section navigation and on-page navigation landmarks', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <DocsSidebar search='' />
        <DocsTableOfContents />
      </I18nextProvider>
    )
  })

  const sectionLandmarks = container.querySelectorAll(
    'nav[aria-label="Documentation sections"]'
  )
  const onPageLandmarks = container.querySelectorAll(
    'nav[aria-label="On this page"]'
  )
  assert.equal(sectionLandmarks.length, 2)
  assert.equal(onPageLandmarks.length, 1)
  assert.equal(sectionLandmarks.item(0).querySelectorAll('a').length, 5)
  assert.equal(sectionLandmarks.item(1).querySelectorAll('a').length, 5)
  assert.equal(onPageLandmarks.item(0).querySelectorAll('a').length, 4)

  await act(async () => root.unmount())
  container.remove()
})

test('filters documentation section links without hiding the article outline', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <DocsSidebar search='routing' />
        <DocsTableOfContents />
      </I18nextProvider>
    )
  })

  const sectionLandmarks = container.querySelectorAll(
    'nav[aria-label="Documentation sections"]'
  )
  for (const landmark of sectionLandmarks) {
    const sectionLinks = landmark.querySelectorAll('a')
    assert.equal(sectionLinks.length, 1)
    assert.equal(sectionLinks.item(0).getAttribute('href'), '#routing')
  }
  assert.equal(
    container.querySelectorAll('nav[aria-label="On this page"] a').length,
    4
  )

  await act(async () => root.unmount())
  container.remove()
})
