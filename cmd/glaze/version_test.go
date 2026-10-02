package main

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

// withStamp sets the build stamp for one test and restores it afterwards.
func withStamp(t *testing.T, version, commit, date string) {
	t.Helper()

	saved := [3]string{Version, Commit, Date}
	Version, Commit, Date = version, commit, date
	t.Cleanup(func() { Version, Commit, Date = saved[0], saved[1], saved[2] })
}

// buildInfo returns build info with the main module version and the given VCS settings.
func buildInfo(version string, settings ...debug.BuildSetting) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: version}, Settings: settings}
}

func TestFillFromBuildInfo(t *testing.T) {
	vcs := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "7fd85d757e311d5a6f872f739a63e8b35463c184"},
		{Key: "vcs.time", Value: "2026-10-02T09:55:02Z"},
	}

	t.Run("go install of a release reports its version", func(t *testing.T) {
		withStamp(t, unstampedVersion, unstampedCommit, unstampedDate)

		fillFromBuildInfo(buildInfo("v0.1.7"))
		assert.Equal(t, "v0.1.7", Version)
		assert.Equal(t, unstampedCommit, Commit)
	})

	t.Run("a build in a git checkout reports the commit and its time", func(t *testing.T) {
		withStamp(t, unstampedVersion, unstampedCommit, unstampedDate)

		fillFromBuildInfo(buildInfo("(devel)", vcs...))
		assert.Equal(t, unstampedVersion, Version, "(devel) says nothing, so the version stays dev")
		assert.Equal(t, "7fd85d7", Commit)
		assert.Equal(t, "2026-10-02T09:55:02Z", Date)
	})

	t.Run("the Makefile stamp wins", func(t *testing.T) {
		withStamp(t, "v0.1.6", "1234567", "2026-09-28T00:00:00+1000")

		fillFromBuildInfo(buildInfo("v0.1.7", vcs...))
		assert.Equal(t, "v0.1.6", Version)
		assert.Equal(t, "1234567", Commit)
		assert.Equal(t, "2026-09-28T00:00:00+1000", Date)
	})

	t.Run("no build info changes nothing", func(t *testing.T) {
		withStamp(t, unstampedVersion, unstampedCommit, unstampedDate)

		fillFromBuildInfo(nil)
		assert.Equal(t, unstampedVersion, Version)
	})
}
