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
  CustomerService01Icon,
  Message01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const CUSTOMER_QQ = '3138763753'
const TELEGRAM_GROUP_URL = 'https://t.me/gtongxue'

export function ContactFloatingCard() {
  const { t } = useTranslation()

  return (
    <Card
      size='sm'
      className='bg-card/95 fixed right-4 bottom-4 z-40 w-72 max-w-[calc(100vw-2rem)] shadow-lg backdrop-blur'
    >
      <CardHeader className='pb-0'>
        <CardTitle className='flex items-center gap-2'>
          <span className='text-primary [&_svg]:size-4'>
            <HugeiconsIcon
              icon={CustomerService01Icon}
              aria-hidden='true'
              strokeWidth={2}
            />
          </span>
          {t('Contact us')}
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-2'>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground'>
            {t('Customer service QQ')}
          </span>
          <span className='font-medium tabular-nums select-all'>
            {CUSTOMER_QQ}
          </span>
        </div>
        <a
          href={TELEGRAM_GROUP_URL}
          target='_blank'
          rel='noopener noreferrer'
          className='hover:bg-muted focus-visible:ring-ring flex items-center justify-between gap-3 rounded-md py-1 transition-colors focus-visible:ring-2 focus-visible:outline-none'
        >
          <span className='text-muted-foreground'>{t('Telegram group')}</span>
          <span className='text-primary flex items-center gap-1 font-medium [&_svg]:size-4'>
            <HugeiconsIcon
              icon={Message01Icon}
              aria-hidden='true'
              strokeWidth={2}
            />
            t.me/gtongxue
          </span>
        </a>
      </CardContent>
    </Card>
  )
}
