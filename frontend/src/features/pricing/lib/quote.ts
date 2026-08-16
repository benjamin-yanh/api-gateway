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
import type { PricingModel } from '../types'

export const PRICING_CATEGORIES = [
  'all',
  'text',
  'image',
  'audio',
  'video',
  'embeddings',
  'other',
] as const

export type PricingCategory = (typeof PRICING_CATEGORIES)[number]

const TEXT_ENDPOINTS = new Set([
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
])

export function getPricingCategory(model: PricingModel): PricingCategory {
  const endpoints = new Set(model.supported_endpoint_types ?? [])
  const modalities = new Set([
    ...(model.input_modalities ?? []),
    ...(model.output_modalities ?? []),
  ])

  if (endpoints.has('openai-video') || modalities.has('video')) {
    return 'video'
  }
  if (endpoints.has('image-generation') || modalities.has('image')) {
    return 'image'
  }
  if (endpoints.has('embeddings')) return 'embeddings'
  if (modalities.has('audio')) return 'audio'
  for (const endpoint of endpoints) {
    if (TEXT_ENDPOINTS.has(endpoint)) return 'text'
  }
  if (modalities.has('text')) return 'text'
  return 'other'
}

export function filterPricingModels(
  models: PricingModel[],
  search: string,
  category: PricingCategory
): PricingModel[] {
  const normalizedSearch = search.trim().toLocaleLowerCase()

  return models.filter((model) => {
    if (category !== 'all' && getPricingCategory(model) !== category) {
      return false
    }
    if (!normalizedSearch) return true

    return [model.model_name, model.vendor_name, model.description].some(
      (value) => value?.toLocaleLowerCase().includes(normalizedSearch)
    )
  })
}
