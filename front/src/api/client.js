// Общий транспорт для API с JSON-ответами. Адрес Go-контейнера знает прокси.
export async function requestJSON(path, options = {}) {
  const response = await fetch(`/api${path}`, {
    ...options,
    credentials: 'same-origin',
    signal: options.signal ?? AbortSignal.timeout(5000),
  })

  if (!response.ok) {
    const error = new Error('API request failed')
    error.status = response.status
    try { error.code = (await response.json()).error } catch { /* Generic fallback below. */ }
    throw error
  }

  return response.json()
}
