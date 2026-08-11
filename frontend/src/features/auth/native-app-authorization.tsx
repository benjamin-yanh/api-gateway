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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { AuthLayout } from './auth-layout'
import {
  buildNativeAppDeniedRedirect,
  parseNativeAppLoopbackRedirect,
} from './lib/native-app-callback'

export interface NativeAppAuthorizationRequest {
  client_id?: string
  redirect_uri?: string
  code_challenge?: string
  code_challenge_method?: string
  state?: string
}

interface NativeAppAuthorizeResponse {
  success?: boolean
  message?: string
  data?: { redirect_url?: string }
}

export function NativeAppAuthorization(props: {
  request: NativeAppAuthorizationRequest
}) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const request = props.request
  const redirect = parseNativeAppLoopbackRedirect(request.redirect_uri ?? '')
  const isValid = Boolean(
    request.client_id &&
    /^[A-Za-z0-9._-]{1,64}$/.test(request.client_id) &&
    redirect &&
    request.code_challenge_method === 'S256' &&
    request.code_challenge &&
    /^[A-Za-z0-9_-]{43,128}$/.test(request.code_challenge) &&
    request.state &&
    /^[A-Za-z0-9._~-]{16,512}$/.test(request.state)
  )

  const authorize = async () => {
    if (!isValid) return
    setIsSubmitting(true)
    try {
      const response = await api.post<NativeAppAuthorizeResponse>(
        '/api/native-auth/authorize',
        request,
        { skipBusinessError: true }
      )
      const redirectURL = response.data?.data?.redirect_url
      if (!response.data?.success || !redirectURL) {
        throw new Error(response.data?.message || t('Authorization failed'))
      }
      window.location.assign(redirectURL)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Authorization failed')
      )
      setIsSubmitting(false)
    }
  }

  const cancel = () => {
    const deniedRedirect = buildNativeAppDeniedRedirect(
      request.redirect_uri ?? '',
      request.state ?? ''
    )
    if (deniedRedirect) {
      window.location.assign(deniedRedirect)
      return
    }
    window.location.assign('/dashboard')
  }

  return (
    <AuthLayout>
      <Card>
        <CardHeader>
          <CardTitle>{t('Native application authorization')}</CardTitle>
          <CardDescription>
            {t('This application wants to access your account.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          {!isValid ? (
            <p className='text-destructive text-sm' role='alert'>
              {t('Invalid authorization request')}
            </p>
          ) : (
            <dl className='grid gap-3 text-sm'>
              <div className='grid gap-1'>
                <dt className='text-muted-foreground'>{t('Application')}</dt>
                <dd className='font-mono break-all'>{request.client_id}</dd>
              </div>
              <div className='grid gap-1'>
                <dt className='text-muted-foreground'>
                  {t('Callback address')}
                </dt>
                <dd className='font-mono break-all'>{redirect?.host}</dd>
              </div>
              <div className='grid gap-1'>
                <dt className='text-muted-foreground'>{t('Account')}</dt>
                <dd>{user?.display_name || user?.username}</dd>
              </div>
              <p className='text-muted-foreground'>
                {t(
                  'After authorization, you will return to the local application.'
                )}
              </p>
            </dl>
          )}
        </CardContent>
        <CardFooter className='justify-end gap-2'>
          <Button variant='outline' onClick={cancel} disabled={isSubmitting}>
            {t('Cancel')}
          </Button>
          <Button onClick={authorize} disabled={!isValid || isSubmitting}>
            {t('Authorize')}
          </Button>
        </CardFooter>
      </Card>
    </AuthLayout>
  )
}
