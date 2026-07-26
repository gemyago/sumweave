export function documentTitle(currentPage?: string, section?: string): string {
  return [currentPage, section, 'Signal Foundry'].filter(Boolean).join(' · ')
}
