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

import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { CashbackRule } from '@/features/usage-cashback/components/cashback-rule'
import type { CashbackSettings } from '@/features/usage-cashback/types'
import { getLobeIcon } from '@/lib/lobe-icon'

import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import {
  formatPrice,
  formatRequestPrice,
  stripTrailingZeros,
} from '../lib/price'
import type { PricingModel } from '../types'

type ApiPricingTableProps = {
  models: PricingModel[]
  priceRate: number
  usdExchangeRate: number
  cashbackRules?: CashbackSettings
  cashbackUnavailable?: boolean
}

function PriceValue(props: { value: string; muted?: boolean }) {
  return (
    <span
      className={
        props.muted
          ? 'text-muted-foreground font-mono text-sm'
          : 'font-mono text-sm font-medium tabular-nums'
      }
    >
      {props.value}
    </span>
  )
}

export function ApiPricingTable(props: ApiPricingTableProps) {
  const { t } = useTranslation()

  if (props.models.length === 0) {
    return (
      <Empty className='min-h-72 rounded-xl border border-dashed'>
        <EmptyHeader>
          <EmptyTitle>{t('No models found')}</EmptyTitle>
          <EmptyDescription>
            {t('Try another search term or model category.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='bg-card overflow-hidden rounded-xl border shadow-sm'>
      <Table aria-label={t('API pricing table')}>
        <TableHeader className='bg-muted/50'>
          <TableRow className='hover:bg-transparent'>
            <TableHead className='min-w-64 px-5'>{t('Model name')}</TableHead>
            <TableHead className='min-w-36'>{t('Input price')}</TableHead>
            <TableHead className='min-w-36'>{t('Cached input')}</TableHead>
            <TableHead className='min-w-36'>{t('Output price')}</TableHead>
            <TableHead className='min-w-36 pr-5'>{t('Pricing unit')}</TableHead>
            <TableHead className='min-w-48 pr-5'>
              {t('Usage cashback')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.models.map((model) => {
            const iconKey = model.icon || model.vendor_icon
            const icon = iconKey ? getLobeIcon(iconKey, 26) : null
            const dynamicPricing = getDynamicPricingSummary(model, {
              tokenUnit: 'M',
              showRechargePrice: false,
              priceRate: props.priceRate,
              usdExchangeRate: props.usdExchangeRate,
              groupRatioMultiplier: getDynamicDisplayGroupRatio(model),
            })
            const isRequestPriced = model.quota_type === 1
            const inputPrice = isRequestPriced
              ? stripTrailingZeros(
                  formatRequestPrice(
                    model,
                    false,
                    props.priceRate,
                    props.usdExchangeRate
                  )
                )
              : stripTrailingZeros(
                  formatPrice(
                    model,
                    'input',
                    'M',
                    false,
                    props.priceRate,
                    props.usdExchangeRate
                  )
                )
            const outputPrice = isRequestPriced
              ? '—'
              : stripTrailingZeros(
                  formatPrice(
                    model,
                    'output',
                    'M',
                    false,
                    props.priceRate,
                    props.usdExchangeRate
                  )
                )
            const cachePrice =
              isRequestPriced || model.cache_ratio == null
                ? '—'
                : stripTrailingZeros(
                    formatPrice(
                      model,
                      'cache',
                      'M',
                      false,
                      props.priceRate,
                      props.usdExchangeRate
                    )
                  )

            return (
              <TableRow key={model.model_name}>
                <TableCell className='px-5 py-4 whitespace-normal'>
                  <div className='flex min-w-0 items-center gap-3'>
                    <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-lg'>
                      {icon ?? (
                        <span className='text-muted-foreground text-sm font-semibold'>
                          {model.model_name.slice(0, 1).toUpperCase()}
                        </span>
                      )}
                    </div>
                    <div className='min-w-0'>
                      <div className='truncate font-mono text-sm font-semibold'>
                        {model.model_name}
                      </div>
                      <div className='text-muted-foreground mt-0.5 truncate text-xs'>
                        {model.vendor_name || t('API model')}
                      </div>
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  {dynamicPricing ? (
                    <Badge variant='secondary'>{t('Dynamic Pricing')}</Badge>
                  ) : (
                    <PriceValue value={inputPrice} />
                  )}
                </TableCell>
                <TableCell>
                  <PriceValue
                    value={dynamicPricing ? '—' : cachePrice}
                    muted={cachePrice === '—'}
                  />
                </TableCell>
                <TableCell>
                  <PriceValue
                    value={dynamicPricing ? '—' : outputPrice}
                    muted={outputPrice === '—'}
                  />
                </TableCell>
                <TableCell className='text-muted-foreground pr-5 text-xs'>
                  {isRequestPriced ? t('per request') : t('per 1M tokens')}
                </TableCell>
                <TableCell className='pr-5'>
                  <CashbackRule
                    modelName={model.model_name}
                    rules={props.cashbackRules}
                    unavailable={props.cashbackUnavailable}
                  />
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
