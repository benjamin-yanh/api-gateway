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
import { useTranslation } from 'react-i18next'

import { ratioToPercent } from '../lib/rules'
import type { CashbackSettings } from '../types'

export function CashbackRule(props: {
  modelName: string
  rules?: CashbackSettings
  unavailable?: boolean
}) {
  const { t } = useTranslation()
  const rule = props.rules?.models[props.modelName]
  if (props.unavailable) {
    return (
      <span className='text-muted-foreground text-xs'>
        {t('Cashback rules unavailable')}
      </span>
    )
  }
  if (!props.rules) {
    return (
      <span className='text-muted-foreground text-xs'>{t('Loading...')}</span>
    )
  }
  if (
    !props.rules.enabled ||
    !rule?.enabled ||
    props.rules.supported_models?.[props.modelName]?.supported !== true
  ) {
    return (
      <span className='text-muted-foreground text-xs'>
        {t('Cashback is not active')}
      </span>
    )
  }

  return (
    <details className='max-w-sm text-xs'>
      <summary className='cursor-pointer font-medium'>
        {t('Usage cashback')}
      </summary>
      <div className='mt-2 grid gap-2 whitespace-normal'>
        <p>
          {t('Uncached input: {{amount}} CNY / 1M tokens', {
            amount: rule.input_per_million,
          })}
        </p>
        <p>
          {t('Output: {{amount}} CNY / 1M tokens', {
            amount: rule.output_per_million,
          })}
        </p>
        <p>
          {props.rules.max_ratio
            ? t('Cashback is capped at {{percent}}% of the actual charge.', {
                percent: ratioToPercent(props.rules.max_ratio),
              })
            : t(
                'No percentage cap. Cashback is calculated from actual eligible usage.'
              )}
        </p>
        <p className='text-muted-foreground'>
          {t(
            'Only settled balance charges with verified upstream text usage qualify. Cache reads, cache writes, estimated or unknown usage, and subscription usage are excluded.'
          )}
        </p>
        <p className='text-muted-foreground'>
          {t(
            'Amounts are prorated below one million tokens. Zero means no cashback. Rewards are credited after settlement and can be transferred to your API balance.'
          )}
        </p>
        <p className='text-muted-foreground'>
          {t(
            'Gifted balance and cashback transferred to your API balance also qualify. Fractions below one quota unit are not carried over between requests.'
          )}
        </p>
      </div>
    </details>
  )
}
