/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Copy01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

type CodeSampleProps = {
  children: string
  language?: string
}

export function CodeSample(props: CodeSampleProps) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const resetTimer = useRef<number | null>(null)

  useEffect(
    () => () => {
      if (resetTimer.current !== null) window.clearTimeout(resetTimer.current)
    },
    []
  )

  const copyCode = async () => {
    await navigator.clipboard.writeText(props.children)
    setCopied(true)
    if (resetTimer.current !== null) window.clearTimeout(resetTimer.current)
    resetTimer.current = window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className='bg-muted/45 overflow-hidden rounded-lg border'>
      <div className='text-muted-foreground flex h-9 items-center justify-between border-b px-3 text-xs'>
        <span>{props.language ?? 'bash'}</span>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='h-7 gap-1.5 px-2 text-xs'
          onClick={copyCode}
          aria-label={copied ? t('Copied') : t('Copy')}
        >
          <HugeiconsIcon
            icon={copied ? Tick02Icon : Copy01Icon}
            className='size-3.5'
            strokeWidth={2}
            aria-hidden='true'
          />
          {copied ? t('Copied') : t('Copy')}
        </Button>
      </div>
      <pre className='overflow-x-auto p-4 text-sm leading-6'>
        <code>{props.children}</code>
      </pre>
    </div>
  )
}
