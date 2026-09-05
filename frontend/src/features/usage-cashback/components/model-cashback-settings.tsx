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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useId, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { FieldGroup, FieldLegend, FieldSet } from '@/components/ui/field'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useAuthStore } from '@/stores/auth-store'

import { getCashbackSettings, saveCashbackSettings } from '../api'
import {
  createCashbackSchema,
  percentToRatio,
  ratioToPercent,
  type CashbackFormValues,
} from '../lib/rules'
import type { CashbackSettings } from '../types'

interface ModelCashbackSettingsProps {
  modelName: string
  tokenPriced: boolean
}

export function ModelCashbackSettings(props: ModelCashbackSettingsProps) {
  const { t } = useTranslation()
  const isRoot = useAuthStore((state) => (state.auth.user?.role ?? 0) >= 100)
  const query = useQuery({
    queryKey: ['cashback', 'settings'],
    queryFn: getCashbackSettings,
    enabled: isRoot && Boolean(props.modelName),
    refetchOnWindowFocus: false,
    retry: false,
  })

  if (!props.modelName) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Save model pricing before configuring usage cashback.')}
      </p>
    )
  }
  if (!isRoot) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Only the root administrator can configure usage cashback.')}
      </p>
    )
  }
  if (query.isPending) return <Skeleton className='h-48 w-full' />
  if (query.isError || !query.data) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t('Unable to load cashback settings.')}
          <Button
            type='button'
            variant='outline'
            onClick={() => void query.refetch()}
          >
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }
  return (
    <CashbackSettingsForm
      key={`${props.modelName}-${query.data.version}`}
      {...props}
      settings={query.data}
      onReload={() => void query.refetch()}
    />
  )
}

function CashbackSettingsForm(
  props: ModelCashbackSettingsProps & {
    settings: CashbackSettings
    onReload: () => void
  }
) {
  const { t } = useTranslation()
  const id = useId()
  const queryClient = useQueryClient()
  const supported =
    props.tokenPriced &&
    props.settings.supported_models?.[props.modelName]?.supported === true
  const model = props.settings.models[props.modelName]
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const form = useForm<CashbackFormValues>({
    resolver: zodResolver(createCashbackSchema(t, supported)),
    defaultValues: {
      enabled: model?.enabled ?? false,
      input_per_million: model?.input_per_million ?? '0',
      output_per_million: model?.output_per_million ?? '0',
      global_enabled: props.settings.enabled,
      cap_percent: ratioToPercent(props.settings.max_ratio),
    },
  })
  const values = form.watch()
  const mutation = useMutation({
    mutationFn: saveCashbackSettings,
    retry: false,
    onSuccess: (settings) => {
      queryClient.setQueryData(['cashback', 'settings'], settings)
      void queryClient.invalidateQueries({ queryKey: ['cashback', 'rules'] })
      toast.success(t('Cashback rules saved'))
    },
  })
  const save = form.handleSubmit(async (next) => {
    if (mutation.isPending || conflict) return
    setError('')
    try {
      await mutation.mutateAsync({
        ...props.settings,
        enabled: next.global_enabled,
        max_ratio: percentToRatio(next.cap_percent),
        models: {
          ...props.settings.models,
          [props.modelName]: {
            enabled: next.enabled,
            input_per_million: next.input_per_million,
            output_per_million: next.output_per_million,
          },
        },
      })
    } catch (cause) {
      const stale = isAxiosError(cause) && cause.response?.status === 409
      setConflict(stale)
      let message = t(
        'Unable to save cashback settings. Check the configuration and try again.'
      )
      if (stale) {
        message = t(
          'Cashback settings changed. Reload the current rules before saving again.'
        )
      }
      if (
        isAxiosError(cause) &&
        cause.response?.data?.code === 'cashback_requires_durable_billing'
      ) {
        message = t('Disable batch balance updates before enabling cashback.')
      }
      setError(message)
    }
  })

  return (
    <Form {...form}>
      <FieldSet className='rounded-lg border p-4' disabled={mutation.isPending}>
        <FieldLegend>{t('Usage cashback')}</FieldLegend>
        <p className='text-muted-foreground text-sm'>
          {t('Cashback rules are saved separately from model prices.')}
        </p>
        {!supported && (
          <Alert>
            <AlertDescription>
              {t('This model is not verified for usage cashback.')}
            </AlertDescription>
          </Alert>
        )}
        <FieldGroup className='gap-4'>
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <FormItem>
                <div className='flex items-center justify-between gap-3'>
                  <FormLabel htmlFor={`${id}-enabled`}>
                    {t('Enable model cashback')}
                  </FormLabel>
                  <FormControl>
                    <Switch
                      id={`${id}-enabled`}
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={
                        mutation.isPending || (!supported && !field.value)
                      }
                    />
                  </FormControl>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />
          {(['input_per_million', 'output_per_million'] as const).map(
            (name) => (
              <FormField
                key={name}
                control={form.control}
                name={name}
                render={({ field }) => (
                  <FormItem data-disabled={!values.enabled}>
                    <FormLabel>
                      {name === 'input_per_million'
                        ? t('Uncached input cashback (CNY / 1M tokens)')
                        : t('Output cashback (CNY / 1M tokens)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        inputMode='decimal'
                        disabled={!values.enabled || mutation.isPending}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )
          )}
          <p className='text-muted-foreground text-xs'>
            {t(
              'Cache reads and writes are excluded. Zero means no cashback for that token category.'
            )}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Example: 200,000 uncached input and 50,000 output tokens earn {{amount}} CNY before the cap.',
              {
                amount: new Intl.NumberFormat(undefined, {
                  maximumFractionDigits: 10,
                }).format(
                  (Number(values.input_per_million) || 0) * 0.2 +
                    (Number(values.output_per_million) || 0) * 0.05
                ),
              }
            )}
          </p>
          <p className='text-sm'>
            {values.cap_percent
              ? t('Cashback is capped at {{percent}}% of the actual charge.', {
                  percent: values.cap_percent,
                })
              : t(
                  'No percentage cap. Cashback is calculated from actual eligible usage.'
                )}
          </p>
          {!values.global_enabled && (
            <p className='text-muted-foreground text-sm'>
              {t(
                'Global cashback is off. New requests will not earn cashback.'
              )}
            </p>
          )}
          <details
            className='rounded-md border p-3'
            open={Boolean(form.formState.errors.cap_percent)}
          >
            <summary className='cursor-pointer text-sm font-medium'>
              {t('Global cashback settings')}
            </summary>
            <FieldGroup className='mt-4 gap-4'>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'These settings apply to all models. Changes affect new requests only.'
                )}
              </p>
              <FormField
                control={form.control}
                name='global_enabled'
                render={({ field }) => (
                  <FormItem>
                    <div className='flex items-center justify-between gap-3'>
                      <FormLabel htmlFor={`${id}-global`}>
                        {t('Enable global cashback')}
                      </FormLabel>
                      <FormControl>
                        <Switch
                          id={`${id}-global`}
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={mutation.isPending}
                        />
                      </FormControl>
                    </div>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='cap_percent'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Maximum cashback (% of actual charge, optional)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Leave empty for no percentage cap')}
                        inputMode='decimal'
                        disabled={mutation.isPending}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </FieldGroup>
          </details>
          {error && (
            <p role='alert' className='text-destructive text-sm'>
              {error}
            </p>
          )}
          {conflict && (
            <Button type='button' variant='outline' onClick={props.onReload}>
              {t('Reload current cashback rules')}
            </Button>
          )}
          <Button
            type='button'
            disabled={mutation.isPending || conflict}
            onClick={() => void save()}
          >
            {mutation.isPending ? t('Saving...') : t('Save cashback rules')}
          </Button>
        </FieldGroup>
      </FieldSet>
    </Form>
  )
}
