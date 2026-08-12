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

import { registerFormSchema } from '../constants.ts'

const validRegistration = {
  username: 'user@example.com',
  password: 'password123',
  confirmPassword: 'password123',
}

describe('registration email validation', () => {
  test('accepts an email address as the account username', () => {
    const result = registerFormSchema.safeParse(validRegistration)

    assert.equal(result.success, true)
  })

  test('rejects a non-email username with a user-facing message', () => {
    const result = registerFormSchema.safeParse({
      ...validRegistration,
      username: 'ordinary-username',
    })

    assert.equal(result.success, false)
    assert.equal(
      result.error?.issues[0]?.message,
      'Please enter a valid email address'
    )
  })

  test('rejects an email longer than 128 characters with the length hint', () => {
    const result = registerFormSchema.safeParse({
      ...validRegistration,
      username: `${'a'.repeat(117)}@example.com`,
    })

    assert.equal(result.success, false)
    assert.equal(
      result.error?.issues[0]?.message,
      'Email must be at most 128 characters long'
    )
  })
})
