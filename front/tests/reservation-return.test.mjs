import assert from 'node:assert/strict'
import { consumeReservationReturn, rememberReservationReturn } from '../src/reservationReturn.js'

const data = new Map()
globalThis.sessionStorage = {
  setItem(key, value) { data.set(key, value) },
  getItem(key) { return data.get(key) ?? null },
  removeItem(key) { data.delete(key) },
}

const list = `/u/person/wishlist/00000000-0000-4000-8000-000000000001/${'a'.repeat(43)}`
rememberReservationReturn(list)
assert.equal(consumeReservationReturn(), list)
assert.equal(consumeReservationReturn(), '/my', 'return path is single-use')
const wishID = '00000000-0000-4000-8000-000000000002'
rememberReservationReturn(list, wishID)
assert.equal(consumeReservationReturn(), `${list}#reserve-${wishID}`)
rememberReservationReturn(list, 'not-a-wish')
assert.equal(consumeReservationReturn(), list, 'invalid wish IDs must not enter the redirect')
rememberReservationReturn('/u/person/wishes')
assert.equal(consumeReservationReturn(), '/u/person/wishes')
rememberReservationReturn('https://attacker.example/path')
assert.equal(consumeReservationReturn(), '/my')
data.set('wishix_reservation_return', JSON.stringify({ path: '//attacker.example', expires: Date.now() + 60000 }))
assert.equal(consumeReservationReturn(), '/my', 'reject open redirects even if storage is altered')
data.set('wishix_reservation_return', JSON.stringify({ path: list, expires: Date.now() - 1 }))
assert.equal(consumeReservationReturn(), '/my', 'expired return paths must not be used')
console.log('Passed: reservation return path, expiry and redirect validation.')
