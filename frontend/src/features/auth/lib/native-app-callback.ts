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
export function parseNativeAppLoopbackRedirect(raw: string): URL | null {
  try {
    const redirect = new URL(raw)
    const port = Number(redirect.port)
    const isLoopback =
      redirect.hostname === 'localhost' ||
      redirect.hostname === '127.0.0.1' ||
      redirect.hostname === '[::1]'
    if (
      redirect.protocol !== 'http:' ||
      !isLoopback ||
      !Number.isInteger(port) ||
      port < 1024 ||
      port > 65535 ||
      redirect.username ||
      redirect.password ||
      redirect.search ||
      redirect.hash
    ) {
      return null
    }
    return redirect
  } catch {
    return null
  }
}

export function buildNativeAppDeniedRedirect(
  redirectUri: string,
  state: string
): string | null {
  const redirect = parseNativeAppLoopbackRedirect(redirectUri)
  if (!redirect || !/^[A-Za-z0-9._~-]{16,512}$/.test(state)) return null
  redirect.searchParams.set('error', 'access_denied')
  redirect.searchParams.set('state', state)
  return redirect.toString()
}
