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
	{name: "Go Basic", dir:       "gobasic"},
	{name: "Go Workspace", dir:   "gowork"},
	{name: "PNPM Basic", dir:     "pnpmbasic"},
	{name: "PNPM Workspace", dir: "pnpmwork"},
	{name: "PNPM Static", dir:    "pnpmstatic"},
	{name: "Dockerfile", dir:     "dockerfile"},
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
			// infra init generates the whole baseline into a fresh (git-ignored) dir. The
			// module path comes from the CUE_MOD_PREFIX scaffold input (example.com) — the 4th
			// positional arg — NOT the bare, non-domain repository `prod9/infra-new`, which CUE
			// would reject as a module path. The snapshot proves platform.toml carries no
			// module-path key (it lives only in cue.mod); the follow-on render is the guard.
			name: "Infra Init"
			checks: [
				"exitcode",
				"./testbeds/infra-init/platform.toml",
			]
			commands: [
				// Wipe any generated leftovers from a prior local run so init GENERATES fresh
				// rather than surgically merging a stale platform.toml (which would preserve an
				// old import_prefix line). The testbed commits only .gitignore.
				"rm -rf testbeds/infra-init/apps testbeds/infra-init/defaults testbeds/infra-init/cue.mod testbeds/infra-init/platform.toml testbeds/infra-init/platform testbeds/infra-init/k8s",
				// ALWAYS_YES=1 auto-confirms the apply prompt — this testbed carries no
				// committed files to fall back on, so init must actually run to generate them.
				"ALWAYS_YES=1 ./testbed.sh infra-init init \"Johnny Appleseed\" \"john@apple.com\" \"prod9/infra-new\" \"example.com\"",
			]
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
