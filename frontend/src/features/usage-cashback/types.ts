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
export interface CashbackModelRule {
  enabled: boolean
  input_per_million: string
  output_per_million: string
}

export interface CashbackSettings {
  version: number
  enabled: boolean
  max_ratio: string
  models: Record<string, CashbackModelRule>
  supported_models: Record<string, { supported: boolean; reason: string }>
}

export interface CashbackRecord {
  id: string
  request_id: string
  model_name: string
  status: string
  reason: string
  capped: boolean
  original_quota: number
  credited_quota: number
  cancelled_quota: number
  recovered_quota: number
  actual_quota: number
  refunded_quota: number
  input_tokens: number
  output_tokens: number
  created_time: number
  updated_time: number
  rule?: {
    input_per_million: string
    output_per_million: string
    max_ratio: string
  }
}

export interface CashbackRecordsPage {
  items: CashbackRecord[]
  total: number
}

export interface CashbackRecordDetail {
  record: CashbackRecord
  refunds: Array<{
    id: string
    quota: number
    cancelled_quota: number
    recovered_quota: number
    cashback_debited: number
    refund_withheld: number
    wallet_credited: number
    created_time: number
  }>
}
