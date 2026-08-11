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
import { Mail } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { SiGithub } from 'react-icons/si'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { createOAuthFlow } from '@/features/auth/api'
import {
  OAUTH_BIND_CALLBACK_MESSAGE,
  OAUTH_BIND_RESULT_MESSAGE,
} from '@/features/auth/constants'
import { watchOAuthPopupClosed } from '@/features/auth/lib/oauth-bind-window'
import {
  getOAuthSessionStorage,
  markOAuthBindPopup,
} from '@/features/auth/lib/oauth-callback-mode'
import { useDialogs } from '@/hooks/use-dialog'
import { useStatus } from '@/hooks/use-status'
import { api } from '@/lib/api'
import { buildGitHubOAuthUrl } from '@/lib/oauth'

import type { UserProfile, BindingItem } from '../../types'
import { EmailBindDialog } from '../dialogs/email-bind-dialog'

// ============================================================================
// Account Bindings Tab Component
// ============================================================================

interface AccountBindingsTabProps {
  profile: UserProfile | null
  onUpdate: () => void
}

type DialogKey = 'email'

interface PendingOAuthBinding {
  provider: string
  state: string
  popup: Window
  stopCloseWatcher: () => void
}

interface OAuthBindingCallback {
  type: typeof OAUTH_BIND_CALLBACK_MESSAGE
  provider: string
  state: string
  code?: string
  error?: string
  errorDescription?: string
}

export function AccountBindingsTab({
  profile,
  onUpdate,
}: AccountBindingsTabProps) {
  const { t } = useTranslation()
  const dialogs = useDialogs<DialogKey>()
  const { status, loading } = useStatus()
  const pendingOAuthBinding = useRef<PendingOAuthBinding | null>(null)

  const clearPendingOAuthBinding = useCallback(
    (expected?: PendingOAuthBinding) => {
      const pending = pendingOAuthBinding.current
      if (!pending || (expected && pending !== expected)) return
      pending.stopCloseWatcher()
      pendingOAuthBinding.current = null
    },
    []
  )

  const startOAuthBinding = useCallback(
    async (provider: string, buildUrl: (state: string) => string) => {
      const previous = pendingOAuthBinding.current
      if (previous) {
        clearPendingOAuthBinding(previous)
        if (!previous.popup.closed) previous.popup.close()
      }

      const popup = window.open('', '_blank')
      if (!popup) {
        toast.error(t('OAuth pop-up was blocked'))
        return
      }
      const pending: PendingOAuthBinding = {
        provider,
        state: '',
        popup,
        stopCloseWatcher: () => undefined,
      }
      pending.stopCloseWatcher = watchOAuthPopupClosed(popup, () =>
        clearPendingOAuthBinding(pending)
      )
      pendingOAuthBinding.current = pending
      try {
        const state = await createOAuthFlow(provider, 'bind')
        if (pendingOAuthBinding.current !== pending || popup.closed) return
        // Stamp the popup while it is still same-origin (about:blank). Tying
        // the mark to this state prevents a stale popup from claiming a later
        // login callback. If storage is blocked, do not navigate into a
        // callback that cannot safely identify the bind flow.
        if (
          !markOAuthBindPopup(getOAuthSessionStorage(popup), provider, state)
        ) {
          throw new Error('OAuth bind popup storage is unavailable')
        }
        pending.state = state
        popup.location.replace(buildUrl(state))
      } catch {
        const isCurrent = pendingOAuthBinding.current === pending
        clearPendingOAuthBinding(pending)
        popup.close()
        if (isCurrent) toast.error(t('Failed to initialize OAuth'))
      }
    },
    [clearPendingOAuthBinding, t]
  )

  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleMessage = async (event: MessageEvent<unknown>) => {
      if (event.origin !== window.location.origin) return
      const message = event.data as Partial<OAuthBindingCallback> | null
      const pending = pendingOAuthBinding.current
      if (
        !message ||
        message.type !== OAUTH_BIND_CALLBACK_MESSAGE ||
        !pending ||
        message.provider !== pending.provider ||
        message.state !== pending.state ||
        event.source !== pending.popup
      ) {
        return
      }

      clearPendingOAuthBinding(pending)
      let success = false
      let resultMessage = t('OAuth failed')
      try {
        if (!message.code && !message.error) {
          throw new Error(t('Missing code'))
        }
        const params: Record<string, string> = { state: message.state }
        if (message.code) params.code = message.code
        if (message.error) params.error = message.error
        if (message.errorDescription) {
          params.error_description = message.errorDescription
        }
        const response = await api.get(`/api/oauth/${message.provider}`, {
          params,
          skipBusinessError: true,
        })
        success = Boolean(response.data?.success)
        resultMessage = response.data?.message || resultMessage
        if (success) {
          toast.success(t('Binding successful!'))
          onUpdate()
        } else {
          toast.error(resultMessage)
        }
      } catch (error: unknown) {
        resultMessage =
          (error as { response?: { data?: { message?: string } } }).response
            ?.data?.message ||
          (error instanceof Error ? error.message : resultMessage)
        toast.error(resultMessage)
      }

      pending.popup.postMessage(
        {
          type: OAUTH_BIND_RESULT_MESSAGE,
          provider: message.provider,
          state: message.state,
          success,
          message: resultMessage,
        },
        window.location.origin
      )
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [clearPendingOAuthBinding, onUpdate, t])

  useEffect(
    () => () => {
      const pending = pendingOAuthBinding.current
      clearPendingOAuthBinding(pending ?? undefined)
      if (pending && !pending.popup.closed) pending.popup.close()
    },
    [clearPendingOAuthBinding]
  )

  const bindings: BindingItem[] =
    profile && status
      ? [
          {
            id: 'email',
            label: t('Email'),
            icon: Mail,
            value: profile.email,
            isBound: Boolean(profile.email),
            isEnabled: true,
            onBind: () => dialogs.open('email'),
          },
          {
            id: 'github',
            label: t('GitHub'),
            icon: SiGithub,
            value: (profile as unknown as Record<string, unknown>).github_id as
              | string
              | undefined,
            isBound: Boolean(
              (profile as unknown as Record<string, unknown>).github_id
            ),
            isEnabled: status?.github_oauth || false,
            onBind: () => {
              const clientId = status?.github_client_id
              if (clientId) {
                void startOAuthBinding('github', (state) =>
                  buildGitHubOAuthUrl(clientId, state)
                )
              }
            },
          },
        ].filter((binding) => binding.isEnabled)
      : []

  if (!profile || loading) return null

  return (
    <>
      <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2 sm:gap-3'>
        {bindings.map((binding) => {
          let actionLabel = t('Bind')
          if (binding.isBound && binding.id === 'email') {
            actionLabel = t('Change')
          } else if (binding.isBound) {
            actionLabel = t('Bound')
          }

          return (
            <div
              key={binding.id}
              className='flex items-center justify-between gap-2.5 rounded-lg border p-2.5 sm:gap-3 sm:p-3'
            >
              <div className='flex min-w-0 items-center gap-2.5 sm:gap-3'>
                <div className='bg-muted shrink-0 rounded-md p-1.5 sm:p-2'>
                  <binding.icon className='h-4 w-4' />
                </div>
                <div className='min-w-0'>
                  <div className='flex items-center gap-1.5'>
                    <p className='text-sm font-medium'>{binding.label}</p>
                    {binding.isBound && (
                      <StatusBadge
                        label={t('Bound')}
                        variant='success'
                        copyable={false}
                      />
                    )}
                  </div>
                  <p className='text-muted-foreground truncate text-xs'>
                    {binding.value || t('Not bound')}
                  </p>
                </div>
              </div>
              <Button
                variant='outline'
                size='sm'
                className='h-7 shrink-0 px-2.5 text-xs'
                onClick={binding.onBind}
                disabled={binding.isBound && binding.id !== 'email'}
              >
                {actionLabel}
              </Button>
            </div>
          )
        })}
      </div>

      {/* Email Bind Dialog */}
      <EmailBindDialog
        open={dialogs.isOpen('email')}
        onOpenChange={(open) =>
          open ? dialogs.open('email') : dialogs.close('email')
        }
        currentEmail={profile.email}
        onSuccess={onUpdate}
      />
    </>
  )
}
