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
  buildNativeAppDeniedRedirect,
  parseNativeAppLoopbackRedirect,
} from '../native-app-callback.ts'

describe('native app callback validation', () => {
  test('accepts an unprivileged IPv4 loopback callback', () => {
    const callback = parseNativeAppLoopbackRedirect(
      'http://127.0.0.1:49152/callback'
    )
    assert.equal(callback?.host, '127.0.0.1:49152')
  })

  test('rejects a remote or lookalike callback host', () => {
    assert.equal(
      parseNativeAppLoopbackRedirect('https://example.com/callback'),
      null
    )
    assert.equal(
      parseNativeAppLoopbackRedirect(
        'http://localhost.example.com:49152/callback'
      ),
      null
    )
  })

  test('returns cancellation state without exposing a credential', () => {
    assert.equal(
      buildNativeAppDeniedRedirect(
        'http://127.0.0.1:49152/callback',
        'request-state-1234'
      ),
      'http://127.0.0.1:49152/callback?error=access_denied&state=request-state-1234'
    )
  })
})
