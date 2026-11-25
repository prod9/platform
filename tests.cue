#Config: {
	interpreter?: string | *"/bin/sh"
	timeout?:     =~"\\d+[s|m|h]"
	workdir?:     string
}

#Check: "exitcode" | "stdout" | "stderr" | =~"\\./testbeds/.+"

#Test: {
	config?: #Config
	name:    string
	tests?: [...#Test]
	checks?: [...#Check]
	commands?: [...string]
}

let testbeds = [...{name: string, dir: string}] &
[
	{name: "Go Basic", dir: "gobasic"},
	{name: "Go Workspace", dir: "gowork"},
	{name: "PNPM Basic", dir: "pnpmbasic"},
	{name: "PNPM Static", dir: "pnpmstatic"},
	{name: "Dockerfile", dir: "dockerfile"},
]

#Test & {
	name: "Platform"
	config: {
		interpreter: "/bin/sh"
		timeout:     "1m"
		workdir:     "."
	}
	tests: [
		{
			name: "Platform"
			checks: ["exitcode"]
			commands: ["go build -v -o ./bin/platform ."]
		},
		{
			name: "Release"
			checks: ["exitcode", "stdout"]
			commands: ["sh ./testbeds/release/test.sh"]
		},
		{
			// A selected module that cannot be interpreted fails in the observed config
			// phase, before any framework step or engine connection starts.
			name: "Build Missing Module"
			checks: ["exitcode", "stderr"]
			commands: ["./bin/platform -f testbeds/gobasic/platform.toml build missing-module"]
		},
		for testbed in testbeds {
			{
				name: testbed.name
				tests: [
					{
						// init discovers the owning framework for the testbed dir and runs that
						// framework's Scaffold; the written platform.toml [modules] captures the
						// discovered framework, so the dropped standalone `discover` command needs
						// no separate test.
						name: "Init"
						checks: [
							"./testbeds/\(testbed.dir)/platform.toml",
							"./testbeds/\(testbed.dir)/platform",
						]
						commands: [
							"./testbed.sh \(testbed.dir) init \"Johnny Appleseed\" \"john@apple.com\" \"github.com/prod9/platform\"",
						]
					},
					{
						name: "Build"
						checks: ["exitcode"]
						commands: [
							"./testbed.sh \(testbed.dir) -q build",
						]
					},
				]
			}
		},
		{
			// The only smoke case that touches a built container after its run ends — the
			// path a session-scoped connection breaks first (a closed session 502s here long
			// before it shows up in a plain build). The tarball is binary and unstable, so
			// the file check is its existence, not its contents.
			name: "Export"
			checks: ["exitcode"]
			commands: [
				"rm -f testbeds/gobasic/gobasic.docker",
				"./testbed.sh gobasic -q export",
				"test -f testbeds/gobasic/gobasic.docker",
			]
		},
		{
			// infra init into a fresh (git-ignored) dir: it vendors infra-lua's newest tag from
			// GitHub as a git subtree, copies its templates and seeds the vars they declare
			// (docs/spec/scaffolding.md). The snapshot is the generated platform.toml; the
			// follow-on render is the guard.
			name: "Infra Init"
			checks: [
				"exitcode",
				"./testbeds/infra-init/platform.toml",
			]
			commands: [
				// Wipe the leftovers of a prior local run, the nested repo and its vendored
				// infra-lua included, so init vendors and GENERATES fresh. The outer repo tracks
				// only .gitignore here.
				"rm -rf testbeds/infra-init/.git testbeds/infra-init/vendor testbeds/infra-init/apps testbeds/infra-init/defaults.lua testbeds/infra-init/platform.toml testbeds/infra-init/platform testbeds/infra-init/k8s",
				// git subtree add needs a commit on the branch to merge into.
				"git -C testbeds/infra-init init -q .",
				"git -C testbeds/infra-init -c user.name=smoke -c user.email=smoke@example.com commit -q --allow-empty -m base",
				// ALWAYS_YES=1 confirms the vendoring and the apply prompt.
				"ALWAYS_YES=1 ./testbed.sh infra-init init \"Johnny Appleseed\" \"john@apple.com\" \"prod9/infra-new\"",
			]
		},
		{
			// Check the exact generated pin before projecting commit-dependent metadata.
			name: "Infra Init Launcher"
			checks: ["exitcode", "stdout"]
			commands: ["sh ./testbeds/check-launcher.sh"]
		},
		{
			name: "Infra Init Render"
			checks: ["exitcode"]
			commands: [
				"./testbed.sh infra-init render",
			]
		},
		{
			name: "Render"
			checks: [
				"exitcode",
				"stdout",
				"./testbeds/infra-basic/k8s/infra-basic/*.yaml",
			]
			commands: [
				"./testbed.sh infra-basic render",
			]
		},
		{
			// Exercises the Infra framework end to end: render apps/ and pack the
			// manifest tree into a FROM scratch image. A clean exit means the render fed a
			// buildable image (scratch has no shell, so we can't ls inside it; the Render
			// test above already snapshots the rendered contents).
			name: "Infra Build"
			checks: ["exitcode"]
			commands: [
				"./testbed.sh infra-basic -q build",
			]
		},
	]
}
