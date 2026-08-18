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
import {
  ArrowUpRight01Icon,
  ShoppingBag01Icon,
  WalletCardsIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import type { UserWalletData } from '../types'

const CARD_STORE_URL = 'https://www.kufaka.com/shop/GBCRMEYE'

interface RechargeCenterHeroProps {
  user: UserWalletData | null
  loading: boolean
}

export function RechargeCenterHero(props: RechargeCenterHeroProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-primary text-primary-foreground overflow-hidden rounded-2xl border shadow-sm'>
      <div className='grid gap-6 p-6 sm:p-8 lg:grid-cols-[1fr_auto] lg:items-center'>
        <div className='flex min-w-0 items-center gap-4 sm:gap-5'>
          <div className='bg-background/45 flex size-14 shrink-0 items-center justify-center rounded-2xl sm:size-16'>
            <HugeiconsIcon icon={WalletCardsIcon} size={30} strokeWidth={1.8} />
          </div>
          <div className='min-w-0'>
            <p className='text-sm font-medium opacity-75'>
              {t('Current Balance')}
            </p>
            {props.loading ? (
              <Skeleton className='bg-background/50 mt-2 h-10 w-40' />
            ) : (
              <p className='mt-1 font-mono text-3xl font-bold tracking-tight tabular-nums sm:text-4xl'>
                {formatQuota(props.user?.quota ?? 0)}
              </p>
            )}
            <p className='mt-2 text-sm opacity-70'>
              {t('Purchase a card and redeem it instantly below')}
            </p>
          </div>
        </div>

        <Button
          size='lg'
          variant='secondary'
          className='w-full lg:w-auto'
          render={
            <a
              href={CARD_STORE_URL}
              target='_blank'
              rel='noopener noreferrer'
            />
          }
        >
          <HugeiconsIcon icon={ShoppingBag01Icon} data-icon='inline-start' />
          {t('Buy recharge card')}
          <HugeiconsIcon icon={ArrowUpRight01Icon} data-icon='inline-end' />
        </Button>
      </div>
    </section>
  )
}
