/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

import { getAvailableModels } from './api'

export function ModelCatalog() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const {
    data: models = [],
    isLoading,
    isError,
  } = useQuery({
    queryKey: ['public-models'],
    queryFn: getAvailableModels,
    staleTime: 60 * 1000,
  })
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const filteredModels = normalizedSearch
    ? models.filter((model) =>
        model.id.toLocaleLowerCase().includes(normalizedSearch)
      )
    : models

  return (
    <div className='space-y-4'>
      <div className='relative max-w-md'>
        <HugeiconsIcon
          icon={Search01Icon}
          className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2'
          strokeWidth={2}
          aria-hidden='true'
        />
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search available models')}
          aria-label={t('Search available models')}
          className='pl-9'
        />
      </div>

      {isLoading && (
        <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
          {Array.from({ length: 9 }, (_, index) => (
            <Skeleton key={index} className='h-9 rounded-lg' />
          ))}
        </div>
      )}

      {isError && (
        <p className='text-destructive border-destructive/30 bg-destructive/5 rounded-lg border p-4 text-sm'>
          {t(
            'The model list is temporarily unavailable. Please try again later.'
          )}
        </p>
      )}

      {!isLoading && !isError && (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('{{count}} models available', { count: filteredModels.length })}
          </p>
          {filteredModels.length > 0 ? (
            <div className='flex flex-wrap gap-2'>
              {filteredModels.map((model) => (
                <Badge
                  key={model.id}
                  variant='outline'
                  className='h-auto max-w-full rounded-md px-2.5 py-1 font-mono font-normal break-all whitespace-normal'
                >
                  {model.id}
                </Badge>
              ))}
            </div>
          ) : (
            <p className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
              {t('No matching models found.')}
            </p>
          )}
        </>
      )}
    </div>
  )
}
