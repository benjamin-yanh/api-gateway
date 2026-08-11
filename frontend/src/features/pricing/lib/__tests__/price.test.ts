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

import type { PricingModel } from '../../types'
import { formatDynamicUnitPrice } from '../dynamic-price'
import { formatPrice, formatRequestPrice } from '../price'

const tokenModel: PricingModel = {
  id: 1,
  model_name: 'test-token-model',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 2,
  enable_groups: [],
}

const requestModel: PricingModel = {
  ...tokenModel,
  id: 2,
  model_name: 'test-request-model',
  quota_type: 1,
  model_price: 0.5,
}

describe('model pricing currency', () => {
  test('formats token and request prices as RMB', () => {
    assert.equal(formatPrice(tokenModel, 'input', 'M', false, 1, 7), 'RMB 14')
    assert.equal(formatRequestPrice(requestModel, false, 1, 7), 'RMB 3.5')
  })

  test('formats dynamic prices as RMB', () => {
    assert.equal(
      formatDynamicUnitPrice(2, {
        tokenUnit: 'M',
        usdExchangeRate: 7,
      }),
      'RMB 14'
    )
  })
})
