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

import { renderToStaticMarkup } from 'react-dom/server'

import { getPaymentIcon } from '../ui'

test('renders standard configured payment icons without waiting for icon packs', () => {
  const iconNames = ['SiAlipay', 'SiWechat', 'SiStripe', 'LuCreditCard']

  for (const iconName of iconNames) {
    const markup = renderToStaticMarkup(
      getPaymentIcon(undefined, 'payment-icon', iconName, iconName)
    )

    assert.match(markup, /^<svg/)
    assert.match(markup, /payment-icon/)
  }
})
