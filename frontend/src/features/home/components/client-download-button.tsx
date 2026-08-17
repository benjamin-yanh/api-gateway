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
  AiChat01Icon,
  AiComputerIcon,
  AiProgrammingIcon,
  Download01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import { CLIENT_DOWNLOADS } from '../constants'

export function ClientDownloadButton() {
  const { t } = useTranslation()
  const clientDetails = {
    claude: {
      name: 'Claude',
      description: t(
        'For focused Claude conversations, coding, and knowledge work.'
      ),
      icon: AiChat01Icon,
    },
    chatgpt: {
      name: 'ChatGPT',
      description: t(
        'For ChatGPT conversations, writing, analysis, and everyday productivity.'
      ),
      icon: AiComputerIcon,
    },
    'deepseek-harness': {
      name: 'DeepSeek Harness',
      description: t(
        'For DeepSeek coding workflows and agent-based development.'
      ),
      icon: AiProgrammingIcon,
    },
  } satisfies Record<
    (typeof CLIENT_DOWNLOADS)[number]['id'],
    {
      name: string
      description: string
      icon: typeof AiChat01Icon
    }
  >

  return (
    <div className='grid w-full grid-cols-1 gap-3 sm:grid-cols-3' role='list'>
      {CLIENT_DOWNLOADS.map((download) => {
        const details = clientDetails[download.id]

        return (
          <Card
            key={download.filename}
            size='sm'
            className='h-full transition-shadow hover:shadow-md'
            role='listitem'
          >
            <CardHeader>
              <div className='bg-primary/10 text-primary mb-1 flex size-9 items-center justify-center rounded-lg'>
                <HugeiconsIcon icon={details.icon} className='size-5' />
              </div>
              <CardTitle>{details.name}</CardTitle>
              <CardDescription>{details.description}</CardDescription>
              <CardAction>
                <Badge variant='secondary'>macOS</Badge>
              </CardAction>
            </CardHeader>
            <CardContent className='mt-auto'>
              <p
                className='text-muted-foreground truncate font-mono text-[11px]'
                title={download.filename}
              >
                {download.filename}
              </p>
            </CardContent>
            <CardFooter>
              <Button
                variant='outline'
                className='w-full'
                aria-label={t('Download client: {{filename}}', {
                  filename: download.filename,
                })}
                render={
                  <a
                    href={download.url}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                <HugeiconsIcon icon={Download01Icon} data-icon='inline-start' />
                {t('Download')}
              </Button>
            </CardFooter>
          </Card>
        )
      })}
    </div>
  )
}
