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
export function formatAccessLogJSON(value: string | undefined): string {
  if (!value) return ''
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

export type AccessLogJSONToken = {
  offset: number
  type:
    | 'boolean'
    | 'key'
    | 'null'
    | 'number'
    | 'plain'
    | 'punctuation'
    | 'string'
  value: string
}

const JSON_NUMBER_PATTERN = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/

/**
 * Tokenizes JSON and JSON fragments embedded in SSE without changing the
 * stored transcript. Unknown text is preserved as plain text, so malformed
 * legacy records remain fully visible.
 */
export function tokenizeAccessLogJSON(value: string): AccessLogJSONToken[] {
  const tokens: AccessLogJSONToken[] = []
  let index = 0

  while (index < value.length) {
    const character = value[index]
    if (/\s/.test(character)) {
      const start = index
      while (index < value.length && /\s/.test(value[index])) index++
      tokens.push({
        offset: start,
        type: 'plain',
        value: value.slice(start, index),
      })
      continue
    }

    if (character === '"') {
      const start = index++
      let escaped = false
      while (index < value.length) {
        const current = value[index++]
        if (escaped) {
          escaped = false
          continue
        }
        if (current === '\\') {
          escaped = true
          continue
        }
        if (current === '"') break
      }
      let lookahead = index
      while (lookahead < value.length && /\s/.test(value[lookahead])) {
        lookahead++
      }
      tokens.push({
        offset: start,
        type: value[lookahead] === ':' ? 'key' : 'string',
        value: value.slice(start, index),
      })
      continue
    }

    const number = value.slice(index).match(JSON_NUMBER_PATTERN)?.[0]
    if (number) {
      tokens.push({ offset: index, type: 'number', value: number })
      index += number.length
      continue
    }

    const keyword = ['true', 'false', 'null'].find((candidate) =>
      value.startsWith(candidate, index)
    )
    if (keyword) {
      tokens.push({
        offset: index,
        type: keyword === 'null' ? 'null' : 'boolean',
        value: keyword,
      })
      index += keyword.length
      continue
    }

    if ('{}[],:'.includes(character)) {
      tokens.push({ offset: index, type: 'punctuation', value: character })
      index++
      continue
    }

    const start = index++
    while (
      index < value.length &&
      !/\s/.test(value[index]) &&
      !'"{}[],:'.includes(value[index]) &&
      !JSON_NUMBER_PATTERN.test(value.slice(index)) &&
      !['true', 'false', 'null'].some((candidate) =>
        value.startsWith(candidate, index)
      )
    ) {
      index++
    }
    tokens.push({
      offset: start,
      type: 'plain',
      value: value.slice(start, index),
    })
  }

  return tokens
}
