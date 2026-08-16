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
import { Download01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { CLIENT_DOWNLOADS } from '../constants'

export function ClientDownloadButton() {
  const { t } = useTranslation()

  return (
    <div className='flex flex-wrap gap-3'>
      {CLIENT_DOWNLOADS.map((download) => (
        <Button
          key={download.filename}
          variant='secondary'
          className='h-auto min-h-12 rounded-xl px-4 py-2.5 shadow-sm'
          aria-label={t('Download client: {{filename}}', {
            filename: download.filename,
          })}
          render={
            <a href={download.url} target='_blank' rel='noopener noreferrer' />
          }
        >
          <HugeiconsIcon icon={Download01Icon} data-icon='inline-start' />
          <span className='flex flex-col items-start leading-tight'>
            <span>{t('Download macOS client')}</span>
            <span className='text-muted-foreground text-[11px] font-normal'>
              {download.filename}
            </span>
          </span>
        </Button>
      ))}
    </div>
  )
}
