// Package harness runs the glaze binary against real tmux servers and checks
// the result through tmux itself. It never imports glaze, so a bug in glaze
// cannot hide itself in a check.
//
// A case is a function that harness.Run calls with a new Case:
//
//	harness.Run(t, "dirs_relative", func(c *harness.Case) {
//		c.Fixture("directories/relative.glaze", "prof/.glaze")
//		c.OK(c.Up("--profile-path", "prof/.glaze"), "up")
//		c.EventuallyEqual("pane directory", c.Path("prof/sub"), func() string { return c.PanePaths("=dr:") })
//	})
//
// The files of the package follow the steps of a case:
//
//   - case.go: the Case, its isolation and its cleanup.
//   - files.go: the work directory, fixtures and profiles.
//   - glaze.go and exec.go: glaze and any other command, with a deadline.
//   - control.go and terminal.go: an attached tmux client and a pseudo-terminal.
//   - tmux.go: queries that read the state back from tmux.
//   - check.go and golden.go: the checks.
//   - env.go, baseline.go and report.go: the run as a whole.
//
// Every check takes a label first. The label names the check in the report and
// in expected-failures.txt, so it must be unique in its case and the same on
// every target: never put a path, a process id, a time or a tmux version in it.
package harness
