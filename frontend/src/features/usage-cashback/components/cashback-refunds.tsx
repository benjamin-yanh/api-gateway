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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'

import { getCashbackRecordDetail } from '../api'

export function CashbackRefunds(props: { id: string; admin?: boolean }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['cashback', 'detail', props.admin ?? false, props.id],
    queryFn: () => getCashbackRecordDetail(props.id, props.admin),
    retry: false,
  })
  if (query.isPending) return <Skeleton className='mt-3 h-20 w-full' />
  if (query.isError || !query.data) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t('Unable to load refund details.')}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }
  if (query.data.refunds.length === 0) {
    return (
      <p className='text-muted-foreground mt-3 text-xs'>
        {t('No refund entries available.')}
      </p>
    )
  }
  return (
    <div className='mt-3 grid gap-3'>
      {query.data.refunds.map((refund) => {
        const rows = [
          { label: t('Original refund'), quota: refund.quota },
          {
            label: t('Cancelled before credit'),
            quota: refund.cancelled_quota,
          },
          {
            label: t('Deducted from cashback balance'),
            quota: refund.cashback_debited,
          },
          { label: t('Withheld from refund'), quota: refund.refund_withheld },
          {
            label: t('Net refund to API balance'),
            quota: refund.wallet_credited,
          },
        ]
        return (
          <div key={refund.id} className='rounded border p-3'>
            <p className='text-muted-foreground text-xs'>
              {formatTimestamp(refund.created_time)}
            </p>
            <dl className='mt-2 grid gap-2'>
              {rows.map((row) => (
                <div
                  key={row.label}
                  className='flex flex-wrap justify-between gap-2 text-xs'
                >
                  <dt>{row.label}</dt>
                  <dd className='font-mono tabular-nums'>
                    {formatQuotaWithCurrency(row.quota, {
                      digitsLarge: 8,
                      digitsSmall: 8,
                      abbreviate: false,
                    })}{' '}
                    ({t('{{count}} quota units', { count: row.quota })})
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        )
      })}
    </div>
  )
}
