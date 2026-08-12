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
import { describe, test } from 'node:test'

import {
  buildPreviewRows,
  createInitialLaneState,
  priceFromUSD,
  priceToUSD,
} from '../model-pricing-core'

const t = (key: string) => key

describe('model pricing RMB editor', () => {
  test('converts stored USD prices to and from RMB', () => {
    assert.equal(priceFromUSD('3', 7), '21')
    assert.equal(priceToUSD('21', 7), '3')

    const initial = createInitialLaneState(
      {
        name: 'test-model',
        ratio: '1.5',
        completionRatio: '2',
      },
      7
    )

    assert.equal(initial.promptPrice, '21')
    assert.equal(initial.prices.completion, '42')
  })

  test('removes floating-point drift from converted RMB prices', () => {
    assert.equal(priceFromUSD('9.999999999999571', 7), '70')
    assert.equal(priceFromUSD('0.000000001', 1), '1e-9')
  })

  test('uses the full-width RMB symbol in the preview', () => {
    const rows = buildPreviewRows(
      { name: 'test-model' },
      'per-token',
      '',
      '',
      '21',
      {
        completion: '42',
        cache: '',
        createCache: '',
        image: '',
        audioInput: '',
        audioOutput: '',
      },
      {
        completion: true,
        cache: false,
        createCache: false,
        image: false,
        audioInput: false,
        audioOutput: false,
      },
      t
    )

    assert.equal(rows[0].value, '￥21')
    assert.equal(rows[1].value, '￥42')
  })
})
