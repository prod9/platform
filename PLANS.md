# Pending proposals

The current design lives in [docs/spec/](docs/spec/); the active implementation handoff
lives in the machine-local ACE save and ledger under `.ace/`.
The retired June implementation sequence is historical Git content, not a current plan.

## Proposals requiring a fresh scope decision

- **Container hardening.** The older plan proposed non-root runner images, runtime
  capability restrictions, read-only filesystems, signal handling, and engine/platform
  workload resource settings. These are retained as proposals whose completion and scope
  need verification before implementation; the current specs do not define that pass.
  The scaffold already supplies an engine NetworkPolicy and the platform pod labels that
  admit it ([apps-platform.cue.tmpl](framework/skel/apps-platform.cue.tmpl)), so the old
  claim that engine ingress still has no policy must not be reused.
- **Build version metadata.** The older plan proposed commit/release metadata in runner
  images, environment variables, and OCI revision labels. It has no current approved spec;
  the old `/app` paths and builder package names are obsolete. Any proposal must start
  from [frameworks](docs/spec/frameworks.md), the
  [engine contract](docs/spec/engine.md), and the application-runtime boundary recorded
  in [the container-opacity decision][container-opacity].

Neither proposal authorizes work or implies that the old implementation details remain
valid. Completed and superseded items belong to Git history rather than a second resume
record here.

[container-opacity]: docs/decisions/2026-07-27-platform-never-reaches-into-the-container.md
