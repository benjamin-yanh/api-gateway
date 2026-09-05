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
import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useDeferredValue, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { getCashbackRules } from '@/features/usage-cashback/api'
import { useAuthStore } from '@/stores/auth-store'

import { ApiPricingTable } from './components/api-pricing-table'
import { usePricingData } from './hooks/use-pricing-data'
import {
  filterPricingModels,
  PRICING_CATEGORIES,
  type PricingCategory,
} from './lib/quote'

const CATEGORY_LABELS: Record<PricingCategory, string> = {
  all: 'All Models',
  text: 'Text',
  image: 'Image',
  audio: 'Audio',
  video: 'Video',
  embeddings: 'Embeddings',
  other: 'Other',
}

export function Pricing() {
  const { t } = useTranslation()
  const isLoggedIn = useAuthStore((state) => state.auth.user !== null)
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState<PricingCategory>('all')
  const deferredSearch = useDeferredValue(search)
  const { models, isLoading, error, priceRate, usdExchangeRate } =
    usePricingData()
  const cashback = useQuery({
    queryKey: ['cashback', 'rules'],
    queryFn: getCashbackRules,
    enabled: isLoggedIn,
    staleTime: 30_000,
    retry: false,
  })

  const filteredModels = useMemo(
    () => filterPricingModels(models, deferredSearch, category),
    [category, deferredSearch, models]
  )

  let pricingContent: ReactNode
  if (isLoading) {
    pricingContent = (
      <div className='space-y-3' aria-label={t('Loading')}>
        <Skeleton className='h-12 w-full' />
        <Skeleton className='h-64 w-full' />
      </div>
    )
  } else if (error) {
    pricingContent = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Failed to load pricing')}</AlertTitle>
        <AlertDescription>
          {t('Please refresh the page and try again.')}
        </AlertDescription>
      </Alert>
    )
  } else {
    pricingContent = (
      <ApiPricingTable
        models={filteredModels}
        priceRate={priceRate}
        usdExchangeRate={usdExchangeRate}
        cashbackRules={cashback.data}
        cashbackUnavailable={cashback.isError}
      />
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-7xl px-4 pt-20 pb-14 sm:px-6 sm:pt-24 lg:px-8'>
        <header className='mx-auto max-w-3xl text-center'>
          <p className='text-primary text-sm font-semibold tracking-wide'>
            {t('Simple and transparent billing')}
          </p>
          <h1 className='mt-3 text-4xl font-bold tracking-tight sm:text-5xl'>
            {t('API Pricing')}
          </h1>
          <p className='text-muted-foreground mx-auto mt-5 max-w-2xl text-base leading-7'>
            {t(
              'Compare model input and output prices, then choose the right API for your workload.'
            )}
          </p>
        </header>

        <main className='mt-12'>
          <section aria-labelledby='pricing-table-heading'>
            <div className='flex flex-col gap-5'>
              <div className='flex flex-col justify-between gap-4 lg:flex-row lg:items-end'>
                <div>
                  <h2
                    id='pricing-table-heading'
                    className='text-2xl font-semibold tracking-tight'
                  >
                    {t('Model pricing')}
                  </h2>
                  <p className='text-muted-foreground mt-1 text-sm'>
                    {t('{{count}} models available', {
                      count: filteredModels.length,
                    })}
                  </p>
                </div>

                <label className='relative block w-full lg:max-w-sm'>
                  <span className='sr-only'>{t('Search models')}</span>
                  <HugeiconsIcon
                    icon={Search01Icon}
                    aria-hidden='true'
                    className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2'
                    strokeWidth={2}
                  />
                  <Input
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder={t('Search by model or provider')}
                    className='h-11 pl-9'
                  />
                </label>
              </div>

              <div
                className='flex gap-2 overflow-x-auto pb-1'
                role='group'
                aria-label={t('Model category')}
              >
                {PRICING_CATEGORIES.map((item) => (
                  <Button
                    key={item}
                    type='button'
                    size='sm'
                    variant={category === item ? 'default' : 'outline'}
                    aria-pressed={category === item}
                    onClick={() => setCategory(item)}
                    className='shrink-0 rounded-full'
                  >
                    {t(CATEGORY_LABELS[item])}
                  </Button>
                ))}
              </div>

              {pricingContent}
            </div>
          </section>

          <Alert className='mt-8'>
            <AlertTitle>{t('Pricing notes')}</AlertTitle>
            <AlertDescription>
              {t(
                'Prices are displayed in CNY. Actual charges are based on API usage records and the active billing configuration.'
              )}
            </AlertDescription>
          </Alert>
        </main>
      </PageTransition>
    </PublicLayout>
  )
}
