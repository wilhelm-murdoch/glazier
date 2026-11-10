package cases

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// registryCommands are the subcommands that take a profile name.
var registryCommands = []string{"up", "down", "format"}

// invalidProfileNames are the words that glaze rejects as a profile name before it reads anything.
var invalidProfileNames = []string{"./work", "work/", "a/b", "../work", "/abs", "..", ".", ".hidden", "-1", "work.glaze", "~/work", "a b", "a:b", "a;b", "$work", ""}

// TestRegistry checks the profile registry: a name after up, down and format selects <name>.glaze in it.
func TestRegistry(t *testing.T) {
	harness.Run(t, "reg_up_name", func(c *harness.Case) {
		c.Simple("reg-home", registryHome+"/work.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		c.OK(c.Up("work"), "up by name from the home registry")
		c.SessionExists("home registry session created", "reg-home")
		c.SessionGone("the registry name is not the session name", "work")
		c.Must(c.Down("work"), "down")
		c.OK(c.Glaze("up", "work", "--detached", "--socket-name", c.Socket), "up with the name before the flags")
		c.SessionExists("name before the flags creates the session", "reg-home")
	})

	harness.Run(t, "reg_up_xdg", func(c *harness.Case) {
		c.Simple("reg-xdg", "xdg/glaze/work.glaze")
		c.Simple("reg-home-loses", registryHome+"/work.glaze")
		c.OK(c.UpWith(xdgConfigHome(c.Path("xdg")), "work"), "up by name with XDG_CONFIG_HOME")
		c.SessionExists("XDG_CONFIG_HOME registry used", "reg-xdg")
		c.SessionGone("home registry not used when XDG_CONFIG_HOME is set", "reg-home-loses")
	})

	// The XDG base directory specification says to ignore a relative XDG_CONFIG_HOME.
	harness.Run(t, "reg_up_xdg_relative", func(c *harness.Case) {
		c.Simple("reg-rel-home", registryHome+"/work.glaze")
		c.Simple("reg-rel-loses", "rel/glaze/work.glaze")
		r := c.UpWith(xdgConfigHome("rel"), "work")
		c.OK(r, "up by name with a relative XDG_CONFIG_HOME")
		c.Match("a relative XDG_CONFIG_HOME warns", "XDG_CONFIG_HOME is not absolute", r.Stderr)
		c.SessionExists("a relative XDG_CONFIG_HOME is ignored", "reg-rel-home")
		c.SessionGone("relative XDG_CONFIG_HOME directory not used", "reg-rel-loses")
		r = c.UpWith(xdgConfigHome("rel"), "nope")
		c.ExitCode(r, "unknown name with a relative XDG_CONFIG_HOME exits 3", exitInvalidProfile)
		c.Match("unknown name with a relative XDG_CONFIG_HOME still warns", "XDG_CONFIG_HOME is not absolute", r.Stderr)
	})

	harness.Run(t, "reg_up_glaze_registry", func(c *harness.Case) {
		c.Simple("reg-env", "reg/work.glaze")
		c.Simple("reg-xdg-loses", "xdg/glaze/work.glaze")
		c.Simple("reg-home-loses", registryHome+"/work.glaze")
		both := harness.Opts{Env: []string{"GLAZE_REGISTRY=" + c.Path("reg"), "XDG_CONFIG_HOME=" + c.Path("xdg")}}
		c.OK(c.UpWith(both, "work"), "up by name with GLAZE_REGISTRY")
		c.SessionExists("GLAZE_REGISTRY beats XDG_CONFIG_HOME", "reg-env")
		c.SessionGone("XDG_CONFIG_HOME not used when GLAZE_REGISTRY is set", "reg-xdg-loses")
		c.SessionGone("home registry not used when GLAZE_REGISTRY is set", "reg-home-loses")
		c.Simple("reg-tilde", "home/reg2/work.glaze")
		c.OK(c.UpWith(glazeRegistry("~/reg2"), "work"), "GLAZE_REGISTRY with ~")
		c.SessionExists("GLAZE_REGISTRY ~ session created", "reg-tilde")
	})

	// A name never reads the current directory, so a relative GLAZE_REGISTRY has no base and is an error.
	harness.Run(t, "reg_glaze_registry_relative", func(c *harness.Case) {
		c.Simple("reg-rel", registryHome+"/work.glaze")
		r := c.UpWith(glazeRegistry("rel"), "work")
		c.ExitCode(r, "a relative GLAZE_REGISTRY exits 3", exitInvalidProfile)
		c.Match("a relative GLAZE_REGISTRY names the fix", "absolute", r.Stderr)
		c.NoServer("a relative GLAZE_REGISTRY")
		c.SessionGone("home registry not used with a relative GLAZE_REGISTRY", "reg-rel")
		r = c.GlazeWith(glazeRegistry("rel"), "down", "--socket-name", c.Socket, "work")
		c.ExitCode(r, "down with a relative GLAZE_REGISTRY exits 3", exitInvalidProfile)
		c.Match("down with a relative GLAZE_REGISTRY names --session", "--session", r.Stderr)
	})

	harness.Run(t, "reg_name_beats_everything", func(c *harness.Case) {
		c.Simple("cwd-loses")
		c.Simple("local-loses", "work.glaze")
		c.Simple("gp-loses", "gp/.glaze")
		c.Simple("reg-wins", registryHome+"/work.glaze")
		c.OK(c.UpWith(glazePath(c.Path("gp")), "work"), "up with a name, .glaze in cwd, work.glaze in cwd and GLAZE_PATH")
		c.SessionExists("a name uses the registry", "reg-wins")
		c.SessionGone("cwd .glaze not used with a name", "cwd-loses")
		c.SessionGone("work.glaze in cwd not used with a name", "local-loses")
		c.SessionGone("GLAZE_PATH not used with a name", "gp-loses")
	})

	harness.Run(t, "reg_no_name_keeps_old_search", func(c *harness.Case) {
		c.Simple("reg-present", registryHome+"/work.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		r := c.Up()
		c.ExitCode(r, "bare up with a registry present exits 3", exitInvalidProfile)
		c.Match("bare up message still lists the three places", `--profile-path(.|\n)*current directory(.|\n)*GLAZE_PATH`, r.Stderr)
		c.Match("bare up message says how to use the registry", "ls --profiles", r.Stderr)
		c.NoServer("bare up with a registry present")
	})

	harness.Run(t, "reg_unknown_name", func(c *harness.Case) {
		c.Simple("reg-present", registryHome+"/work.glaze")
		c.Simple("cwd-nope", "nope.glaze")
		r := c.Up("nope")
		c.ExitCode(r, "unknown name exits 3", exitInvalidProfile)
		c.Match("unknown name names the file it looked for", `glaze/nope\.glaze`, r.Stderr)
		c.Match("unknown name names the registry directory", `\.config/glaze`, r.Stderr)
		c.Match("unknown name points at ls --profiles", "ls --profiles", r.Stderr)
		c.Match("unknown name points at save", "glaze save nope", r.Stderr)
		c.Match("unknown name hints at the file in the current directory", `--profile-path nope\.glaze`, r.Stderr)
		c.SessionGone("unknown name does not start the file in the current directory", "cwd-nope")
		c.NoServer("unknown name")
	})

	// The registry exists but is empty, so the message is about the missing profile and not about the missing registry.
	harness.Run(t, "reg_glaze_path_hint", func(c *harness.Case) {
		c.Mkdir(registryHome)
		c.Simple("gp-hint", "gp/work.glaze")
		r := c.UpWith(glazePath(c.Path("gp")), "work")
		c.ExitCode(r, "a file in GLAZE_PATH is not a registry profile (exit 3)", exitInvalidProfile)
		c.Match("the miss names the file in GLAZE_PATH", `gp/work\.glaze`, r.Stderr)
		c.Match("the miss says GLAZE_PATH is not the registry", "not the profile registry", r.Stderr)
		c.NoServer("a file in GLAZE_PATH")
		c.SessionGone("GLAZE_PATH file not started by name", "gp-hint")
	})

	harness.Run(t, "reg_no_registry_dir", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Cd("empty")
		r := c.Up("work")
		c.ExitCode(r, "name without a registry directory exits 3", exitInvalidProfile)
		c.Match("the message says the registry does not exist", "does not exist", r.Stderr)
		c.Match("the message says how to create the registry", "mkdir -p", r.Stderr)
		c.Match("the missing registry message points at save", "glaze save work", r.Stderr)
		c.True("up does not create the registry directory", !c.Exists(registryHome), "%s exists", registryHome)
		c.NoServer("name without a registry directory")
	})

	// Each word is rejected before glaze reads anything, so a name cannot reach a file outside the registry.
	harness.Run(t, "reg_name_shapes", func(c *harness.Case) {
		c.Simple("escaped", "home/.config/work.glaze")
		c.Simple("reg-present", registryHome+"/work.glaze")
		results := map[string]*harness.Result{}
		for _, name := range invalidProfileNames {
			r := c.Up(name)
			results[name] = r
			c.ExitCode(r, fmt.Sprintf("name [%s] is a usage error (exit 2)", name), exitUsage)
			c.NoServer(fmt.Sprintf("name [%s]", name))
		}

		c.SessionGone("a name cannot leave the registry", "escaped")
		c.Match("the .glaze suffix message gives the name", "the name is `work`", results["work.glaze"].Stderr)
		c.Match("the .glaze suffix message gives the --profile-path form", `--profile-path work\.glaze`, results["work.glaze"].Stderr)
		c.Match("a name with a slash says to use --profile-path", "--profile-path", results["a/b"].Stderr)
		c.Match("a leading dash is named", "cannot start with `-`", results["-1"].Stderr)
	})

	harness.Run(t, "reg_second_word_and_conflicts", func(c *harness.Case) {
		c.Simple("reg-present", registryHome+"/work.glaze")
		c.Simple("other", "other.glaze")
		r := c.Up("work", "extra")
		c.ExitCode(r, "a second word is a usage error (exit 2)", exitUsage)
		c.Match("the second word is named", "`extra`", r.Stderr)
		r = c.Up("work", "--profile-path", "other.glaze")
		c.ExitCode(r, "a name and --profile-path together is a usage error (exit 2)", exitUsage)
		c.Match("a name and --profile-path message says not both", "not both", r.Stderr)
		c.ExitCode(c.Ls("work"), "ls with a word is a usage error (exit 2)", exitUsage)
		c.ExitCode(c.Save("work", "--profile-path", "x.glaze"), "save with a name and --profile-path together is a usage error (exit 2)", exitUsage)
		c.NoServer("bad arguments")
	})

	harness.Run(t, "reg_down_name", func(c *harness.Case) {
		c.Simple("reg-down", registryHome+"/work.glaze")
		c.TmuxSetup("new-session", "-d", "-s", "bystander")
		c.Must(c.Up("work"), "up by name")
		c.OK(c.Down("work"), "down by name")
		c.SessionGone("down by name kills the session that the profile names", "reg-down")
		c.SessionExists("down by name leaves other sessions", "bystander")
		r := c.Down("work")
		c.OK(r, "down by name again is a no-op")
		c.Match("down again says nothing to do", "session is not running session=reg-down", r.Stderr)
		r = c.Down("reg-down")
		c.ExitCode(r, "down by the session name is not a registry name (exit 3)", exitInvalidProfile)
		c.Match("down by an unknown name mentions --session", "--session", r.Stderr)
		c.ExitCode(c.Down("work", "--session", "bystander"), "down with a name and --session together is a usage error (exit 2)", exitUsage)
		c.SessionExists("bystander survives the usage error", "bystander")
		c.OK(c.Down("--session", "bystander"), "down --session still kills by tmux name")
		c.SessionGone("bystander gone through --session", "bystander")
	})

	harness.Run(t, "reg_down_deleted_profile", func(c *harness.Case) {
		c.Simple("reg-del", registryHome+"/work.glaze")
		c.Must(c.Up("work"), "up by name")
		c.Remove(registryHome + "/work.glaze")
		r := c.Down("work")
		c.ExitCode(r, "down by name after the profile was deleted exits 3", exitInvalidProfile)
		c.Match("deleted profile message names --session", "--session", r.Stderr)
		c.SessionExists("deleted profile session survives", "reg-del")
		c.OK(c.Down("--session", "reg-del"), "down --session still works")
		c.SessionGone("session gone through --session", "reg-del")
	})

	// A --var-file is read from the current directory and not from the registry.
	harness.Run(t, "reg_name_vars", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze", registryHome+"/gig.glaze")
		c.OK(c.Up("gig", "--var", "fixer=x", "--var", "district=pacifica"), "up by name with --var after the name")
		c.SessionExists("var session created", "gig-pacifica")
		c.Match("down by name without --var uses the default name", "session is not running session=gig-watson", c.Down("gig").Stderr)
		c.OK(c.Down("--var", "district=pacifica", "gig"), "down by name with --var before the name")
		c.SessionGone("var session gone", "gig-pacifica")
		c.Write("elsewhere/vars.hcl", "fixer = \"y\"\ndistrict = \"pacifica\"\n")
		c.Cd("elsewhere")
		c.OK(c.Up("gig", "--var-file", "vars.hcl"), "up by name with --var-file from the current directory")
		c.SessionExists("var-file session created", "gig-pacifica")
	})

	// A registry profile is reached by a word that looks like a session name, so glaze refuses one whose session has no name.
	harness.Run(t, "reg_no_session_name", func(c *harness.Case) {
		c.Fixture("structure/no-names.glaze", registryHome+"/anon.glaze")
		r := c.Up("anon")
		c.ExitCode(r, "up by name without a session name is an invalid profile (exit 3)", exitInvalidProfile)
		c.Match("the missing name diagnostic says the session needs a name", "Missing session name", r.Stderr)
		c.Match("the missing name diagnostic names the profile", "`anon`", r.Stderr)
		c.NoServer("up by name without a session name")
		c.ExitCode(c.Down("anon"), "down by name without a session name is an invalid profile (exit 3)", exitInvalidProfile)
		c.ExitCode(c.Glaze("format", "--validate", "anon"), "format --validate by name without a session name is an invalid profile (exit 3)", exitInvalidProfile)
		c.OK(c.Glaze("format", "--stdout", "anon"), "format --stdout by name without a session name")
		c.OK(c.Up("--profile-path", registryHome+"/anon.glaze"), "the same file by path needs no session name")
		c.SessionExists("the session by path is named default", "default")
	})

	// The leaf commands have no help subcommand, so a profile can be named help.
	harness.Run(t, "reg_help_name", func(c *harness.Case) {
		c.Simple("reg-help", registryHome+"/help.glaze")
		c.OK(c.Up("help"), "up help is a registry lookup")
		c.SessionExists("help profile session created", "reg-help")
		r := c.Glaze("up", "--help")
		c.OK(r, "up --help still prints the help")
		c.Match("up --help shows the name argument", `USAGE:\s+glaze up \[options\] \[name\]`, r.Stdout)
		c.Match("up --help describes the profile registry", "profile registry", r.Stdout)
		c.Match("help up still prints the help", "USAGE", c.Glaze("help", "up").Stdout)
		r = c.Glaze("save", "--help")
		c.OK(r, "save --help prints the help")
		c.Match("save --help shows the name argument", `USAGE:\s+glaze save \[options\] \[name\]`, r.Stdout)
		c.Match("save --help describes the write into the profile registry", "A name writes <name>.glaze into the profile registry", r.Stdout)
		c.Match("save --help describes --force for any target", "replaces the target profile when it exists", r.Stdout)
	})

	harness.Run(t, "reg_name_dot_inside", func(c *harness.Case) {
		c.Simple("reg-dot", registryHome+"/api.v2.glaze")
		c.OK(c.Up("api.v2"), "a dot inside a name is accepted")
		c.SessionExists("dotted name session created", "reg-dot")
		c.Match("ls --profiles lists the dotted name", `(?m)^api\.v2 `, c.Ls("--profiles").Stdout)
	})

	// The formatted registry file must equal the golden output of format --stdout for the same fixture.
	harness.Run(t, "reg_format_name", func(c *harness.Case) {
		c.Fixture(unformattedFixture, registryHome+"/fmt.glaze")
		c.Simple("cwd-untouched")
		before := c.Read(".glaze")
		original := c.Read(registryHome + "/fmt.glaze")
		r := c.Glaze("format", "--stdout", "fmt")
		c.OK(r, "format --stdout by name")
		c.Golden("format --stdout by name output", "format/stdout.glaze", r.Stdout)
		c.Equal("format --stdout by name leaves the registry file", original, c.Read(registryHome+"/fmt.glaze"))
		c.OK(c.Glaze("format", "fmt"), "format in place by name")
		c.Golden("registry file formatted in place", "format/stdout.glaze", c.Read(registryHome+"/fmt.glaze"))
		r = c.Glaze("format", "fmt")
		c.OK(r, "format in place by name again is a no-op")
		c.Match("the second format says the file is already formatted", "already formatted", r.Stderr)
		c.Equal("format by name leaves .glaze in the current directory alone", before, c.Read(".glaze"))
		c.OK(c.Glaze("format", "--validate", "fmt"), "format --validate by name")
		c.NoServer("format by name")
	})

	// save <name> is the one command that creates the registry: after a successful capture and right before the write.
	harness.Run(t, "reg_save_into_registry", func(c *harness.Case) {
		c.Mkdir("nobin")
		c.ExitCode(c.GlazeWith(harness.Opts{Env: []string{"PATH=" + c.Path("nobin")}}, "save", "--socket-name", c.Socket, "work"), "save by name without tmux on PATH exits 4", exitUnreachable)
		c.ExitCode(c.Save("work"), "save by name without a server exits 1", exitFailure)
		c.TmuxSetup("new-session", "-d", "-s", "sv-reg")
		c.TmuxSetup("split-window", "-h", "-t", "=sv-reg:")
		c.WaitForPanes("sv-reg")
		r := c.Save("work", "--session", "nope")
		c.ExitCode(r, "save by name with an unknown session exits 1", exitFailure)
		c.Match("save by name with an unknown session names the session", "session `nope` not found", r.Stderr)
		c.True("save by name with an unknown session creates no registry directory", !c.Exists(registryHome), "%s exists", registryHome)
		r = c.Save("work", "--session", "sv-reg", "--stdout")
		c.OK(r, "save --stdout by name")
		c.Match("save --stdout by name prints a profile", `^session \{`, r.Stdout)
		c.True("save --stdout by name writes nothing", !c.Exists(registryHome), "%s exists", registryHome)
		c.Equal("save --stdout by name writes no file in the work directory", "", workFiles(c, "*glaze*"))
		r = c.GlazeWith(xdgConfigHome("rel"), "save", "--socket-name", c.Socket, "--session", "sv-reg", "--stdout", "work")
		c.OK(r, "save --stdout by name with a relative XDG_CONFIG_HOME")
		c.Match("save by name with a relative XDG_CONFIG_HOME warns", "XDG_CONFIG_HOME is not absolute", r.Stderr)
		r = c.GlazeWith(glazeRegistry("rel"), "save", "--socket-name", c.Socket, "--session", "sv-reg", "work")
		c.ExitCode(r, "save by name with a relative GLAZE_REGISTRY exits 3", exitInvalidProfile)
		c.Match("save by name with a relative GLAZE_REGISTRY names the fix", "absolute", r.Stderr)
		c.NoMatch("save by name with a relative GLAZE_REGISTRY captures nothing", "saving session", r.Stderr)
		c.True("save by name with a relative GLAZE_REGISTRY creates no directory", !c.Exists("rel"), "rel exists")
		r = c.GlazeWith(glazeRegistry("rel"), "save", "--socket-name", c.Socket, "--session", "sv-reg", "--stdout", "work")
		c.ExitCode(r, "save --stdout by name with a relative GLAZE_REGISTRY exits 3", exitInvalidProfile)
		c.NoMatch("save --stdout by name with a relative GLAZE_REGISTRY prints no profile", "session", r.Stdout)

		before := c.State("sv-reg", unnamedWindow, unnamedPane)
		r = c.Save("work", "--session", "sv-reg")
		c.OK(r, "save into the registry")
		c.Match("save into the registry logs the absolute path", "saved session .*path="+regexp.QuoteMeta(c.Path(registryHome+"/work.glaze")), r.Stderr)
		c.True("save creates the registry profile", c.Exists(registryHome+"/work.glaze"), "%s/work.glaze is missing", registryHome)
		c.Equal("save creates the registry directory with mode 0700", "drwx------", modeOf(c, registryHome))
		c.Match("save writes the registry profile as a regular file the owner can read and write", "^-rw-", modeOf(c, registryHome+"/work.glaze"))
		c.NoMatch("save writes the registry profile without an execute bit", "x", modeOf(c, registryHome+"/work.glaze"))
		c.Equal("save by name leaves the work directory clean", "", workFiles(c, "*glaze*"))
		saved := c.Read(registryHome + "/work.glaze")
		c.Logf("saved profile:\n%s", saved)
		c.Match("the registry profile names the session", `name += "sv-reg"`, saved)
		r = c.Save("work", "--session", "sv-reg")
		c.ExitCode(r, "save refuses an existing registry profile (exit 1)", exitFailure)
		c.Match("save refusal says to use --force", "use --force", r.Stderr)
		c.NoMatch("save refusal happens before the capture", "saving session", r.Stderr)
		c.Equal("save refusal leaves the registry profile as it was", saved, c.Read(registryHome+"/work.glaze"))
		c.OK(c.Save("work", "--session", "sv-reg", "--force"), "save --force replaces the registry profile")
		c.OK(c.Glaze("format", "--validate", "work"), "saved registry profile validates")
		// The session is the last one, so the server exits; KillServer waits until it is gone.
		c.KillServer()
		c.OK(c.Up("work"), "up from the saved registry profile")
		roundTrip := func() string { return c.State("sv-reg", unnamedWindow, unnamedPane) }
		if !c.EventuallyEqual("saved registry profile round-trips", before, roundTrip) {
			c.Snapshot("sv-reg")
		}

		r = c.Save("work", "--profile-path", "x.glaze")
		c.ExitCode(r, "save with a name and --profile-path together is a usage error (exit 2)", exitUsage)
		c.Match("save with a name and --profile-path says not both", "not both", r.Stderr)
		c.True("save with a name and --profile-path writes no file", !c.Exists("x.glaze"), "x.glaze exists")
		r = c.Save("a/b", "--session", "sv-reg")
		c.ExitCode(r, "save with a bad name is a usage error (exit 2)", exitUsage)
		c.Match("save with a bad name says to use --profile-path", "--profile-path", r.Stderr)
		c.True("save with a bad name creates no directory", !c.Exists(registryHome+"/a"), "%s/a exists", registryHome)
		c.ExitCode(c.Save("work.glaze", "--session", "sv-reg"), "save with the .glaze suffix is a usage error (exit 2)", exitUsage)
		c.Write("parentfile", "")
		// The capture logs at debug, so this run shows it.
		r = c.GlazeWith(xdgConfigHome(c.Path("parentfile")), "--log-level", "debug", "save", "--socket-name", c.Socket, "--session", "sv-reg", "work")
		c.ExitCode(r, "save by name under a file exits 3", exitInvalidProfile)
		c.Match("save by name under a file names the cause", "a part of its path is a file", r.Stderr)
		c.Match("save by name under a file fails after the capture", "capturing window", r.Stderr)
	})

	harness.Run(t, "reg_registry_symlink", func(c *harness.Case) {
		c.Mkdir(registryHome)
		c.Simple("reg-link", "real.glaze")
		c.Symlink(c.Path("real.glaze"), registryHome+"/work.glaze")
		c.Fixture(unformattedFixture, "unf.glaze")
		c.Chmod("unf.glaze", 0o600)
		c.Symlink(c.Path("unf.glaze"), registryHome+"/link.glaze")
		c.Symlink(c.Path("gone.glaze"), registryHome+"/dead.glaze")
		c.OK(c.Up("work"), "up through a registry symlink")
		c.SessionExists("symlinked profile session created", "reg-link")
		c.OK(c.Glaze("format", "link"), "format by name through a symlink")
		c.True("format by name keeps the registry symlink", isSymlink(c, registryHome+"/link.glaze"), "format replaced the symlink with a regular file")
		c.Golden("format by name writes the link target", "format/stdout.glaze", c.Read("unf.glaze"))
		c.Equal("format by name keeps the permissions of the target", "-rw-------", modeOf(c, "unf.glaze"))
		r := c.Up("dead")
		c.ExitCode(r, "a dangling registry symlink exits 3", exitInvalidProfile)
		c.Match("a dangling registry symlink names the target", `gone\.glaze`, r.Stderr)
		r = c.Ls("--profiles")
		c.NoMatch("ls --profiles skips a dangling symlink", "(?m)^dead ", r.Stdout)
		c.Match("ls --profiles warns about a dangling symlink", "dead", r.Stderr)
	})

	harness.Run(t, "reg_ls_profiles", func(c *harness.Case) {
		c.Simple("ls-a", registryHome+"/alpha.glaze")
		c.Simple("ls-b", registryHome+"/beta.glaze")
		c.Write(registryHome+"/notes.txt", "")
		c.Write(registryHome+"/.hidden.glaze", "")
		c.Write(registryHome+"/bad name.glaze", "")
		c.Mkdir(registryHome + "/sub")
		c.Mkdir(registryHome + "/sub.glaze")
		r := c.Ls("--profiles")
		c.OK(r, "ls --profiles")
		c.Match("ls --profiles header", "NAME +PATH", r.Stdout)
		c.Match("ls --profiles alpha row", "(?m)^alpha +"+regexp.QuoteMeta(c.Path(registryHome+"/alpha.glaze")), r.Stdout)
		c.Match("ls --profiles lists names in order", `alpha(.|\n)*beta`, r.Stdout)
		c.NoMatch("ls --profiles skips a file that is not a profile", "(?m)^notes", r.Stdout)
		c.NoMatch("ls --profiles skips a hidden file", `(?m)^\.hidden`, r.Stdout)
		c.NoMatch("ls --profiles skips a directory", "(?m)^sub", r.Stdout)
		c.NoMatch("ls --profiles skips a directory named like a profile", "(?m)^sub", r.Stdout)
		c.NoMatch("ls --profiles skips a name that cannot be used", "(?m)^bad name", r.Stdout)
		c.Match("ls --profiles warns about a name that cannot be used", "bad name", r.Stderr)
		c.Golden("ls --profiles output", "ls/profiles.txt", r.Stdout)
		r = c.Up("sub")
		c.ExitCode(r, "up of a directory named like a profile exits 3", exitInvalidProfile)
		c.Match("up of a directory names the cause", "is a directory, not a profile", r.Stderr)
		c.NoServer("ls --profiles")
		c.Mkdir("nobin")
		c.OK(c.GlazeWith(harness.Opts{Env: []string{"PATH=" + c.Path("nobin")}}, "ls", "--profiles"), "ls --profiles works without tmux on PATH")
		c.Simple("xdg-one", "xdg/glaze/one.glaze")
		c.Match("ls --profiles honours XDG_CONFIG_HOME", "(?m)^one ", c.GlazeWith(xdgConfigHome(c.Path("xdg")), "ls", "--socket-name", c.Socket, "--profiles").Stdout)
		c.Simple("env-one", "reg/two.glaze")
		c.Match("ls --profiles honours GLAZE_REGISTRY", "(?m)^two ", c.GlazeWith(glazeRegistry(c.Path("reg")), "ls", "--socket-name", c.Socket, "--profiles").Stdout)
	})

	// No registry means no profiles, so stdout stays empty for a script that reads it.
	harness.Run(t, "reg_ls_profiles_empty", func(c *harness.Case) {
		r := c.Ls("--profiles")
		c.OK(r, "ls --profiles without a registry exits 0")
		c.Equal("ls --profiles without a registry prints nothing", "", r.Stdout)
		c.Match("ls --profiles without a registry says so", "no profile registry", r.Stderr)
		c.Match("ls --profiles without a registry points at save", "`glaze save <name>` creates it", r.Stderr)
		c.True("ls --profiles without a registry creates nothing", !c.Exists(registryHome), "%s exists", registryHome)
		c.Mkdir(registryHome)
		c.Match("ls --profiles with an empty registry prints the header only", `^NAME +PATH\n$`, c.Ls("--profiles").Stdout)
		c.NoServer("ls --profiles with an empty registry")
	})

	harness.Run(t, "reg_registry_is_file", func(c *harness.Case) {
		c.Write(registryHome, "")
		r := c.Up("work")
		c.ExitCode(r, "registry that is a file exits 3", exitInvalidProfile)
		c.Match("registry that is a file says so", "is a file, not the profile registry", r.Stderr)
		c.Match("registry that is a file names the variables to fix", "XDG_CONFIG_HOME", r.Stderr)
		c.NoServer("registry that is a file")
		r = c.Ls("--profiles")
		c.ExitCode(r, "ls --profiles on a registry that is a file exits 3", exitInvalidProfile)
		c.Match("ls --profiles on a registry that is a file names the cause", "is a file", r.Stderr)
		r = c.Glaze("up", "--generate-shell-completion")
		c.OK(r, "candidates on a registry that is a file exit 0")
		c.Equal("candidates on a registry that is a file are empty", "", r.Stdout)
		c.Equal("candidates on a registry that is a file stay silent", "", r.Stderr)
		r = c.Down("work")
		c.ExitCode(r, "down on a registry that is a file exits 3", exitInvalidProfile)
		c.Match("down on a registry that is a file names --session", "--session", r.Stderr)
	})

	// A user cannot read a directory without permissions. root can, so as root the case only logs the result.
	harness.Run(t, "reg_registry_unreadable", func(c *harness.Case) {
		c.Simple("unr-reg", registryHome+"/work.glaze")
		c.Chmod(registryHome, 0)
		if os.Geteuid() == 0 {
			c.Logf("up by name on an unreadable registry as root: exit %d", c.Up("work").Code)
			return
		}

		r := c.Up("work")
		c.ExitCode(r, "unreadable registry exits 3", exitInvalidProfile)
		c.Match("unreadable registry names the cause", "permission denied", r.Stderr)
		c.NoServer("unreadable registry")
		c.ExitCode(c.Ls("--profiles"), "ls --profiles on an unreadable registry exits 3", exitInvalidProfile)
		r = c.Glaze("up", "--generate-shell-completion")
		c.OK(r, "candidates with an unreadable registry exit 0")
		c.Equal("candidates with an unreadable registry stay silent", "", r.Stderr)
		c.Chmod(registryHome, 0o100)
		c.OK(c.Up("work"), "up by name works on a search-only registry")
		c.ExitCode(c.Ls("--profiles"), "ls --profiles on a search-only registry exits 3", exitInvalidProfile)
	})

	// The completion scripts run glaze with --generate-shell-completion and read one name per line.
	harness.Run(t, "reg_completion", func(c *harness.Case) {
		c.Simple("cmp-work", registryHome+"/work.glaze")
		c.Simple("cmp-api", registryHome+"/api.glaze")
		c.Write(registryHome+"/bad name.glaze", "")
		for _, sub := range registryCommands {
			r := c.Glaze(sub, "--generate-shell-completion")
			c.OK(r, sub+" candidates with a registry")
			c.Match(sub+" candidates include work", "(?m)^work$", r.Stdout)
			c.Match(sub+" candidates include api", "(?m)^api$", r.Stdout)
			c.Equal(sub+" candidates write nothing to stderr", "", r.Stderr)
		}

		r := c.Glaze("up", "--generate-shell-completion")
		c.NoMatch("candidates leave out a name that cannot be used", "bad", r.Stdout)
		c.NoMatch("candidates have no help line", "(?m)^help", r.Stdout)
		c.Match("flag candidates of up still include --profile-path", "(?m)^--profile-path:", c.Glaze("up", "--p", "--generate-shell-completion").Stdout)
		c.Equal("candidates after a name are empty", "", c.Glaze("up", "work", "--generate-shell-completion").Stdout)
		c.Equal("candidates with --profile-path are empty", "", c.Glaze("up", "--profile-path", "x", "--generate-shell-completion").Stdout)
		c.Equal("candidates with --session are empty", "", c.Glaze("down", "--session", "x", "--generate-shell-completion").Stdout)
		for _, sub := range []string{"save", "ls"} {
			c.NoMatch(sub+" candidates do not list profiles", "(?m)^work$", c.Glaze(sub, "--generate-shell-completion").Stdout)
		}

		c.Simple("cmp-xdg", "xdg/glaze/other.glaze")
		c.Match("candidates honour XDG_CONFIG_HOME", "(?m)^other$", c.GlazeWith(xdgConfigHome(c.Path("xdg")), "up", "--generate-shell-completion").Stdout)
		c.Simple("cmp-env", "reg/envp.glaze")
		c.Match("candidates honour GLAZE_REGISTRY", "(?m)^envp$", c.GlazeWith(glazeRegistry(c.Path("reg")), "up", "--generate-shell-completion").Stdout)
		c.NoServer("completion")
	})

	harness.Run(t, "reg_completion_no_registry", func(c *harness.Case) {
		r := c.Glaze("up", "--generate-shell-completion")
		c.OK(r, "candidates without a registry directory exit 0")
		c.Equal("candidates without a registry are empty", "", r.Stdout)
		c.Equal("candidates without a registry write nothing to stderr", "", r.Stderr)
	})

	// A relative starting_directory keeps its base, the directory of the profile, which is the registry for a registry profile.
	harness.Run(t, "reg_relative_dir", func(c *harness.Case) {
		c.Fixture("directories/relative.glaze", registryHome+"/rel.glaze")
		c.Mkdir(registryHome+"/sub", "elsewhere/sub")
		c.Cd("elsewhere")
		c.OK(c.Up("rel"), "up by name with a relative starting_directory")
		c.EventuallyPanePaths("relative starting_directory is relative to the registry directory", c.Path(registryHome+"/sub"), "=relative:")
	})

	harness.Run(t, "reg_relative_dir_missing", func(c *harness.Case) {
		c.Fixture("directories/relative.glaze", registryHome+"/rel.glaze")
		r := c.Up("rel")
		c.ExitCode(r, "a missing relative directory in the registry is rejected (exit 3)", exitInvalidProfile)
		c.Match("the diagnostic names the registry directory as the base", `relative to the directory of the profile(.|\n)*\.config/glaze`, r.Stderr)
		c.NoServer("missing relative directory")
	})

	harness.Run(t, "reg_dot_dir", func(c *harness.Case) {
		c.Fixture("registry/dot-dir.glaze", registryHome+"/dot.glaze")
		c.Mkdir("elsewhere")
		c.Cd("elsewhere")
		c.OK(c.Up("dot"), "up by name with starting_directory .")
		c.EventuallyPanePaths("starting_directory . in a registry profile is the registry directory", c.Path(registryHome), "=reg-dot-dir:")
	})

	// A session without starting_directory starts where glaze runs, and path.base is that directory, so one profile serves many projects.
	harness.Run(t, "reg_default_cwd", func(c *harness.Case) {
		c.Simple("reg-cwd", registryHome+"/cwd.glaze")
		c.Fixture("registry/per-project.glaze", registryHome+"/dev.glaze")
		c.Mkdir("here")
		c.Cd("here")
		c.OK(c.Up("cwd"), "a registry profile without starting_directory uses the current directory")
		c.EventuallyPanePaths("the session without starting_directory starts in the current directory", c.Path("here"), "=reg-cwd:")
		c.OK(c.Up("dev"), "up by name with path.base in the session name")
		c.SessionExists("path.base in a registry profile is the current directory", "reg-here")
		c.Mkdir("other")
		c.Cd("other")
		r := c.Down("dev")
		c.OK(r, "down by name from another directory is a no-op")
		c.Match("down by name from another directory says that the name depends on the directory", "depends on the current directory.*session=reg-other", r.Stderr)
		c.SessionExists("path.base session survives a down from another directory", "reg-here")
		c.Cd("here")
		c.OK(c.Down("dev"), "down by name computes path.base from the current directory")
		c.SessionGone("path.base session gone", "reg-here")
	})

	harness.Run(t, "reg_home_dir", func(c *harness.Case) {
		c.Fixture("registry/home-dir.glaze", registryHome+"/hd.glaze")
		c.Mkdir("home/code")
		c.OK(c.Up("hd"), "up by name with a ~ starting_directory")
		c.EventuallyPanePaths("~ in a registry profile is the case HOME", c.Path("home/code"), "=reg-home-dir:")
	})

	// A name never reads the current directory, so it still resolves from a directory that was deleted.
	harness.Run(t, "reg_deleted_cwd", func(c *harness.Case) {
		if runtime.GOOS == "darwin" {
			c.T().Skip("macOS still reports the path of a deleted working directory")
		}

		c.Fixture("registry/home-dir.glaze", registryHome+"/with.glaze")
		c.Mkdir("home/code")
		c.Simple("reg-without", registryHome+"/without.glaze")
		c.Fixture("registry/per-project.glaze", registryHome+"/pwd.glaze")
		up := []string{"up", "--detached", "--socket-name", c.Socket}

		c.OK(fromDeleted(c, "format", "--validate", "with"), "format --validate by name from a deleted directory")
		c.OK(fromDeleted(c, append(up, "with")...), "up by name from a deleted directory")
		c.SessionExists("session by name from a deleted directory", "reg-home-dir")
		c.OK(fromDeleted(c, "down", "--socket-name", c.Socket, "with"), "down by name from a deleted directory")
		c.SessionGone("down by name from a deleted directory kills the session", "reg-home-dir")
		c.ExitCode(fromDeleted(c, "diff", "--socket-name", c.Socket, "with"), "diff by name from a deleted directory plans the session (exit 5)", exitDifferences)

		r := fromDeleted(c, append(up, "without")...)
		c.Fails(r, "up by name without a session directory from a deleted directory")
		c.Match("the error says the session needs a directory", "no starting_directory", r.Stderr)
		r = fromDeleted(c, "diff", "--socket-name", c.Socket, "without")
		c.ExitCode(r, "diff by name without a session directory from a deleted directory exits 1", exitFailure)
		c.Match("the diff error by name says the session needs a directory", "no starting_directory", r.Stderr)

		r = fromDeleted(c, append(up, "pwd")...)
		c.Fails(r, "path.base by name from a deleted directory")
		c.Match("the path.base error names the current directory", "Current directory not available", r.Stderr)

		r = fromDeleted(c, append(up, "nope")...)
		c.ExitCode(r, "an unknown name from a deleted directory exits 3 without a cwd hint", exitInvalidProfile)
		c.NoMatch("the miss from a deleted directory has no current directory hint", "current directory", r.Stderr)
	})

	harness.Run(t, "reg_root_hint", func(c *harness.Case) {
		c.Simple("reg-root", registryHome+"/work.glaze")
		r := c.Glaze("work")
		c.ExitCode(r, "an unknown command that names a profile is still a usage error (exit 2)", exitUsage)
		c.Match("the usage error suggests glaze up", "glaze up work", r.Stderr)
		c.NoMatch("an unknown command that names no profile keeps the plain message", "glaze up", c.Glaze("nope").Stderr)
		c.NoServer("an unknown command")
	})

	// format refuses an in-place write of a pipe or a device in the registry before it reads it, so it cannot block.
	harness.Run(t, "reg_format_fifo", func(c *harness.Case) {
		c.Mkdir(registryHome)
		c.Mkfifo(registryHome + "/f.glaze")
		r := c.GlazeWith(harness.Opts{Timeout: hangTimeout}, "format", "f")
		c.Fails(r, "format by name of a FIFO in place")
		c.Match("FIFO by name refusal says to use --stdout", "use --stdout", r.Stderr)
		c.Symlink("/dev/stdin", registryHome+"/dev.glaze")
		r = c.GlazeWith(harness.Opts{Timeout: hangTimeout}, "format", "dev")
		c.Fails(r, "format by name of a symlink to /dev/stdin in place")
		c.Match("/dev/stdin by name refusal says to use --stdout", "use --stdout", r.Stderr)
		c.Match("ls --profiles warns about a FIFO", "not a regular file", c.Ls("--profiles").Stderr)
		c.NoServer("format by name of a FIFO")
	})
}
