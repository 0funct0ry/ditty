# internal/fixture — protocol parity checklist

Every behaviour below is exercised by `internal/fixture`'s Hub against the
`ditty.v1` wire protocol, driven by a scripted Scenario rather than a live
PTY. `internal/session` (M9) must reproduce every line here against the
*real* Hub before `internal/fixture` and the `--fixture` flag are deleted
(SPEC.md §12 M9). This file is the acceptance criteria for that deletion,
not a suggestion.

- [ ] **Join replay (first attach).** A Client's first `Attach` receives
      `Hello`, then a ring replay of everything emitted so far, before any
      new live `Output`. Exercised by every scenario.
- [ ] **Mid-session join.** A Client attaching after the scenario has
      already emitted several chunks sees them all via ring replay, not
      just the live tail. Exercised by `deploy`, `htop`.
- [ ] **Resize.** A Client's `Resize` frame is accepted without error.
      Exercised by all scenarios (no dedicated one — the fixture has no PTY
      to resize against, so this only proves the frame is well-formed and
      harmless).
- [ ] **Disconnect / reconnect.** The Hub force-closes every attached
      Client at a scripted point, then a fresh `Attach` after redial gets
      `Hello` plus a ring replay that includes everything emitted both
      before and during the drop. Exercised by `flaky`.
- [ ] **Exit code.** The Hub broadcasts an `Exit` frame with a non-zero
      `code` and transitions session state to `closed`; a Client attaching
      afterward gets the same `Exit` immediately after replay. Exercised by
      `quickexit`.
- [ ] **Exit signal.** The `Ending{Kind: EndingSignal}` path broadcasts an
      `Exit` frame with `signal` set instead of `code`. Not currently
      exercised by any of the four named scenarios — `EndingSignal` exists
      in `scenario.go` for M9 to drive with its own signal-based fixture
      data if useful, or to remove if unused once the real Hub covers it
      with a live `SIGTERM`/`SIGKILL` test.
- [ ] **Roster join/leave.** `Roster` is broadcast to every attached Client
      on every `Attach` and `Detach`. Exercised by all scenarios whenever
      more than one Client is attached.
- [ ] **Sizing handover.** The first-attached Client is the sizing Client;
      on its `Detach`, the role passes to the next Client by join order and
      a `State` frame announces it. Exercised by attaching two+ Clients to
      any scenario and detaching the first.
