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
import test from 'node:test'

import { formatAccessLogJSON, tokenizeAccessLogJSON } from '../lib/format.ts'

test('formats stored JSON for readable access-log details', () => {
  assert.equal(
    formatAccessLogJSON('{"model":"gpt-test","stream":false}'),
    '{\n  "model": "gpt-test",\n  "stream": false\n}'
  )
})

test('preserves malformed legacy values instead of hiding details', () => {
  assert.equal(formatAccessLogJSON('{not-json'), '{not-json')
})

test('preserves a complete SSE transcript for stream response details', () => {
  const stream = [
    'data: {"type":"message_start"}',
    '',
    'data: {"type":"content_block_delta","delta":{"text":"hello"}}',
    '',
    'data: [DONE]',
    '',
  ].join('\n')

  assert.equal(formatAccessLogJSON(stream), stream)
})

test('tokenizes formatted JSON fields and values for syntax highlighting', () => {
  assert.deepEqual(tokenizeAccessLogJSON('{"ok":true,"count":12}'), [
    { offset: 0, type: 'punctuation', value: '{' },
    { offset: 1, type: 'key', value: '"ok"' },
    { offset: 5, type: 'punctuation', value: ':' },
    { offset: 6, type: 'boolean', value: 'true' },
    { offset: 10, type: 'punctuation', value: ',' },
    { offset: 11, type: 'key', value: '"count"' },
    { offset: 18, type: 'punctuation', value: ':' },
    { offset: 19, type: 'number', value: '12' },
    { offset: 21, type: 'punctuation', value: '}' },
  ])
})

test('preserves SSE prefixes while highlighting embedded JSON', () => {
  const tokens = tokenizeAccessLogJSON(
    'data: {"delta":{"text":"hello"}}\n\ndata: [DONE]'
  )

  assert.equal(
    tokens.map((token) => token.value).join(''),
    'data: {"delta":{"text":"hello"}}\n\ndata: [DONE]'
  )
  assert.ok(
    tokens.some((token) => token.type === 'key' && token.value === '"text"')
  )
  assert.ok(
    tokens.some((token) => token.type === 'string' && token.value === '"hello"')
  )
})
