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
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { createInstance } from 'i18next'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { userSchema, type User } from '../../types'
import { useUsersColumns } from '../users-columns'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function CashbackTable(props: { users: User[] }) {
  const columns = useUsersColumns().filter(
    (column) =>
      'accessorKey' in column &&
      ['cashback_quota', 'cashback_history_quota'].includes(
        String(column.accessorKey)
      )
  )
  const table = useReactTable({
    data: props.users,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <table>
      <thead>
        {table.getHeaderGroups().map((group) => (
          <tr key={group.id}>
            {group.headers.map((header) => (
              <th key={header.id}>
                {flexRender(
                  header.column.columnDef.header,
                  header.getContext()
                )}
              </th>
            ))}
          </tr>
        ))}
      </thead>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <td key={cell.id}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

test('user table displays withdrawable and lifetime cashback in CNY with both locales and preserves tiny amounts', async () => {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({ lng: 'zh', resources: { en, zh } })
  const previous = useSystemConfigStore.getState().config
  useSystemConfigStore
    .getState()
    .setConfig({
      currency: {
        ...previous.currency,
        quotaDisplayType: 'CNY',
        quotaPerUnit: 500000,
        usdExchangeRate: 7,
      },
    })
  const users = [
    userSchema.parse({
      id: 1,
      username: 'earned',
      display_name: '',
      quota: 0,
      used_quota: 0,
      request_count: 0,
      group: 'default',
      status: 1,
      role: 1,
      cashback_quota: 1,
      cashback_history_quota: 4000000000,
    }),
    userSchema.parse({
      id: 2,
      username: 'zero',
      display_name: '',
      quota: 0,
      used_quota: 0,
      request_count: 0,
      group: 'default',
      status: 1,
      role: 1,
      cashback_quota: 0,
      cashback_history_quota: 0,
    }),
    userSchema.parse({
      id: 3,
      username: 'missing',
      display_name: '',
      quota: 0,
      used_quota: 0,
      request_count: 0,
      group: 'default',
      status: 1,
      role: 1,
    }),
  ]
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <CashbackTable users={users} />
        </I18nextProvider>
      )
    )
    assert.deepEqual(
      [...container.querySelectorAll('th')].map((cell) => cell.textContent),
      ['待提现返现', '累计返现']
    )
    const cells = [...container.querySelectorAll('td')]
    assert.match(cells[0].textContent ?? '', /0\.000014/)
    assert.match(cells[1].textContent ?? '', /56,?000/)
    assert.ok(
      cells[1]
        .querySelector('[title]')
        ?.getAttribute('title')
        ?.includes('不含待入账奖励')
    )
    assert.match(cells[2].textContent ?? '', /0/)
    assert.match(cells[3].textContent ?? '', /0/)
    assert.equal(cells[4].textContent, '-')
    assert.equal(cells[5].textContent, '-')
    await act(async () => {
      await i18n.changeLanguage('en')
    })
    assert.deepEqual(
      [...container.querySelectorAll('th')].map((cell) => cell.textContent),
      ['Withdrawable cashback', 'Lifetime cashback']
    )
    assert.ok(
      container
        .querySelector('td:nth-child(2) [title]')
        ?.getAttribute('title')
        ?.includes('Pending rewards are excluded.')
    )
  } finally {
    await act(async () => root.unmount())
    useSystemConfigStore.getState().setConfig(previous)
    container.remove()
  }
})
