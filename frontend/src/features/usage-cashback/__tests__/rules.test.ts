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

import {
  createCashbackSchema,
  percentToRatio,
  ratioToPercent,
} from '../lib/rules'

const i18n = createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

const valid = {
  enabled: true,
  input_per_million: '0.00000001',
  output_per_million: '0',
  global_enabled: true,
  cap_percent: '9.1234',
}

test('percentage conversion preserves the six-place ratio without floating point artifacts', () => {
  assert.equal(percentToRatio('9.1234'), '0.091234')
  assert.equal(ratioToPercent('0.091234'), '9.1234')
  assert.equal(percentToRatio('0.0001'), '0.000001')
  assert.equal(ratioToPercent('0.999999'), '99.9999')
  assert.equal(percentToRatio(''), '')
  assert.equal(percentToRatio('1e1'), '')
})

test('verified model accepts a positive eight-place amount and an explicit zero output rate', () => {
  assert.equal(
    createCashbackSchema(i18n.t, true).safeParse(valid).success,
    true
  )
})

test('enabled model rejects empty, negative, scientific, oversized and overly precise rates', () => {
  const schema = createCashbackSchema(i18n.t, true)
  for (const input_per_million of [
    '',
    '-1',
    '1e2',
    'NaN',
    'Infinity',
    '01',
    '00.1',
    '1000000.01',
    '0.000000001',
  ]) {
    assert.equal(
      schema.safeParse({ ...valid, input_per_million }).success,
      false,
      input_per_million
    )
  }
  assert.equal(
    schema.safeParse({ ...valid, input_per_million: '0' }).success,
    false
  )
})

test('unsupported models cannot be enabled but existing rates can be retained when disabled', () => {
  const schema = createCashbackSchema(i18n.t, false)
  assert.equal(schema.safeParse(valid).success, false)
  assert.equal(schema.safeParse({ ...valid, enabled: false }).success, true)
})

test('global cashback accepts an empty cap and validates a configured percentage', () => {
  const schema = createCashbackSchema(i18n.t, true)
  for (const cap_percent of ['0', '100', '100.0001', '-1', '1.12345', '1e1']) {
    assert.equal(
      schema.safeParse({ ...valid, cap_percent }).success,
      false,
      cap_percent
    )
  }
  assert.equal(
    schema.safeParse({ ...valid, global_enabled: true, cap_percent: '' })
      .success,
    true
  )
})
