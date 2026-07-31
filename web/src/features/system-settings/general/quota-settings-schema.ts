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
import * as z from 'zod'

export const BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS = 922_337_203_685_477

export const balanceProtectionThreshold10KTokensSchema = z.coerce
  .number()
  .int()
  .min(1)
  .max(BALANCE_PROTECTION_THRESHOLD_MAX_10K_TOKENS)
