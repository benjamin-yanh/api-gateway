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
import { useMutation } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel, FieldLegend, FieldSet } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Skeleton } from '@/components/ui/skeleton'
import { CashbackRecordsButton } from '@/features/usage-cashback/components/cashback-records'
import { formatQuota } from '@/lib/format'

import { withdrawCashback } from '../api'

interface CashbackCardProps {
  quota: number
  loading: boolean
  onSuccess: () => Promise<void>
}

export function CashbackCard(props: CashbackCardProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [error, setError] = useState('')
  const submitting = useRef(false)
  const withdrawal = useMutation({
    mutationFn: withdrawCashback,
    retry: false,
  })
  const available = Number.isSafeInteger(props.quota) && props.quota > 0
  const methods = [
    { value: 'bank_card', label: t('Bank card'), disabled: true },
    { value: 'alipay', label: t('Alipay'), disabled: true },
    { value: 'wechat', label: t('WeChat'), disabled: true },
    { value: 'usdt', label: 'USDT', disabled: true },
    { value: 'balance', label: t('Current Balance'), disabled: false },
  ]

  const handleWithdraw = async () => {
    if (!available || props.loading || submitting.current) return
    submitting.current = true
    setError('')
    try {
      const response = await withdrawal.mutateAsync(props.quota)
      if (!response.success) {
        setError(t('Cashback withdrawal failed. Please try again.'))
        return
      }
      setOpen(false)
      toast.success(t('Cashback transferred to your current balance'))
      await props.onSuccess()
    } catch (cause) {
      if (
        isAxiosError(cause) &&
        cause.response?.data?.code === 'cashback_balance_changed'
      ) {
        setError(
          t(
            'Cashback balance changed or the current balance limit was reached. Refresh and try again.'
          )
        )
      } else {
        setError(t('Cashback withdrawal failed. Please try again.'))
      }
    } finally {
      submitting.current = false
    }
  }

  return (
    <>
      <div className='flex flex-wrap items-center justify-between gap-3 rounded-lg border px-4 py-3 sm:px-5'>
        <div className='min-w-0'>
          <div className='text-muted-foreground text-sm'>
            {t('Cashback amount')}
          </div>
          {props.loading ? (
            <Skeleton className='mt-1 h-7 w-28' />
          ) : (
            <div className='mt-1 font-mono text-xl font-semibold break-all tabular-nums'>
              {formatQuota(props.quota)}
            </div>
          )}
        </div>
        <div className='flex flex-wrap gap-2'>
          <CashbackRecordsButton />
          <Button
            variant='outline'
            disabled={props.loading}
            onClick={() => {
              setError('')
              setOpen(true)
            }}
          >
            {t('Withdraw')}
          </Button>
        </div>
      </div>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          if (!submitting.current) setOpen(nextOpen)
        }}
        title={t('Withdraw cashback')}
        description={t(
          'Withdraw all available cashback to your current balance for API usage.'
        )}
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        showCloseButton={!withdrawal.isPending}
        footer={
          <>
            <Button
              variant='outline'
              disabled={withdrawal.isPending}
              onClick={() => setOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              disabled={props.loading || withdrawal.isPending || !available}
              onClick={handleWithdraw}
            >
              {withdrawal.isPending
                ? t('Processing...')
                : t('Confirm withdrawal')}
            </Button>
          </>
        }
      >
        <div className='mb-5'>
          <p className='text-muted-foreground text-sm'>
            {t('Cashback amount')}
          </p>
          <p className='mt-1 font-mono text-2xl font-semibold break-all tabular-nums'>
            {formatQuota(props.quota)}
          </p>
          {!available && (
            <p className='text-muted-foreground mt-2 text-sm'>
              {t('No cashback available to withdraw')}
            </p>
          )}
        </div>
        <FieldSet>
          <FieldLegend id='cashback-method-label'>
            {t('Withdrawal method')}
          </FieldLegend>
          <RadioGroup aria-labelledby='cashback-method-label' value='balance'>
            {methods.map((method) => (
              <Field
                key={method.value}
                orientation='horizontal'
                data-disabled={method.disabled}
              >
                <RadioGroupItem
                  id={`cashback-${method.value}`}
                  value={method.value}
                  disabled={method.disabled || withdrawal.isPending}
                />
                <FieldLabel htmlFor={`cashback-${method.value}`}>
                  {method.label}
                  {method.disabled && ` (${t('Not yet available')})`}
                </FieldLabel>
              </Field>
            ))}
          </RadioGroup>
        </FieldSet>
        {error && (
          <p role='alert' className='text-destructive mt-4 text-sm'>
            {error}
          </p>
        )}
      </Dialog>
    </>
  )
}
