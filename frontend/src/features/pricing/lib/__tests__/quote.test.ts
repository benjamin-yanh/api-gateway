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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../../types'
import { filterPricingModels, getPricingCategory } from '../quote'

const baseModel: PricingModel = {
  id: 1,
  model_name: 'text-model',
  vendor_name: 'Example AI',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 2,
  enable_groups: [],
  supported_endpoint_types: ['openai'],
}

describe('API pricing filters', () => {
  test('classifies media endpoints before text modalities', () => {
    const imageModel: PricingModel = {
      ...baseModel,
      id: 2,
      model_name: 'image-model',
      supported_endpoint_types: ['openai', 'image-generation'],
      output_modalities: ['image'],
    }

    assert.equal(getPricingCategory(imageModel), 'image')
  })

  test('filters by category and provider search in one result set', () => {
    const videoModel: PricingModel = {
      ...baseModel,
      id: 2,
      model_name: 'video-model',
      vendor_name: 'Cinema AI',
      supported_endpoint_types: ['openai-video'],
    }

    assert.deepEqual(
      filterPricingModels([baseModel, videoModel], 'cinema', 'video').map(
        (model) => model.model_name
      ),
      ['video-model']
    )
  })
})
