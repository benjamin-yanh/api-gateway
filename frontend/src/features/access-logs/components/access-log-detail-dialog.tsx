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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestamp } from '@/lib/format'

import { getAccessLog } from '../api'
import {
  formatAccessLogJSON,
  tokenizeAccessLogJSON,
  type AccessLogJSONToken,
} from '../lib/format'
import type { AccessLogDetail } from '../types'

interface AccessLogDetailDialogProps {
  logId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function DetailField(props: { label: string; value: React.ReactNode }) {
  return (
    <div className='grid gap-1'>
      <dt className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </dt>
      <dd className='break-all'>{props.value}</dd>
    </div>
  )
}

function BodyBlock(props: {
  label: string
  value: string
  contentType?: string
}) {
  const formattedValue = formatAccessLogJSON(props.value)
  const tokens = tokenizeAccessLogJSON(formattedValue)

  return (
    <section className='grid gap-2'>
      <div className='flex items-center gap-2'>
        <h3 className='text-sm font-medium'>{props.label}</h3>
        {props.contentType ? (
          <Badge variant='secondary'>{props.contentType}</Badge>
        ) : null}
      </div>
      <pre className='bg-muted/40 max-h-96 overflow-auto rounded-lg border p-3 font-mono text-xs leading-5 break-all whitespace-pre-wrap'>
        <code>
          {tokens.map((token) => (
            <span className={jsonTokenClassName(token)} key={token.offset}>
              {token.value}
            </span>
          ))}
        </code>
      </pre>
    </section>
  )
}

function jsonTokenClassName(token: AccessLogJSONToken): string | undefined {
  switch (token.type) {
    case 'key':
      return 'text-primary font-medium'
    case 'string':
      return 'text-chart-5'
    case 'number':
      return 'text-chart-4'
    case 'boolean':
    case 'null':
      return 'text-chart-1 font-medium'
    case 'punctuation':
      return 'text-muted-foreground'
    default:
      return undefined
  }
}

function AccessLogDetailContent(props: { detail: AccessLogDetail }) {
  const { t } = useTranslation()
  const detail = props.detail
  return (
    <div className='grid gap-5'>
      <dl className='grid gap-4 sm:grid-cols-2'>
        <DetailField
          label={t('Time')}
          value={formatTimestamp(detail.created_at)}
        />
        <DetailField label={t('Request ID')} value={detail.request_id || '-'} />
        <DetailField
          label={t('Method')}
          value={<Badge variant='outline'>{detail.method}</Badge>}
        />
        <DetailField label={t('Status')} value={detail.status} />
        <DetailField label={t('URL')} value={<code>{detail.url}</code>} />
        <DetailField
          label={t('Route')}
          value={<code>{detail.route || '-'}</code>}
        />
        <DetailField label={t('Username')} value={detail.username || '-'} />
        <DetailField label={t('IP Address')} value={detail.ip || '-'} />
        <DetailField label={t('Latency')} value={`${detail.latency_ms} ms`} />
        <DetailField label={t('Node')} value={detail.node_name || '-'} />
      </dl>
      <section className='grid gap-4 border-t pt-5'>
        <h2 className='text-base font-semibold'>{t('Request Content')}</h2>
        <BodyBlock label={t('Request Headers')} value={detail.headers} />
        {detail.body ? (
          <BodyBlock label={t('JSON Request Body')} value={detail.body} />
        ) : (
          <section className='grid gap-2'>
            <h3 className='text-sm font-medium'>{t('JSON Request Body')}</h3>
            <p className='text-muted-foreground rounded-lg border border-dashed p-3 text-sm'>
              {detail.body_omitted
                ? t(
                    'JSON body was omitted because it exceeded the logging limit.'
                  )
                : t('No JSON request body was recorded.')}
            </p>
          </section>
        )}
      </section>
      <section className='grid gap-4 border-t pt-5'>
        <h2 className='text-base font-semibold'>{t('Response Content')}</h2>
        {detail.response_body ? (
          <section className='grid gap-2'>
            <BodyBlock
              label={
                detail.response_body_type === 'text/event-stream'
                  ? t('Stream Response')
                  : t('Response Body')
              }
              value={detail.response_body}
              contentType={detail.response_body_type}
            />
            {detail.response_body_truncated ? (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Response body was truncated because it exceeded the logging limit.'
                )}
              </p>
            ) : null}
          </section>
        ) : (
          <section className='grid gap-2'>
            <h3 className='text-sm font-medium'>{t('Response Body')}</h3>
            <p className='text-muted-foreground rounded-lg border border-dashed p-3 text-sm'>
              {t('No response body was recorded.')}
            </p>
          </section>
        )}
      </section>
    </div>
  )
}

export function AccessLogDetailDialog(props: AccessLogDetailDialogProps) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['access-log-detail', props.logId],
    queryFn: async () => {
      const response = await getAccessLog(props.logId as number)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to load access log details')
        )
      }
      return response.data
    },
    enabled: props.open && props.logId != null,
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('Access Log Details')}</DialogTitle>
        </DialogHeader>
        <ScrollArea className='max-h-[calc(90vh-7rem)] pr-3'>
          {query.isLoading && <Skeleton className='h-96 w-full' />}
          {query.isError && (
            <p className='text-destructive p-4 text-sm'>
              {t('Failed to load access log details')}
            </p>
          )}
          {query.data && <AccessLogDetailContent detail={query.data} />}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
