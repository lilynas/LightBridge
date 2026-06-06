import { describe, expect, it } from 'vitest'
import { versionedRemoteEntry } from '../remoteEntry'

describe('versionedRemoteEntry', () => {
  it('appends module version to a remote entry without query', () => {
    expect(versionedRemoteEntry('/modules/mock/remoteEntry.js', '1.2.3'))
      .toBe('/modules/mock/remoteEntry.js?v=1.2.3')
  })

  it('appends module version to an existing query string', () => {
    expect(versionedRemoteEntry('/modules/mock/remoteEntry.js?asset=remote', '1.2.3'))
      .toBe('/modules/mock/remoteEntry.js?asset=remote&v=1.2.3')
  })

  it('keeps hash fragments after the version query parameter', () => {
    expect(versionedRemoteEntry('/modules/mock/remoteEntry.js#chunk', '1.2.3'))
      .toBe('/modules/mock/remoteEntry.js?v=1.2.3#chunk')
  })

  it('preserves existing query strings and hash fragments', () => {
    expect(versionedRemoteEntry('/modules/mock/remoteEntry.js?asset=remote#chunk', '1.2.3'))
      .toBe('/modules/mock/remoteEntry.js?asset=remote&v=1.2.3#chunk')
  })

  it('leaves the remote entry unchanged when module version is missing', () => {
    expect(versionedRemoteEntry('/modules/mock/remoteEntry.js?asset=remote#chunk'))
      .toBe('/modules/mock/remoteEntry.js?asset=remote#chunk')
  })
})
