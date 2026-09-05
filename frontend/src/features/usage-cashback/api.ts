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
import { api } from '@/lib/api'

import type {
  CashbackRecordDetail,
  CashbackRecordsPage,
  CashbackSettings,
} from './types'

interface CashbackResponse<T> {
  success: boolean
  message?: string
  data: T
}

export async function getCashbackSettings(): Promise<CashbackSettings> {
  const response = await api.get<CashbackResponse<CashbackSettings>>(
    '/api/cashback/settings'
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function saveCashbackSettings(
  settings: CashbackSettings
): Promise<CashbackSettings> {
  const response = await api.put<CashbackResponse<CashbackSettings>>(
    '/api/cashback/settings',
    settings,
    { skipErrorHandler: true }
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function getCashbackRules(): Promise<CashbackSettings> {
  const response = await api.get<CashbackResponse<CashbackSettings>>(
    '/api/cashback/rules'
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function getCashbackRecords(
  page: number,
  requestId?: string,
  admin = false
): Promise<CashbackRecordsPage> {
  const response = await api.get<CashbackResponse<CashbackRecordsPage>>(
    admin ? '/api/cashback/records' : '/api/user/cashback/records',
    { params: { p: page, page_size: 10, request_id: requestId } }
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function getCashbackRecordDetail(
  id: string,
  admin = false
): Promise<CashbackRecordDetail> {
  const path = admin ? '/api/cashback/records' : '/api/user/cashback/records'
  const response = await api.get<CashbackResponse<CashbackRecordDetail>>(
    `${path}/${encodeURIComponent(id)}`
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}
