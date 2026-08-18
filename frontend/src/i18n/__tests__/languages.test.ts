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

import {
  convertDetectedLanguage,
  INTERFACE_LANGUAGE_OPTIONS,
  normalizeInterfaceLanguage,
  toIntlLocale,
} from '../languages'

test('offers only English and Simplified Chinese', () => {
  assert.deepEqual(
    INTERFACE_LANGUAGE_OPTIONS.map((language) => language.code),
    ['zhCN', 'en']
  )
})

test('maps every Chinese locale variant to Simplified Chinese', () => {
  assert.equal(convertDetectedLanguage('zh-TW'), 'zhCN')
  assert.equal(convertDetectedLanguage('zh-Hant-HK'), 'zhCN')
  assert.equal(normalizeInterfaceLanguage('zhTW'), 'zhCN')
  assert.equal(toIntlLocale('zhCN'), 'zh-CN')
})

test('falls back removed language preferences to English', () => {
  assert.equal(normalizeInterfaceLanguage('fr'), 'en')
  assert.equal(normalizeInterfaceLanguage('ja'), 'en')
})
