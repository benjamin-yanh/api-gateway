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
  BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS,
  balanceProtectionThreshold10KTokensSchema,
} from '../quota-settings-schema'

describe('balance protection threshold validation', () => {
  test('accepts integer values expressed in ten-thousand-token units', () => {
    assert.equal(balanceProtectionThreshold10KTokensSchema.parse('1'), 1)
    assert.equal(balanceProtectionThreshold10KTokensSchema.parse('100'), 100)
    assert.equal(
      balanceProtectionThreshold10KTokensSchema.parse(
        String(BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS)
      ),
      BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS
    )
  })

  test('rejects values that cannot be converted safely by the backend', () => {
    for (const value of [
      '',
      '0',
      '-1',
      '1.5',
      'not-a-number',
      String(BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS + 1),
    ]) {
      assert.equal(
        balanceProtectionThreshold10KTokensSchema.safeParse(value).success,
        false,
        `expected ${JSON.stringify(value)} to be rejected`
      )
    }
  })
})
