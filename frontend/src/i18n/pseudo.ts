const interpolation = /(\{[^{}]+\})/g
const accents: Record<string, string> = {
  a: 'à', A: 'À', b: 'ƀ', B: 'Ɓ', c: 'ç', C: 'Ç', d: 'đ', D: 'Đ',
  e: 'ë', E: 'Ë', f: 'ƒ', F: 'Ƒ', g: 'ĝ', G: 'Ĝ', h: 'ĥ', H: 'Ĥ',
  i: 'ï', I: 'Ï', j: 'ĵ', J: 'Ĵ', k: 'ķ', K: 'Ķ', l: 'ĺ', L: 'Ĺ',
  m: 'ṁ', M: 'Ṁ', n: 'ñ', N: 'Ñ', o: 'ö', O: 'Ö', p: 'ṕ', P: 'Ṕ',
  q: 'ɋ', Q: 'Ɋ', r: 'ř', R: 'Ř', s: 'š', S: 'Š', t: 'ţ', T: 'Ţ',
  u: 'ü', U: 'Ü', v: 'ṽ', V: 'Ṽ', w: 'ŵ', W: 'Ŵ', x: 'ẋ', X: 'Ẋ',
  y: 'ÿ', Y: 'Ÿ', z: 'ž', Z: 'Ž',
}

export function pseudoLocalize(message: string): string {
  return message.split('|').map((pluralForm) => {
    const transformed = pluralForm.split(interpolation).map((part) => {
      if (part.startsWith('{') && part.endsWith('}')) return part
      return [...part].map((character) => accents[character] ?? character).join('')
    }).join('')
    return `[«${transformed.trim()}~~~»]`
  }).join(' | ')
}

export function pseudoLocalizeMessages(value: unknown): unknown {
  if (typeof value === 'string') return pseudoLocalize(value)
  if (Array.isArray(value)) return value.map(pseudoLocalizeMessages)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, pseudoLocalizeMessages(item)]))
  }
  return value
}
