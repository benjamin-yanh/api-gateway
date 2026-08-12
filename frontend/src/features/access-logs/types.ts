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
export interface AccessLogListItem {
  id: number
  created_at: number
  request_id: string
  user_id: number
  username: string
  method: string
  url: string
  route: string
  status: number
  latency_ms: number
  response_size: number
  ip: string
  node_name: string
  body_size: number
  body_omitted: boolean
}

export interface AccessLogDetail extends AccessLogListItem {
  headers: string
  body?: string
}

export interface AccessLogListResponse {
  success: boolean
  message?: string
  data?: {
    items: AccessLogListItem[]
    total: number
    page: number
    page_size: number
  }
}

export interface AccessLogDetailResponse {
  success: boolean
  message?: string
  data?: AccessLogDetail
}

export interface GetAccessLogsParams {
  p: number
  page_size: number
  url?: string
}
