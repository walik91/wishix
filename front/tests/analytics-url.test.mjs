import assert from 'node:assert/strict'
import { analyticsLocation } from '../src/analytics-url.js'

const origin = 'https://wishix.example'
const list = '12345678-1234-4234-8234-123456789abc'
const token = 'a'.repeat(43)
const clean = path => analyticsLocation(new URL(origin + path))

assert.equal(clean('/'), origin + '/')
assert.equal(clean('/login?reset=1&utm_source=example'), origin + '/login?reset=1&utm_source=example')
assert.equal(clean('/my/lists/' + list + '?notice=deleted'), origin + '/my/lists/' + list + '?notice=deleted')
assert.equal(clean('/u/anna/wishlist/' + list + '/' + token), origin + '/u/anna/wishlist/' + list)
assert.equal(clean('/u/anna/wishlist/' + list), origin + '/u/anna/wishlist/' + list)
assert.equal(clean('/u/anna/wishes'), origin + '/u/anna/wishes')
assert.equal(clean('/login?email=private%40example.com&code=918273&token=secret&state=secret&notice=ok'), origin + '/login?notice=ok')
assert.equal(clean('/login?ref=private%2540example.com&next=%2Flogin%3Fcode%3D918273&safe=ok'), origin + '/login?safe=ok')
assert.equal(clean('/login?ref=' + token + '&safe=ok'), origin + '/login?safe=ok')
assert.equal(clean('/u/anna/wishlist/' + list + '/' + token + '#reserve-' + list), origin + '/u/anna/wishlist/' + list + '#reserve-' + list)
assert.equal(clean('/login#' + token), origin + '/login')
assert.equal(clean('/unexpected/' + token), origin + '/other')
console.log('Passed: actual routes, list IDs, usernames and sensitive URL filtering.')
