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
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { Eye } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { TableId } from '@/components/table-id'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatTimestamp } from '@/lib/format'

import { getAccessLogs } from '../api'
import type { AccessLogListItem } from '../types'
import { AccessLogDetailDialog } from './access-log-detail-dialog'

const route = getRouteApi('/_authenticated/access-logs/')

function statusVariant(status: number): 'outline' | 'warning' | 'destructive' {
  if (status >= 500) return 'destructive'
  if (status >= 400) return 'warning'
  return 'outline'
}

export function AccessLogsTable() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [selectedLogId, setSelectedLogId] = useState<number | null>(null)
  const tableState = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 50 },
    globalFilter: { enabled: true, key: 'url' },
    columnFilters: [],
  })
  const query = useQuery({
    queryKey: [
      'access-logs',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
    ],
    queryFn: async () => {
      const result = await getAccessLogs({
        p: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        url: tableState.globalFilter?.trim() || undefined,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to load access logs'))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })
  const columns = useMemo<ColumnDef<AccessLogListItem>[]>(
    () => [
      {
        accessorKey: 'created_at',
        header: t('Time'),
        cell: ({ row }) => formatTimestamp(row.original.created_at),
        size: 170,
        meta: { mobileOrder: 10 },
      },
      {
        accessorKey: 'method',
        header: t('Method'),
        cell: ({ row }) => (
          <Badge variant='outline'>{row.original.method}</Badge>
        ),
        size: 90,
        meta: { mobileBadge: true },
      },
      {
        accessorKey: 'url',
        header: t('URL'),
        cell: ({ row }) => (
          <LongText className='max-w-[34rem] font-mono text-xs'>
            {row.original.url}
          </LongText>
        ),
        size: 480,
        meta: { mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: t('Status'),
        cell: ({ row }) => (
          <Badge variant={statusVariant(row.original.status)}>
            {row.original.status}
          </Badge>
        ),
        size: 90,
        meta: { mobileBadge: true },
      },
      {
        accessorKey: 'latency_ms',
        header: t('Latency'),
        cell: ({ row }) => `${row.original.latency_ms} ms`,
        size: 110,
        meta: { mobileOrder: 30 },
      },
      {
        accessorKey: 'username',
        header: t('Username'),
        cell: ({ row }) => row.original.username || '-',
        size: 180,
        meta: { mobileOrder: 20 },
      },
      {
        accessorKey: 'ip',
        header: t('IP Address'),
        cell: ({ row }) => <TableId value={row.original.ip || '-'} />,
        size: 150,
        meta: { mobileOrder: 40 },
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: ({ row }) => (
          <Button
            variant='ghost'
            size='sm'
            onClick={() => setSelectedLogId(row.original.id)}
            aria-label={t('View access log details')}
          >
            <Eye aria-hidden='true' />
            {t('Details')}
          </Button>
        ),
        enableHiding: false,
        size: 110,
        meta: { mobileOrder: 50 },
      },
    ],
    [t]
  )
  const logs = query.data?.items || []
  const table = useDataTable({
    data: logs,
    columns,
    globalFilter: tableState.globalFilter,
    pagination: tableState.pagination,
    enableRowSelection: false,
    onGlobalFilterChange: tableState.onGlobalFilterChange,
    onPaginationChange: tableState.onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: query.data?.total || 0,
    ensurePageInRange: tableState.ensurePageInRange,
  })

  return (
    <>
      <DataTablePage
        table={table.table}
        columns={columns}
        isLoading={query.isLoading}
        isFetching={query.isFetching}
        emptyTitle={t('No Access Logs Found')}
        emptyDescription={t(
          'API requests will appear here after they are received.'
        )}
        skeletonKeyPrefix='access-log-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Filter by URL...'),
          searchDebounceMs: 500,
        }}
      />
      <AccessLogDetailDialog
        logId={selectedLogId}
        open={selectedLogId != null}
        onOpenChange={(open) => {
          if (!open) setSelectedLogId(null)
        }}
      />
    </>
  )
}
