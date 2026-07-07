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
						// init on a non-"infra" testbed dir takes the app-scaffold path;
						// the written platform.toml [modules] captures the discovered builder,
						// so the dropped standalone `discover` command needs no separate test.
						name: "Init"
						checks: [
							"./testbeds/\(testbed.dir)/platform.toml",
							"./testbeds/\(testbed.dir)/platform",
						]
						commands: [
							"./testbed.sh \(testbed.dir) init \"Johnny Appleseed\" \"john@apple.com\" \"github.com/prod9/platform\" \"ghcr.io/prod9/platform\"",
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
			name: "Render"
			checks: [
				"exitcode",
				"stdout",
				"./testbeds/infra-basic/k8s/infra-basic/*.yaml",
			]
			commands: [
				"./testbed.sh infra-basic ops render",
			]
		},
	]
}
