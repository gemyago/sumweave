export function documentTitle(currentPage?: string, section?: string): string {
  return [currentPage, section, 'Sumweave'].filter(Boolean).join(' · ')
}
