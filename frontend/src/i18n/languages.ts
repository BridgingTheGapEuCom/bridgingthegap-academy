export type LanguageOption = { tag: string; label: string; search: string }

// A practical discovery set, not an Academy allow-list. The API remains the
// authority for every syntactically valid canonical BCP 47 tag.
export const commonLanguageTags = [
  'ar', 'cs', 'da', 'de', 'el', 'en', 'en-GB', 'en-US', 'es', 'es-ES', 'fi', 'fr', 'fr-CA',
  'he', 'hi', 'id', 'it', 'ja', 'ko', 'nl', 'no', 'pl', 'pt', 'pt-BR', 'ro', 'ru', 'sv', 'tr',
  'uk', 'vi', 'zh-Hans', 'zh-Hant',
] as const

function displayName(type: Intl.DisplayNamesType, value: string): string | undefined {
  try {
    return new Intl.DisplayNames(undefined, { type }).of(value) ?? undefined
  } catch {
    return undefined
  }
}

export function canonicalLanguageTag(value: string): string | undefined {
  const trimmed = value.trim()
  if (!trimmed || trimmed.length > 64 || !/^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$/.test(trimmed)) return undefined
  try {
    const tag = new Intl.Locale(trimmed).toString()
    const inputBase = trimmed.split('-')[0]!
    const canonicalBase = tag.split('-')[0]!
    if ((tag === 'und' && trimmed.toLowerCase() !== 'und') || (inputBase.length === 3 && inputBase.toLowerCase() !== canonicalBase.toLowerCase())) return undefined
    return tag
  } catch {
    return undefined
  }
}

export function languageLabel(value: string): string {
  const tag = canonicalLanguageTag(value) ?? value
  try {
    const locale = new Intl.Locale(tag)
    const language = displayName('language', locale.language)
    if (!language) return tag
    const script = locale.script ? displayName('script', locale.script) : undefined
    const region = locale.region ? displayName('region', locale.region) : undefined
    const qualifier = [script, region].filter(Boolean).join(' — ')
    return qualifier ? `${language} — ${qualifier} (${tag})` : `${language} (${tag})`
  } catch {
    return tag
  }
}

export const commonLanguageOptions: LanguageOption[] = commonLanguageTags.map((tag) => ({
  tag,
  label: languageLabel(tag),
  search: `${tag} ${languageLabel(tag)}`.toLocaleLowerCase(),
}))

export function filterLanguageOptions(query: string): LanguageOption[] {
  const search = query.trim().toLocaleLowerCase()
  return search ? commonLanguageOptions.filter((option) => option.search.includes(search)) : commonLanguageOptions
}
