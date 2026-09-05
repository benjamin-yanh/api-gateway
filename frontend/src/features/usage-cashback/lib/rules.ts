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
import type { TFunction } from 'i18next'
import { z } from 'zod'

// Decimal strings remain exact at the API boundary, including percentage conversion.
export function ratioToPercent(ratio: string): string {
  if (!/^\d+(\.\d{1,6})?$/.test(ratio)) return ''
  const [whole, fraction = ''] = ratio.split('.')
  const scaled = BigInt(whole) * 1_000_000n + BigInt(fraction.padEnd(6, '0'))
  const tail = String(scaled % 10_000n)
    .padStart(4, '0')
    .replace(/0+$/, '')
  return `${scaled / 10_000n}${tail ? `.${tail}` : ''}`
}

export function percentToRatio(percent: string): string {
  if (!/^\d+(\.\d{1,4})?$/.test(percent)) return ''
  const [whole, fraction = ''] = percent.split('.')
  const scaled = BigInt(whole) * 10_000n + BigInt(fraction.padEnd(4, '0'))
  const tail = String(scaled % 1_000_000n)
    .padStart(6, '0')
    .replace(/0+$/, '')
  return `${scaled / 1_000_000n}${tail ? `.${tail}` : ''}`
}

export function createCashbackSchema(t: TFunction, supported: boolean) {
  const amount = z
    .string()
    .refine(
      (value) =>
        /^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$/.test(value) &&
        Number(value) <= 1_000_000,
      t('Enter an amount from 0 to 1,000,000 with up to 8 decimal places.')
    )
  return z
    .object({
      enabled: z.boolean(),
      input_per_million: amount,
      output_per_million: amount,
      global_enabled: z.boolean(),
      cap_percent: z.string(),
    })
    .superRefine((value, ctx) => {
      if (value.enabled && !supported) {
        ctx.addIssue({
          code: 'custom',
          path: ['enabled'],
          message: t('This model is not verified for usage cashback.'),
        })
      }
      if (
        value.enabled &&
        Number(value.input_per_million) === 0 &&
        Number(value.output_per_million) === 0
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['input_per_million'],
          message: t('Set at least one cashback amount above zero.'),
        })
      }
      const percent = value.cap_percent
      if (
        percent !== '' &&
        (!/^\d+(\.\d{1,4})?$/.test(percent) ||
          Number(percent) <= 0 ||
          Number(percent) >= 100)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['cap_percent'],
          message: t(
            'Enter a cashback cap above 0% and below 100%, with up to 4 decimal places.'
          ),
        })
      }
    })
}

export type CashbackFormValues = z.infer<
  ReturnType<typeof createCashbackSchema>
>
