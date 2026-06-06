export function versionedRemoteEntry(remoteEntry: string, moduleVersion?: string): string {
  if (!moduleVersion) return remoteEntry
  const hashIndex = remoteEntry.indexOf('#')
  const base = hashIndex >= 0 ? remoteEntry.slice(0, hashIndex) : remoteEntry
  const hash = hashIndex >= 0 ? remoteEntry.slice(hashIndex) : ''
  const separator = base.includes('?') ? '&' : '?'
  return `${base}${separator}v=${encodeURIComponent(moduleVersion)}${hash}`
}
