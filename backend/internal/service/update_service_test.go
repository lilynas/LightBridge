//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release  *GitHubRelease
	releases []GitHubRelease
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) FetchReleases(context.Context, string, int) ([]GitHubRelease, error) {
	if s.releases != nil {
		return s.releases, nil
	}
	if s.release == nil {
		return nil, nil
	}
	return []GitHubRelease{*s.release}, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServiceListVersionReleasesIncludesHistory(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.1",
				Name:    "v0.1.1",
				Body:    "latest",
			},
			releases: []GitHubRelease{
				{
					TagName:     "v0.1.1",
					Name:        "v0.1.1",
					Body:        "latest",
					PublishedAt: "2026-06-07T00:00:00Z",
					HTMLURL:     "https://example.test/releases/v0.1.1",
				},
				{
					TagName:     "v0.0.1",
					Name:        "v0.0.1",
					Body:        "first",
					PublishedAt: "2026-05-01T00:00:00Z",
					HTMLURL:     "https://example.test/releases/v0.0.1",
				},
			},
		},
		"0.0.1",
		"release",
	)

	releases, info, err := svc.ListVersionReleases(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, "0.1.1", info.LatestVersion)
	require.Len(t, releases, 2)
	require.Equal(t, "0.1.1", releases[0].Version)
	require.True(t, releases[0].Latest)
	require.False(t, releases[0].Current)
	require.Equal(t, "0.0.1", releases[1].Version)
	require.True(t, releases[1].Current)
	require.False(t, releases[1].Latest)
}
