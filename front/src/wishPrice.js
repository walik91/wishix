const pricePattern = /^(0|[1-9][0-9]{0,9})(\.[0-9]{1,2})?$/
export const currencyPattern = /^[A-Z]{3}$/

export function validateWishPrice(amount, code) {
  const price = amount.trim().replace(',', '.')
  if (!price) return { price: null, currency: '', errors: {} }

  const currency = code.trim().toUpperCase()
  const errors = {}
  if (!pricePattern.test(price)) errors.price = 'wish_price_invalid'
  if (!currency) errors.currency = 'wish_currency_required'
  else if (!currencyPattern.test(currency)) errors.currency = 'wish_currency_invalid'
  return { price, currency, errors }
}

function currencyStorageKey(userID) {
  return `wishix.wish-currency.${userID}`
}

export function readWishCurrency(userID) {
  if (!userID) return ''
  try {
    const code = localStorage.getItem(currencyStorageKey(userID))
    return currencyPattern.test(code) ? code : ''
  } catch {
    return ''
  }
}

export function rememberWishCurrency(userID, code) {
  if (!userID || !currencyPattern.test(code)) return
  try {
    localStorage.setItem(currencyStorageKey(userID), code)
  } catch { /* The form still works when storage is unavailable. */ }
}
