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
import { GiftCardIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import dayjs from '@/lib/dayjs'

import { getRedemptionCardHistory } from '../api'
import { redemptionCardHistoryQueryKey } from '../query-keys'

export function RedemptionHistoryCard() {
  const { t } = useTranslation()
  const historyQuery = useQuery({
    queryKey: redemptionCardHistoryQueryKey,
    queryFn: getRedemptionCardHistory,
  })
  const history = historyQuery.data?.data ?? []

  let content = (
    <p className='text-muted-foreground py-8 text-center text-sm'>
      {t('No redemption records yet')}
    </p>
  )
  if (historyQuery.isPending) {
    content = (
      <div
        className='flex flex-col gap-3'
        aria-label={t('Loading redemption history')}
      >
        {['first', 'second', 'third'].map((key) => (
          <Skeleton key={key} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (historyQuery.isError) {
    content = (
      <p role='alert' className='text-destructive text-sm'>
        {t('Failed to load redemption history')}
      </p>
    )
  } else if (history.length > 0) {
    content = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Card type')}</TableHead>
            <TableHead>{t('Amount')}</TableHead>
            <TableHead className='text-right'>{t('Redeemed at')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {history.map((item) => (
            <TableRow key={item.id}>
              <TableCell>
                <Badge variant='secondary'>{item.group}</Badge>
              </TableCell>
              <TableCell className='font-medium'>¥{item.amount_rmb}</TableCell>
              <TableCell className='text-muted-foreground text-right'>
                {dayjs.unix(item.redeemed_time).format('YYYY-MM-DD HH:mm:ss')}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <div className='flex items-center gap-3'>
          <div className='bg-primary/15 text-primary flex size-10 items-center justify-center rounded-xl'>
            <HugeiconsIcon icon={GiftCardIcon} size={22} strokeWidth={1.8} />
          </div>
          <div>
            <CardTitle>{t('Redemption history')}</CardTitle>
            <CardDescription>
              {t('Your 10 most recent card redemptions')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>{content}</CardContent>
    </Card>
  )
}
