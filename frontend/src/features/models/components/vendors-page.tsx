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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Plus, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { ProviderBadge } from '@/components/provider-badge'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { getVendors } from '../api'
import { vendorsQueryKeys } from '../lib/query-keys'
import { handleDeleteVendor } from '../lib/vendor-actions'
import type { Vendor } from '../types'
import { VendorMutateDialog } from './dialogs/vendor-mutate-dialog'
import { useModels } from './models-provider'

export function VendorsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { open, setOpen, currentVendor, setCurrentVendor } = useModels()
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Vendor | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
  })

  const vendors = useMemo(() => {
    const items = data?.data?.items || []
    const keyword = search.trim().toLocaleLowerCase()
    if (!keyword) return items

    return items.filter((vendor) =>
      [vendor.name, vendor.description, vendor.icon].some((value) =>
        value?.toLocaleLowerCase().includes(keyword)
      )
    )
  }, [data?.data?.items, search])

  const handleCreate = () => {
    setCurrentVendor(null)
    setOpen('create-vendor')
  }

  const handleEdit = (vendor: Vendor) => {
    setCurrentVendor(vendor)
    setOpen('update-vendor')
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Manage Vendors')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={handleCreate}>
            <Plus />
            {t('Create Vendor')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            <div className='relative max-w-sm'>
              <Search
                aria-hidden
                className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2'
              />
              <Input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t('Search vendors...')}
                aria-label={t('Search vendors...')}
                className='pl-9'
              />
            </div>

            {isLoading ? (
              <div className='text-muted-foreground flex min-h-40 items-center justify-center'>
                <Loader2 aria-hidden className='size-5 animate-spin' />
                <span className='sr-only'>{t('Loading...')}</span>
              </div>
            ) : (
              <div className='min-h-0 flex-1 overflow-auto'>
                <StaticDataTable
                  data={vendors}
                  getRowKey={(vendor) => vendor.id}
                  emptyClassName='text-muted-foreground text-sm'
                  emptyContent={t('No vendor data available')}
                  columns={[
                    {
                      id: 'name',
                      header: t('Name'),
                      cellClassName: 'font-medium',
                      cell: (vendor) => (
                        <ProviderBadge
                          iconKey={vendor.icon}
                          label={vendor.name}
                        />
                      ),
                    },
                    {
                      id: 'description',
                      header: t('Description'),
                      cellClassName: 'text-muted-foreground max-w-md',
                      cell: (vendor) => vendor.description || '--',
                    },
                    {
                      id: 'status',
                      header: t('Status'),
                      cell: (vendor) => (
                        <StatusBadge
                          label={
                            vendor.status === 1 ? t('Enabled') : t('Disabled')
                          }
                          variant={vendor.status === 1 ? 'success' : 'neutral'}
                          copyable={false}
                        />
                      ),
                    },
                    {
                      id: 'actions',
                      header: t('Actions'),
                      className: 'text-right',
                      cellClassName: 'text-right',
                      cell: (vendor) => (
                        <StaticRowActions
                          editLabel={t('Edit')}
                          deleteLabel={t('Delete')}
                          menuLabel={t('Open menu')}
                          onEdit={() => handleEdit(vendor)}
                          onDelete={() => setDeleteTarget(vendor)}
                        />
                      ),
                    },
                  ]}
                />
              </div>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <VendorMutateDialog
        open={open === 'create-vendor' || open === 'update-vendor'}
        onOpenChange={(nextOpen) => !nextOpen && setOpen(null)}
        currentVendor={open === 'update-vendor' ? currentVendor : null}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
        title={t('Delete Vendor')}
        desc={t(
          'Are you sure you want to delete vendor "{{name}}"? This action cannot be undone.',
          { name: deleteTarget?.name }
        )}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          if (!deleteTarget) return
          handleDeleteVendor(deleteTarget.id, queryClient, () =>
            setDeleteTarget(null)
          )
        }}
      />
    </>
  )
}
