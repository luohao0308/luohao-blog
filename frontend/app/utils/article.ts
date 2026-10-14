export interface ArticleHeading {
  id: string
  level: 2 | 3
  text: string
}

const namedEntities: Record<string, string> = {
  amp: '&',
  apos: '\'',
  gt: '>',
  lt: '<',
  quot: '"',
}

function decodeEntities(value: string): string {
  return value.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (entity, key: string) => {
    if (key.startsWith('#')) {
      const point = Number.parseInt(key.slice(key[1]?.toLowerCase() === 'x' ? 2 : 1), key[1]?.toLowerCase() === 'x' ? 16 : 10)
      return Number.isInteger(point) && point >= 0 && point <= 0x10FFFF ? String.fromCodePoint(point) : entity
    }
    return namedEntities[key.toLowerCase()] ?? entity
  })
}

function headingText(html: string): string {
  return decodeEntities(html.replace(/<[^>]*>/g, '')).replace(/\s+/g, ' ').trim()
}

function headingBase(text: string): string {
  return text
    .normalize('NFKC')
    .toLocaleLowerCase()
    .replace(/[^\p{Letter}\p{Number}]+/gu, '-')
    .replace(/^-+|-+$/g, '') || 'section'
}

export function prepareArticleHtml(source: string): { html: string, headings: ArticleHeading[] } {
  const headings: ArticleHeading[] = []
  const occurrences = new Map<string, number>()
  const usedIDs = new Set<string>()
  const html = source.replace(/<h([23])([^>]*)>([\s\S]*?)<\/h\1>/gi, (_match, rawLevel: string, rawAttrs: string, content: string) => {
    const text = headingText(content)
    if (!text)
      return _match
    // Goldmark already emits deterministic IDs. Preserve them so authored
    // in-article links keep working; only generate one for older cached HTML.
    const existingID = rawAttrs.match(/\s+id\s*=\s*(["'])(.*?)\1/i)?.[2]?.trim()
    const base = existingID || headingBase(text)
    let occurrence = (occurrences.get(base) ?? 0) + 1
    let id = occurrence === 1 ? base : `${base}-${occurrence}`
    while (usedIDs.has(id)) {
      occurrence++
      id = `${base}-${occurrence}`
    }
    occurrences.set(base, occurrence)
    usedIDs.add(id)
    const level = Number(rawLevel) as 2 | 3
    headings.push({ id, level, text })
    const attrs = rawAttrs.replace(/\s+id\s*=\s*(["']).*?\1/i, '')
    return `<h${level}${attrs} id="${id}">${content}</h${level}>`
  })
  return { html, headings }
}

export function readingMinutes(markdown?: string | null): number {
  const source = markdown?.trim() ?? ''
  if (!source)
    return 0
  const prose = source
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`[^`]*`/g, ' ')
    .replace(/!?(?:\[[^\]]*\])\([^)]*\)/g, ' ')
    .replace(/https?:\/\/\S+/g, ' ')
  const han = prose.match(/\p{Script=Han}/gu)?.length ?? 0
  const words = prose.replace(/\p{Script=Han}/gu, ' ').match(/[\p{Letter}\p{Number}]+/gu)?.length ?? 0
  return Math.max(1, Math.ceil(han / 400 + words / 200))
}

export function articleSummary(summary?: string | null, markdown?: string | null, limit = 160): string {
  const explicit = summary?.trim()
  if (explicit)
    return explicit
  const plain = (markdown ?? '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^[#>*+-]+\s*/gm, '')
    .replace(/[`_*~]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
  return plain.length > limit ? `${plain.slice(0, limit).trimEnd()}…` : plain
}
