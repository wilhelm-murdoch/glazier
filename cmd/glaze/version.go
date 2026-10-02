package main

import "runtime/debug"

const (
	// unstampedVersion, unstampedCommit and unstampedDate are the values of a build that the Makefile did not stamp.
	unstampedVersion = "dev"
	unstampedCommit  = "none"
	unstampedDate    = "unknown"

	// shortCommitLength is the length of a commit hash in the version line, the same as `git rev-parse --short`.
	shortCommitLength = 7
)

// fillFromBuildInfo sets the version, commit and date that the Makefile did not stamp, from the build info that Go records.
// `go install module@v1.2.3` records the version, and a build in a git checkout records the commit and its time.
func fillFromBuildInfo(info *debug.BuildInfo) {
	if info == nil {
		return
	}

	if Version == unstampedVersion && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}

	for _, setting := range info.Settings {
		switch {
		case setting.Key == "vcs.revision" && Commit == unstampedCommit:
			Commit = setting.Value[:min(shortCommitLength, len(setting.Value))]
		case setting.Key == "vcs.time" && Date == unstampedDate:
			Date = setting.Value
		}
	}
}
