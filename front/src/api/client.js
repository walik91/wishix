// Общий транспорт для API с JSON-ответами. Адрес Go-контейнера знает прокси.
export async function requestJSON(path, options = {}) {
  const response = await fetch(`/api${path}`, {
    ...options,
    signal: options.signal ?? AbortSignal.timeout(5000),
  })

  if (!response.ok) {
    const error = new Error('API request failed')
    error.status = response.status
    throw error
  }

  return response.json()
}
