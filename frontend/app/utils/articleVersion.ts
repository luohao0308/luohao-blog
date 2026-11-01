// Preserve uint64 protobuf versions without rounding through JavaScript numbers.
export function articleVersion(value: unknown): string {
  const text = typeof value === 'string' ? value : typeof value === 'number' && Number.isSafeInteger(value) ? String(value) : ''
  return /^[1-9]\d*$/.test(text) && BigInt(text) <= 18446744073709551615n ? text : ''
}
