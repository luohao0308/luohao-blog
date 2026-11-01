export interface RevisionDiff { fallback: boolean, lines: { kind: 'same' | 'add' | 'remove', text: string }[] }
// Limit input size, line count and quadratic work before allocating the table.
export function revisionDiff(before: string, after: string): RevisionDiff {
  const fallback = { fallback: true, lines: [] } as RevisionDiff
  if (before.length + after.length > 131072 || new TextEncoder().encode(before + after).length > 131072) return fallback
  const a = before.split('\n'), b = after.split('\n')
  if (a.length > 1000 || b.length > 1000 || (a.length + 1) * (b.length + 1) > 100000) return fallback
  const width = b.length + 1
  const table = new Uint16Array((a.length + 1) * width)
  for (let i = a.length - 1; i >= 0; i--) for (let j = b.length - 1; j >= 0; j--)
    table[i * width + j] = a[i] === b[j] ? table[(i + 1) * width + j + 1]! + 1 : Math.max(table[(i + 1) * width + j]!, table[i * width + j + 1]!)
  const lines: RevisionDiff['lines'] = []
  let i = 0, j = 0
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) { lines.push({ kind: 'same', text: a[i++]! }); j++ }
    else if (j < b.length && (i === a.length || table[i * width + j + 1]! >= table[(i + 1) * width + j]!)) lines.push({ kind: 'add', text: b[j++]! })
    else lines.push({ kind: 'remove', text: a[i++]! })
  }
  return { fallback: false, lines }
}
