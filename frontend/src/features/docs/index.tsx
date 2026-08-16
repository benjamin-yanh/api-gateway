/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Input } from '@/components/ui/input'

import { DocsArticle } from './docs-article'
import { DocsSidebar, DocsTableOfContents } from './docs-navigation'
import { useActiveDocSection } from './use-active-doc-section'

export function Docs() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const activeSection = useActiveDocSection()
  const baseUrl =
    typeof window === 'undefined'
      ? 'http://101.132.177.78'
      : window.location.origin.replace(/\/$/, '')

  return (
    <PublicLayout showMainContainer={false}>
      <div className='mx-auto grid w-full max-w-[90rem] gap-8 px-4 pt-24 pb-20 sm:px-6 xl:grid-cols-[14rem_minmax(0,48rem)_12rem] xl:gap-9'>
        <DocsSidebar search={search} activeSection={activeSection} />

        <main className='min-w-0'>
          <div className='relative mb-8'>
            <HugeiconsIcon
              icon={Search01Icon}
              className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2'
              strokeWidth={2}
              aria-hidden='true'
            />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('Search documentation')}
              aria-label={t('Search documentation')}
              className='h-10 rounded-none pl-9 shadow-none'
            />
          </div>

          <DocsArticle baseUrl={baseUrl} />
        </main>

        <DocsTableOfContents activeSection={activeSection} />
      </div>
    </PublicLayout>
  )
}
