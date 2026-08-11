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
import { Activity, KeyRound, Network, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { Skeleton } from '@/components/ui/skeleton'
import { isLikelyHtml } from '@/lib/content-format'

import { getAboutContent } from './api'

function EmptyAboutState() {
  const { t } = useTranslation()
  const currentYear = new Date().getFullYear()
  const capabilities = [
    {
      icon: Network,
      title: t('Unified API gateway'),
      description: t(
        'Connect applications to multiple AI providers through OpenAI-compatible endpoints.'
      ),
    },
    {
      icon: KeyRound,
      title: t('Access and billing controls'),
      description: t(
        'Manage API keys, quotas, routing, pricing, and usage records from one console.'
      ),
    },
    {
      icon: Activity,
      title: t('Operations visibility'),
      description: t(
        'Monitor channel health, request usage, errors, and service instances.'
      ),
    },
  ]

  return (
    <div className='mx-auto max-w-6xl space-y-10 px-4 py-12 sm:py-16'>
      <section className='space-y-4 text-center'>
        <div className='bg-primary/10 text-primary mx-auto flex size-14 items-center justify-center rounded-2xl'>
          <Network className='size-7' aria-hidden='true' />
        </div>
        <div className='mx-auto max-w-3xl space-y-3'>
          <h1 className='text-3xl font-bold tracking-tight sm:text-4xl'>
            {t('AI API Gateway')}
          </h1>
          <p className='text-muted-foreground text-base leading-7 sm:text-lg'>
            {t(
              'A privately operated AI API relay focused on stable access, unified authentication, routing, and usage management.'
            )}
          </p>
        </div>
      </section>

      <section className='grid gap-4 md:grid-cols-3'>
        {capabilities.map((capability) => (
          <article key={capability.title} className='rounded-xl border p-5'>
            <capability.icon
              className='text-primary mb-4 size-6'
              aria-hidden='true'
            />
            <h2 className='font-semibold'>{capability.title}</h2>
            <p className='text-muted-foreground mt-2 text-sm leading-6'>
              {capability.description}
            </p>
          </article>
        ))}
      </section>

      <section className='bg-muted/40 rounded-xl border p-6 sm:p-8'>
        <div className='flex items-start gap-4'>
          <ShieldCheck
            className='text-primary mt-0.5 size-7 shrink-0'
            aria-hidden='true'
          />
          <div className='space-y-3'>
            <h2 className='text-xl font-semibold'>
              {t('Privacy and security')}
            </h2>
            <p className='text-muted-foreground leading-7'>
              {t(
                'Requests are forwarded only to the AI provider selected by the configured routing policy. This service does not include third-party analytics, external update checks, GPU deployment, or built-in chat features.'
              )}
            </p>
            <p className='text-muted-foreground leading-7'>
              {t(
                'Do not submit secrets or regulated data unless the selected upstream provider and your operating policies permit it.'
              )}
            </p>
          </div>
        </div>
      </section>

      <footer className='border-t pt-6 text-center text-sm'>
        <p className='text-muted-foreground'>
          © {currentYear} {t('NewAPI')}
        </p>
      </footer>
    </div>
  )
}

export function About() {
  const { data, isLoading } = useQuery({
    queryKey: ['about-content'],
    queryFn: getAboutContent,
  })

  const rawContent = data?.data?.trim() ?? ''
  const hasContent = rawContent.length > 0
  const contentIsHtml = hasContent && isLikelyHtml(rawContent)

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto flex max-w-4xl flex-col gap-4 py-12'>
          <Skeleton className='h-8 w-[45%]' />
          <Skeleton className='h-4 w-full' />
          <Skeleton className='h-4 w-[90%]' />
          <Skeleton className='h-4 w-[80%]' />
        </div>
      </PublicLayout>
    )
  }

  if (!hasContent) {
    return (
      <PublicLayout>
        <EmptyAboutState />
      </PublicLayout>
    )
  }

  if (contentIsHtml) {
    return (
      <PublicLayout showMainContainer={false}>
        <RichContent
          mode='html'
          htmlVariant='isolated'
          content={rawContent}
          className='prose-neutral dark:prose-invert max-w-none'
        />
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <div className='mx-auto max-w-6xl px-4 py-8'>
        <RichContent
          mode='markdown'
          content={rawContent}
          className='prose-neutral dark:prose-invert max-w-none'
        />
      </div>
    </PublicLayout>
  )
}
