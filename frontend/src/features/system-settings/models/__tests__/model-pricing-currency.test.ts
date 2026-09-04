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
    assert.equal(priceFromUSD('1.342400000004', 1), '1.3424')
    assert.equal(priceFromUSD('1.342399999996', 1), '1.3424')
    assert.equal(priceFromUSD('0.000000000001', 1), '1e-12')
    assert.equal(priceFromUSD('1.34240001', 1), '1.34240001')
  })

  test('preserves precision when saving and reopening RMB prices', () => {
    assert.equal(priceToUSD('1.3424', 7.3), '0.18389041095890413')
    assert.equal(priceFromUSD(priceToUSD('1.3424', 7.3), 7.3), '1.3424')
    const initial = createInitialLaneState(
      {
        name: 'test-model',
        ratio: '0.09194520547972603',
        cacheRatio: '0.1',
        completionRatio: '6',
      },
      7.3
    )
    assert.equal(initial.promptPrice, '1.3424')
    assert.equal(initial.prices.cache, '0.13424')
    assert.equal(initial.prices.completion, '8.0544')
  })

  test('normalizes saved prices across all token lanes and the preview', () => {
    const initial = createInitialLaneState(
      {
        name: 'gpt-5.6-terra',
        ratio: '0.919452054794',
        completionRatio: '6',
        cacheRatio: '0.1',
        createCacheRatio: '1.25',
        imageRatio: '1.25',
        audioRatio: '1.25',
        audioCompletionRatio: '4.8',
      },
      7.3
    )

    assert.equal(initial.promptPrice, '13.424')
    assert.deepEqual(initial.prices, {
      completion: '80.544',
      cache: '1.3424',
      createCache: '16.78',
      image: '16.78',
      audioInput: '16.78',
      audioOutput: '80.544',
    })
    const rows = buildPreviewRows(
      { name: 'gpt-5.6-terra' },
      'per-token',
      '',
      '',
      initial.promptPrice,
      initial.prices,
      initial.enabled,
      t
    )
    assert.deepEqual(
      rows.map((row) => row.value),
      [
        '￥13.424',
        '￥80.544',
        '￥1.3424',
        '￥16.78',
        '￥16.78',
        '￥16.78',
        '￥80.544',
      ]
    )
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
