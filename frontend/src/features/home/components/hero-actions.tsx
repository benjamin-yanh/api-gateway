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
import { Key01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export function HeroActions() {
  const { t } = useTranslation()

  return (
    <nav
      aria-label={t('Quick actions')}
      className='grid w-full max-w-xl grid-cols-1 gap-3 sm:grid-cols-2'
    >
      <Button
        size='lg'
        className='h-16 w-full justify-start gap-0 bg-foreground p-1 text-base font-semibold text-background shadow-sm hover:bg-foreground/90 sm:text-lg'
        render={<Link to='/keys' />}
      >
        <span className='flex h-full w-14 shrink-0 items-center justify-center rounded-md bg-background text-foreground [&_svg]:size-6'>
          <HugeiconsIcon icon={Key01Icon} aria-hidden='true' strokeWidth={2} />
        </span>
        <span className='flex-1 px-3 text-center'>{t('Get API Key')}</span>
      </Button>

      <Button
        variant='outline'
        size='lg'
        className='relative h-16 w-full border-2 bg-background text-base font-semibold shadow-sm hover:bg-muted sm:text-lg'
        render={<Link to='/docs' hash='desktop-clients' />}
      >
        {t('Download Client')}
        <Badge className='absolute -top-3 right-4 bg-foreground text-background shadow-sm'>
          {t('Installation guide')}
        </Badge>
      </Button>
    </nav>
  )
}
