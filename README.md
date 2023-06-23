# PRODIGY9 PLATFORM

The Platform application is a self-contained (as much as possible) program for building
various projects and application components being built at PRODIGY9.

The application has the following primary goals:

1. Eliminates build configuration from all projects or minimize them as much as feasible.
2. Allows new project to bootstrap into a CI/CD environment as fast as possible.
3. Don't lock-in the company into a single tech stack, instead allow the company to work
   on any and all stacks if/when required as quickly as, or quicker than, the last one.

### Usage

The platform application is self-built and self-contained. It's only requirements is a
recent "go" binary installed on the developer's local machine.

1. Run `go run platform.prodigy9.co bootstrap` to setup the TOML config file.
2. Edit `platform.toml` if needed.
3. Run `go run platform.prodigy9.co build` to start building.


