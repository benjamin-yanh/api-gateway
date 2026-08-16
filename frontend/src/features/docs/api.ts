/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

export interface AvailableModel {
  id: string
  owned_by?: string
}

interface ModelsResponse {
  data?: AvailableModel[]
}

export async function getAvailableModels(): Promise<AvailableModel[]> {
  const response = await api.get<ModelsResponse>('/v1/models', {
    skipErrorHandler: true,
  })

  return (response.data.data ?? [])
    .filter((model) => typeof model.id === 'string' && model.id.length > 0)
    .sort((left, right) => left.id.localeCompare(right.id))
}
