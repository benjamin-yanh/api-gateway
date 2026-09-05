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
import { spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { buildSync } from 'esbuild'

// Run the repository's node:test TS/TSX suites using existing dependencies.
// Bundling handles TypeScript and @/ aliases; runtime packages stay external.
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const tests = process.argv.slice(2).map((file) => resolve(root, file))
if (tests.length === 0) {
  console.error(
    'Usage: node scripts/test-with-dom.mjs src/path/__tests__/name.test.tsx [...]'
  )
  process.exit(2)
}

const directory = mkdtempSync(join(tmpdir(), 'new-api-frontend-tests-'))
try {
  symlinkSync(
    join(root, 'node_modules'),
    join(directory, 'node_modules'),
    'dir'
  )
  buildSync({
    absWorkingDir: root,
    entryPoints: tests,
    outdir: join(directory, 'tests'),
    outbase: root,
    outExtension: { '.js': '.mjs' },
    bundle: true,
    platform: 'node',
    format: 'esm',
    jsx: 'automatic',
    packages: 'external',
  })
  const bootstrap = join(directory, 'bootstrap.mjs')
  writeFileSync(
    bootstrap,
    `
import { registerHooks } from 'node:module'
import { Window } from 'happy-dom'
registerHooks({
  load(url, context, nextLoad) {
    if (url.includes('/node_modules/') && url.endsWith('.json')) {
      return nextLoad(url, { ...context, importAttributes: { type: 'json' } })
    }
    return nextLoad(url, context)
  },
  resolve(specifier, context, nextResolve) {
    try { return nextResolve(specifier, context) }
    catch (error) {
      if (error.code === 'ERR_MODULE_NOT_FOUND' && specifier.startsWith('dayjs/')) {
        return nextResolve(specifier + '.js', context)
      }
      // Some UI packages ship extensionless ESM for bundlers.
      if (error.url?.includes('/node_modules/') &&
          ['ERR_UNSUPPORTED_DIR_IMPORT', 'ERR_MODULE_NOT_FOUND'].includes(error.code)) {
        for (const suffix of ['.js', '/index.js']) {
          try { return nextResolve(error.url + suffix, context) } catch {}
        }
      }
      throw error
    }
  }
})
const window = new Window()
globalThis.matchMedia = window.matchMedia.bind(window)
for (const key of ['window', 'document', 'navigator', 'HTMLElement', 'Node', 'Element', 'MutationObserver', 'customElements']) {
  Object.defineProperty(globalThis, key, { configurable: true, value: window[key] })
}
process.on('exit', () => window.close())
`
  )
  const outputs = tests.map((file) =>
    join(
      directory,
      'tests',
      relative(root, file).replace(/\.[cm]?[jt]sx?$/, '.mjs')
    )
  )
  // Stop after completed suites instead of waiting for library GC timers.
  const result = spawnSync(
    process.execPath,
    ['--import', bootstrap, '--test-force-exit', '--test-timeout=60000', '--test', ...outputs],
    {
      cwd: root,
      stdio: 'inherit',
    }
  )
  process.exitCode = result.status ?? 1
} finally {
  rmSync(directory, { recursive: true, force: true })
}
