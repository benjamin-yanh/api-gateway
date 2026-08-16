/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { BookOpenTextIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

type DocsSidebarProps = {
  search: string
}

const navigationGroups = [
  {
    label: 'API calls',
    links: [
      { href: '#overview', label: 'Overview' },
      { href: '#getting-started', label: 'Getting started' },
      { href: '#models', label: 'Available models' },
      { href: '#protocols', label: 'Protocol endpoints' },
      { href: '#routing', label: 'Routing rules' },
    ],
  },
]

const tableOfContents = [
  { href: '#getting-started', label: 'Base URL and authentication' },
  { href: '#models', label: 'Available models' },
  { href: '#protocols', label: 'Protocol endpoints' },
  { href: '#routing', label: 'Routing rules' },
]

export function DocsSidebar(props: DocsSidebarProps) {
  const { t } = useTranslation()
  const normalizedSearch = props.search.trim().toLocaleLowerCase()
  const groups = navigationGroups
    .map((group) => ({
      ...group,
      links: group.links.filter((link) =>
        t(link.label).toLocaleLowerCase().includes(normalizedSearch)
      ),
    }))
    .filter((group) => group.links.length > 0)

  return (
    <>
      <details className='bg-muted/30 rounded-lg border p-4 xl:hidden'>
        <summary className='cursor-pointer text-sm font-semibold'>
          {t('Browse documentation')}
        </summary>
        <nav
          className='mt-4 border-t pt-3'
          aria-label={t('Documentation sections')}
        >
          {groups.map((group) => (
            <div key={group.label}>
              <p className='mb-2 text-sm font-semibold'>{t(group.label)}</p>
              <ul className='grid gap-1 sm:grid-cols-2'>
                {group.links.map((link) => (
                  <li key={link.href}>
                    <a
                      href={link.href}
                      className='text-muted-foreground hover:text-foreground block py-1.5 text-sm'
                    >
                      {t(link.label)}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
      </details>

      <aside className='hidden xl:block'>
        <nav
          className='sticky top-24 max-h-[calc(100vh-7rem)] overflow-y-auto pr-3'
          aria-label={t('Documentation sections')}
        >
          <div className='text-muted-foreground mb-8 flex items-center gap-2 text-sm'>
            <HugeiconsIcon
              icon={BookOpenTextIcon}
              className='size-4'
              strokeWidth={2}
              aria-hidden='true'
            />
            <span>{t('API documentation')}</span>
          </div>

          {groups.map((group) => (
            <div key={group.label} className='border-border border-t pt-4'>
              <p className='mb-2 px-3 text-sm font-semibold'>
                {t(group.label)}
              </p>
              <ul className='space-y-1'>
                {group.links.map((link) => (
                  <li key={link.href}>
                    <a
                      href={link.href}
                      className={cn(
                        'text-muted-foreground hover:bg-muted hover:text-foreground block rounded-md px-3 py-2 text-sm transition-colors',
                        link.href === '#overview' &&
                          'bg-primary/8 text-primary font-medium'
                      )}
                    >
                      {t(link.label)}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}

          {groups.length === 0 && (
            <p className='text-muted-foreground px-3 text-sm'>
              {t('No documentation sections found.')}
            </p>
          )}
        </nav>
      </aside>
    </>
  )
}

export function DocsTableOfContents() {
  const { t } = useTranslation()

  return (
    <aside className='hidden xl:block'>
      <nav
        className='sticky top-24 border-l pl-5'
        aria-label={t('On this page')}
      >
        <p className='mb-3 text-sm font-semibold'>{t('On this page')}</p>
        <ul className='space-y-2'>
          {tableOfContents.map((item) => (
            <li key={item.href}>
              <a
                href={item.href}
                className='text-muted-foreground hover:text-primary block text-sm leading-5 transition-colors'
              >
                {t(item.label)}
              </a>
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  )
}
