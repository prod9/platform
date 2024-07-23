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
						name: "Discover"
						checks: ["stdout"]
						commands: ["./testbed.sh \(testbed.dir) discover | sort"]
					},
					{
						name: "Bootstrap"
						checks: [
							"./testbeds/\(testbed.dir)/platform.toml",
							"./testbeds/\(testbed.dir)/platform",
							"./testbeds/\(testbed.dir)/.buildkite/*.*",
						]
						commands: [
							"./testbed.sh \(testbed.dir) bootstrap \"Johnny Appleseed\" \"john@apple.com\" \"github.com/prod9/platform\" \"ghcr.io/prod9/platform\"",
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
	]
}
