package harness

// Glaze runs glaze with args in the current directory of the case.
func (c *Case) Glaze(args ...string) *Result {
	c.t.Helper()
	return c.GlazeWith(Opts{}, args...)
}

// GlazeWith runs glaze with args and options.
func (c *Case) GlazeWith(o Opts, args ...string) *Result {
	c.t.Helper()
	return c.Exec(o, c.env.Glaze, args...)
}

// Up runs "glaze up --detached" against the server of the case.
func (c *Case) Up(args ...string) *Result {
	c.t.Helper()
	return c.UpWith(Opts{}, args...)
}

// UpWith runs "glaze up --detached" with options.
func (c *Case) UpWith(o Opts, args ...string) *Result {
	c.t.Helper()
	return c.GlazeWith(o, c.onServer("up", append([]string{"--detached"}, args...))...)
}

// StartUp runs "glaze up --detached" in the background, for a case that
// signals glaze or runs two of them at the same time.
func (c *Case) StartUp(o Opts, args ...string) *Process {
	c.t.Helper()
	return c.Start(o, c.env.Glaze, c.onServer("up", append([]string{"--detached"}, args...))...)
}

// Down runs "glaze down" against the server of the case.
func (c *Case) Down(args ...string) *Result {
	c.t.Helper()
	return c.Glaze(c.onServer("down", args)...)
}

// Save runs "glaze save" against the server of the case.
func (c *Case) Save(args ...string) *Result {
	c.t.Helper()
	return c.Glaze(c.onServer("save", args)...)
}

// Ls runs "glaze ls" against the server of the case.
func (c *Case) Ls(args ...string) *Result {
	c.t.Helper()
	return c.Glaze(c.onServer("ls", args)...)
}

// onServer returns the arguments of a subcommand on the server of the case.
func (c *Case) onServer(subcommand string, args []string) []string {
	return append([]string{subcommand, "--socket-name", c.Socket}, args...)
}
