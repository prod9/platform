First of all, just like TypeScript, I *hate* Helm with a passion. It's magic code at a
location where we should have as little magic code as possible (from a human engineer
perspective) I want human-tracable and human-debuggable at all times and that rules out
Helm completely. Just like TypeScript, mention in my personal CLAUDE.md that I hate it
with a pssion and never suggest to use it. Whenever it comes up, find minimal ways to get
by WITHOUT it. etc.

on BuildKite:
1. BuildKite-the-runner. Dagger yep.
2. BuildKite-the-control-plan. This is what I plan to make platform becomes. Instead of
   just CLI, it's now a server-hosted component with UIs.

on Decision map:
1. Real server. There are a number of RBAC decisions that cannot be cleanly managed it's
   currently quite cumbersome to setup new projects and to prune accesses when people
   leave etc. The current setup also use root/superadmin accounts in multiple places that
   should be replace with fine-grained controls.
2. Where builds execute -> Dagger Engine. The thing I'm worried about is if the engine
   resource consumption (including during builds/container/services it run etc.) signals
   are sent back to kubectl and maanged there so we manage server resources purely through
   kube without having to setup yet another build server/farms and scale outs are the same
   as eveyrthing else: Add/upgrade nodes.
3. GitHub webhooks, mostly. No GH Actions. Or a trigger from dev local machine somehow
   either via platform cli that connects to the platform server or otherwise. We might
   need to have an authentication layer on it anyway (mapping work email -> github
   accounts -> k8s service account -> argo accounts etc.)
4. ArgoCD. platform server shuld do the git dance if only because of automation. Either
   that or we defer to platform cli to have a command to do it but which would just be
   using dagger internally to download-edit-commit and relying on user's setup git
   credentials etc.
5. Yep.
6. I'm thinking we just have a set of CUE manifests somewhere (this is the part where we
   wanted to fold infra-cli in prior) that applies to k8s along with some basic checks.
   Bonus if it can also do some basic DNS management as well (through CF API)


Your rationale is good but that's precisely the problem: Knowledge are scattered across
multiple tools. I'd like a single, streamlined tool instead.
