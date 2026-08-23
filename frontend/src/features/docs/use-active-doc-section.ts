/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { startTransition, useEffect, useState } from 'react'

const sectionIds = [
  'overview',
  'getting-started',
  'desktop-clients',
  'cursor',
  'models',
  'protocols',
  'routing',
] as const

export function useActiveDocSection() {
  const [activeSection, setActiveSection] = useState<string>(sectionIds[0])

  useEffect(() => {
    let animationFrame = 0

    const updateActiveSection = () => {
      animationFrame = 0
      const anchorPosition = window.scrollY + 112
      let nextSection: string = sectionIds[0]

      for (const sectionId of sectionIds) {
        const section = document.querySelector<HTMLElement>(`#${sectionId}`)
        const sectionPosition = section
          ? section.getBoundingClientRect().top + window.scrollY
          : undefined
        if (sectionPosition === undefined || sectionPosition > anchorPosition) {
          break
        }
        nextSection = sectionId
      }

      startTransition(() => setActiveSection(nextSection))
    }

    const scheduleUpdate = () => {
      if (animationFrame) return
      animationFrame = window.requestAnimationFrame(updateActiveSection)
    }

    updateActiveSection()
    window.addEventListener('scroll', scheduleUpdate, { passive: true })
    window.addEventListener('resize', scheduleUpdate)

    return () => {
      window.removeEventListener('scroll', scheduleUpdate)
      window.removeEventListener('resize', scheduleUpdate)
      if (animationFrame) window.cancelAnimationFrame(animationFrame)
    }
  }, [])

  return activeSection
}
