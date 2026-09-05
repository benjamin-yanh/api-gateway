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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'

import { getCashbackRecords } from '../api'
import { ratioToPercent } from '../lib/rules'
import type { CashbackRecord } from '../types'
import { CashbackRefunds } from './cashback-refunds'

export function CashbackRecordsButton() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant='outline' onClick={() => setOpen(true)}>
        {t('Cashback records')}
      </Button>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Cashback records')}
        description={t(
          'Usage rewards are separate from transfers to your API balance.'
        )}
        contentClassName='sm:max-w-2xl'
      >
        {open && <CashbackRecords />}
      </Dialog>
    </>
  )
}

export function CashbackRecords(
  props: { requestId?: string; admin?: boolean } = {}
) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: [
      'cashback',
      'records',
      props.admin ?? false,
      props.requestId ?? '',
      page,
    ],
    queryFn: () => getCashbackRecords(page, props.requestId, props.admin),
    retry: false,
  })
  if (query.isPending) {
    return (
      <div aria-label={t('Loading cashback records')}>
        <Skeleton className='h-28 w-full' />
      </div>
    )
  }
  if (query.isError || !query.data) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t('Unable to load cashback records.')}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }
  if (query.data.items.length === 0) {
    return (
      <Empty className='border p-4'>
        <EmptyHeader>
          <EmptyTitle>{t('No cashback records')}</EmptyTitle>
          <EmptyDescription>
            {t(
              'Only recorded usage rewards appear here. An absent record does not confirm eligibility.'
            )}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='grid gap-3'>
      {query.data.items.map((record) => (
        <CashbackRecordDetails
          key={record.id}
          record={record}
          admin={props.admin}
        />
      ))}
      {!props.requestId && (
        <div className='flex items-center justify-between gap-2'>
          <Button
            variant='outline'
            disabled={page === 1}
            onClick={() => setPage((value) => value - 1)}
          >
            {t('Previous')}
          </Button>
          <span className='text-muted-foreground text-xs'>
            {t('Page {{page}}', { page })}
          </span>
          <Button
            variant='outline'
            disabled={page * 10 >= query.data.total}
            onClick={() => setPage((value) => value + 1)}
          >
            {t('Next')}
          </Button>
        </div>
      )}
      <Button
        variant='ghost'
        onClick={() => void query.refetch()}
        disabled={query.isFetching}
      >
        {t('Refresh')}
      </Button>
    </div>
  )
}

export function CashbackRecordDetails(props: {
  record: CashbackRecord
  admin?: boolean
}) {
  const { t } = useTranslation()
  const [refundsOpen, setRefundsOpen] = useState(false)
  const record = props.record
  let status = t('Cashback under review')
  if (record.status === 'processing') status = t('Awaiting usage settlement')
  if (record.status === 'pending') status = t('Cashback pending credit')
  if (record.status === 'credited') status = t('Cashback credited')
  if (record.status === 'not_eligible') status = t('Not eligible for cashback')
  if (record.status === 'reversed') status = t('Cashback fully reversed')
  const unconfirmed =
    record.status === 'processing' || record.status === 'pending_review'
  const amounts = [
    {
      label: t('Original cashback entitlement'),
      amount: record.original_quota,
      unconfirmed,
    },
    { label: t('Cashback credited'), amount: record.credited_quota },
    { label: t('Cancelled before credit'), amount: record.cancelled_quota },
    { label: t('Recovered after credit'), amount: record.recovered_quota },
  ]
  return (
    <article className='min-w-0 rounded-lg border p-3 text-sm'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <span className='font-mono break-all'>{record.model_name}</span>
        <Badge variant='secondary'>{status}</Badge>
      </div>
      <p className='text-muted-foreground mt-1 text-xs'>
        {formatTimestamp(record.created_time)}
      </p>
      <dl className='mt-3 grid gap-2'>
        {amounts.map((item) => (
          <div
            key={item.label}
            className='flex flex-wrap justify-between gap-2'
          >
            <dt className='text-muted-foreground'>{item.label}</dt>
            <dd className='font-mono tabular-nums'>
              {item.unconfirmed ? (
                t('Not confirmed')
              ) : (
                <>
                  {formatQuotaWithCurrency(item.amount, {
                    digitsLarge: 8,
                    digitsSmall: 8,
                    abbreviate: false,
                  })}{' '}
                  <span className='text-muted-foreground text-xs'>
                    ({t('{{count}} quota units', { count: item.amount })})
                  </span>
                </>
              )}
            </dd>
          </div>
        ))}
      </dl>
      {record.reason && <CashbackReason reason={record.reason} />}
      {record.capped && record.reason !== 'capped' && (
        <p className='text-muted-foreground mt-3 text-xs'>
          {t('The cashback cap was applied.')}
        </p>
      )}
      {record.request_id && (
        <p className='text-muted-foreground mt-3 text-xs break-all'>
          {t('Request ID')}:{' '}
          <span className='font-mono'>{record.request_id}</span>
        </p>
      )}
      {record.rule && (
        <details className='mt-3 text-xs'>
          <summary className='cursor-pointer'>
            {t('Cashback rules for this request')}
          </summary>
          <div className='text-muted-foreground mt-2 grid gap-2'>
            <p>
              {t('Uncached input: {{amount}} CNY / 1M tokens', {
                amount: record.rule.input_per_million,
              })}
            </p>
            <p>
              {t('Output: {{amount}} CNY / 1M tokens', {
                amount: record.rule.output_per_million,
              })}
            </p>
            <p>
              {record.rule.max_ratio
                ? t(
                    'Cashback is capped at {{percent}}% of the actual charge.',
                    { percent: ratioToPercent(record.rule.max_ratio) }
                  )
                : t(
                    'No percentage cap. Cashback is calculated from actual eligible usage.'
                  )}
            </p>
          </div>
        </details>
      )}
      {record.refunded_quota > 0 && (
        <div className='mt-3'>
          <Button
            variant='outline'
            size='sm'
            aria-expanded={refundsOpen}
            onClick={() => setRefundsOpen((value) => !value)}
          >
            {t('Refund breakdown')}
          </Button>
          {refundsOpen && (
            <CashbackRefunds id={record.id} admin={props.admin} />
          )}
        </div>
      )}
    </article>
  )
}

function CashbackReason(props: { reason: string }) {
  const { t } = useTranslation()
  let message = t(
    'This reward requires review. Its final amount has not been confirmed.'
  )
  switch (props.reason) {
    case 'capped':
      message = t('The cashback cap was applied.')
      break
    case 'disabled':
      message = t('Cashback was not active when this request started.')
      break
    case 'subscription':
      message = t('Subscription usage does not qualify for cashback.')
      break
    case 'estimated_usage':
    case 'unknown_usage':
      message = t('Estimated or unknown usage does not qualify for cashback.')
      break
    case 'zero_charge':
      message = t('There was no settled balance charge for this request.')
      break
    case 'unsupported_usage':
      message = t('This usage is outside the supported text cashback scope.')
      break
    case 'below_minimum':
      message = t(
        'Cashback is below one quota unit. Fractions are not carried over.'
      )
      break
    case 'balance_limit':
      message = t(
        'Your cashback is pending because the balance limit was reached.'
      )
      break
  }
  return <p className='text-muted-foreground mt-3 text-xs'>{message}</p>
}
