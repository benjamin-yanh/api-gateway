/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { ClientDownloadButton } from '@/features/home/components/client-download-button'

import { CodeSample } from './code-sample'
import { ModelCatalog } from './model-catalog'

type DocsArticleProps = {
  baseUrl: string
}

const protocols = [
  {
    name: 'Anthropic Messages',
    method: 'POST',
    path: '/v1/messages',
    description:
      'Use the Anthropic request format for Claude-compatible models.',
  },
  {
    name: 'OpenAI Chat Completions',
    method: 'POST',
    path: '/v1/chat/completions',
    description:
      'Use the OpenAI-compatible chat format for supported chat models.',
  },
  {
    name: 'OpenAI Responses',
    method: 'POST',
    path: '/v1/responses',
    description:
      'Use the Responses format when the selected channel supports it.',
  },
  {
    name: 'Gemini native',
    method: 'POST',
    path: '/v1beta/models/{model}:generateContent',
    description:
      'Use the Gemini native request format for compatible Gemini channels.',
  },
]

const routingSteps = [
  {
    title: 'Validate the API key scope',
    description:
      'The requested model and group must be allowed by the API key.',
  },
  {
    title: 'Apply model mapping',
    description:
      'A channel can translate the public model name to its upstream model name.',
  },
  {
    title: 'Filter eligible channels',
    description:
      'Only enabled channels matching the group, model, and request path remain eligible.',
  },
  {
    title: 'Select by priority and weight',
    description:
      'Higher-priority channels are tried first; channels at the same priority are balanced by weight.',
  },
  {
    title: 'Reuse affinity or fail over',
    description:
      'Channel affinity can keep related requests together, while retries move to another eligible channel after configured failures.',
  },
]

const cursorSetupSteps = [
  'Create an API key on the API Keys page and copy it.',
  'Open Cursor Settings, then go to Models > API Keys.',
  'Enter your API key in OpenAI API Key.',
  'Enable Override OpenAI Base URL and enter the address below.',
  'Add or enable a standard chat model from the Available models section, then select it in Cursor.',
  'Depending on your Cursor version, use Verify or send a test message to confirm the connection.',
]

const sectionClassName =
  'scroll-mt-24 space-y-6 border-t pt-10 [content-visibility:auto] [contain-intrinsic-size:auto_520px]'

export function DocsArticle(props: DocsArticleProps) {
  const { t } = useTranslation()

  return (
    <article className='space-y-12'>
      <header id='overview' className='scroll-mt-24 space-y-6'>
        <div>
          <p className='text-primary mb-3 text-sm font-medium'>
            {t('API documentation')}
          </p>
          <h1 className='text-3xl font-bold tracking-tight sm:text-4xl'>
            {t('API integration guide')}
          </h1>
        </div>
        <p className='text-muted-foreground max-w-3xl text-base leading-8 sm:text-lg'>
          {t(
            'Connect OpenAI-compatible, Anthropic, and Gemini clients to this gateway using the protocol each model expects.'
          )}
        </p>
        <div className='bg-primary/5 border-primary/15 rounded-lg border p-4 text-sm leading-7'>
          <dl className='grid gap-x-5 gap-y-1 sm:grid-cols-[7rem_1fr]'>
            <dt className='font-semibold'>{t('Base URL')}</dt>
            <dd>
              <code className='bg-background rounded px-1.5 py-0.5'>
                {props.baseUrl}
              </code>
            </dd>
            <dt className='font-semibold'>{t('Authentication')}</dt>
            <dd>
              API Key (<code>sk-your-key</code>)
            </dd>
            <dt className='font-semibold'>{t('Protocol endpoints')}</dt>
            <dd>OpenAI · Anthropic · Gemini</dd>
          </dl>
        </div>
      </header>

      <section id='getting-started' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>
            {t('Base URL and authentication')}
          </h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t('Use your API key in the header required by each protocol.')}
          </p>
        </div>
        <div className='overflow-x-auto rounded-lg border'>
          <table className='w-full min-w-[36rem] text-left text-sm'>
            <thead className='bg-muted/50'>
              <tr>
                <th className='px-4 py-3 font-semibold'>{t('Protocol')}</th>
                <th className='px-4 py-3 font-semibold'>{t('Base URL')}</th>
                <th className='px-4 py-3 font-semibold'>{t('Header')}</th>
              </tr>
            </thead>
            <tbody className='divide-y'>
              <tr>
                <td className='px-4 py-3'>OpenAI</td>
                <td className='px-4 py-3 font-mono'>{props.baseUrl}/v1</td>
                <td className='px-4 py-3 font-mono'>Authorization: Bearer</td>
              </tr>
              <tr>
                <td className='px-4 py-3'>Anthropic</td>
                <td className='px-4 py-3 font-mono'>{props.baseUrl}</td>
                <td className='px-4 py-3 font-mono'>x-api-key</td>
              </tr>
              <tr>
                <td className='px-4 py-3'>Gemini</td>
                <td className='px-4 py-3 font-mono'>{props.baseUrl}</td>
                <td className='px-4 py-3 font-mono'>Authorization: Bearer</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p className='border-warning/25 bg-warning/5 rounded-lg border p-4 text-sm leading-6'>
          {t(
            'Replace sk-your-key with your own API key and never expose it in client-side code.'
          )}
        </p>
        <p className='bg-muted/40 rounded-lg border p-4 text-sm leading-6'>
          {t(
            'The API address shown here follows the address you used to open this site, including its currently accessible IP address.'
          )}
        </p>
      </section>

      <section id='cursor' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>{t('Cursor setup')}</h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t(
              'Connect Cursor to this gateway through its OpenAI-compatible API settings.'
            )}
          </p>
        </div>

        <div className='overflow-x-auto rounded-lg border'>
          <table className='w-full min-w-[36rem] text-left text-sm'>
            <thead className='bg-muted/50'>
              <tr>
                <th className='px-4 py-3 font-semibold'>
                  {t('Cursor setting')}
                </th>
                <th className='px-4 py-3 font-semibold'>{t('Value')}</th>
              </tr>
            </thead>
            <tbody className='divide-y'>
              <tr>
                <td className='px-4 py-3'>OpenAI API Key</td>
                <td className='px-4 py-3 font-mono'>sk-your-key</td>
              </tr>
              <tr>
                <td className='px-4 py-3'>Override OpenAI Base URL</td>
                <td className='px-4 py-3 font-mono'>{props.baseUrl}/v1</td>
              </tr>
              <tr>
                <td className='px-4 py-3'>{t('Model')}</td>
                <td className='px-4 py-3'>gpt-5.6-sol</td>
              </tr>
            </tbody>
          </table>
        </div>

        <ol className='space-y-3'>
          {cursorSetupSteps.map((step, index) => (
            <li key={step} className='flex gap-3 rounded-lg border p-4'>
              <span className='bg-primary text-primary-foreground flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
                {index + 1}
              </span>
              <p className='pt-0.5 text-sm leading-6'>{t(step)}</p>
            </li>
          ))}
        </ol>

        <div className='border-warning/25 bg-warning/5 space-y-2 rounded-lg border p-4 text-sm leading-6'>
          <p>
            {t(
              "Cursor custom API keys support standard chat models only. Features such as Tab Completion continue using Cursor's built-in models."
            )}
          </p>
          <p>
            {t(
              "Override OpenAI Base URL applies globally. Disable the OpenAI API key before switching back to Cursor's built-in models."
            )}
          </p>
        </div>

        <a
          href='https://docs.cursor.com/settings/api-keys'
          target='_blank'
          rel='noopener noreferrer'
          className='text-primary inline-flex text-sm font-medium hover:underline'
        >
          {t('Official documentation')}
        </a>
      </section>

      <section id='desktop-clients' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>{t('Desktop clients')}</h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t(
              'Download our macOS desktop clients for Claude and ChatGPT, then use the Base URL and API key above to connect.'
            )}
          </p>
        </div>
        <ClientDownloadButton />
      </section>

      <section id='models' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>{t('Available models')}</h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t(
              'The list is loaded from the public model discovery endpoint and reflects the gateway configuration. No API key is required for this request.'
            )}
          </p>
        </div>
        <CodeSample>{`curl ${props.baseUrl}/v1/models`}</CodeSample>
        <ModelCatalog />
      </section>

      <section id='protocols' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>{t('Protocol endpoints')}</h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t(
              'Choose an endpoint that matches the request format supported by your client and the selected model channel.'
            )}
          </p>
        </div>
        <div className='space-y-3'>
          {protocols.map((protocol) => (
            <Card key={protocol.path} className='shadow-none'>
              <CardHeader className='gap-2 sm:flex-row sm:items-start sm:justify-between'>
                <div>
                  <CardTitle>{t(protocol.name)}</CardTitle>
                  <CardDescription className='mt-1 leading-6'>
                    {t(protocol.description)}
                  </CardDescription>
                </div>
                <Badge variant='secondary'>{protocol.method}</Badge>
              </CardHeader>
              <CardContent>
                <code className='bg-muted block overflow-x-auto rounded-md px-3 py-2 text-sm'>
                  {protocol.path}
                </code>
              </CardContent>
            </Card>
          ))}
        </div>
        <CodeSample language='json'>{`{
  "model": "gpt-5.6-sol",
  "messages": [{ "role": "user", "content": "Hello" }],
  "stream": true
}`}</CodeSample>
        <p className='border-warning/25 bg-warning/5 text-muted-foreground rounded-lg border p-4 text-sm leading-6'>
          {t(
            'Route compatibility matters: image generation models such as grok-imagine-image must use /v1/images/generations or /v1/images/edits, not a text-generation endpoint.'
          )}
        </p>
      </section>

      <section id='routing' className={sectionClassName}>
        <div>
          <h2 className='text-2xl font-semibold'>{t('Routing rules')}</h2>
          <p className='text-muted-foreground mt-2 leading-7'>
            {t(
              'Routing is determined by the API key, group, model, request path, and the current channel configuration.'
            )}
          </p>
        </div>
        <ol className='space-y-4'>
          {routingSteps.map((step, index) => (
            <li key={step.title} className='flex gap-4 rounded-lg border p-4'>
              <span className='bg-primary text-primary-foreground flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
                {index + 1}
              </span>
              <div>
                <h3 className='font-semibold'>{t(step.title)}</h3>
                <p className='text-muted-foreground mt-1 text-sm leading-6'>
                  {t(step.description)}
                </p>
              </div>
            </li>
          ))}
        </ol>
      </section>
    </article>
  )
}
