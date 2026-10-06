// 统一按东八区展示日期。基础库对 toLocaleDateString 的 options 支持
// 在 iOS 上不稳定，手动用 UTC 偏移拼接。
export function formatDate(ts?: string): string {
  if (!ts) return ''
  const time = new Date(ts).getTime()
  if (Number.isNaN(time)) return ''
  const shifted = new Date(time + 8 * 3600 * 1000)
  const pad = (n: number) => (n < 10 ? `0${n}` : String(n))
  return `${shifted.getUTCFullYear()}-${pad(shifted.getUTCMonth() + 1)}-${pad(shifted.getUTCDate())}`
}
