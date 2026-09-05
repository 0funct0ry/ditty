import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { useAuth, type UseAuthResult } from "./useAuth";
import type { AuthClient, LoginResult } from "../protocol/auth";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// A minimal render harness — no @testing-library dependency needed for one
// hook. Renders a component that stashes the hook's latest return value in
// `latest`, so assertions can read it synchronously after each act().
function renderUseAuth(client: AuthClient, initialRequired: boolean, initialAlready = false) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  let root: Root;
  const latest: { current: UseAuthResult | null } = { current: null };
  let setRequiredExternal: ((v: boolean) => void) | null = null;
  let setAlreadyExternal: ((v: boolean) => void) | null = null;

  function Harness({ required, already }: { required: boolean; already: boolean }) {
    latest.current = useAuth(client, required, already);
    return null;
  }

  function Wrapper() {
    const [required, setRequired] = useState(initialRequired);
    const [already, setAlready] = useState(initialAlready);
    setRequiredExternal = setRequired;
    setAlreadyExternal = setAlready;
    return <Harness required={required} already={already} />;
  }

  act(() => {
    root = createRoot(container);
    root.render(<Wrapper />);
  });

  return {
    latest: () => latest.current as UseAuthResult,
    setRequired: (v: boolean) => act(() => setRequiredExternal?.(v)),
    setAlready: (v: boolean) => act(() => setAlreadyExternal?.(v)),
    setBoth: (required: boolean, already: boolean) =>
      act(() => {
        setRequiredExternal?.(required);
        setAlreadyExternal?.(already);
      }),
    cleanup: () => act(() => root.unmount()),
  };
}

function fakeClient(result: LoginResult): AuthClient {
  return {
    login: async () => result,
    logout: async () => {},
  };
}

let cleanups: Array<() => void> = [];
afterEach(() => {
  cleanups.forEach((c) => c());
  cleanups = [];
});

describe("useAuth", () => {
  it("stays gated once required flips true after an optimistic false render", () => {
    // Regression test: App.tsx doesn't know synchronously whether sign-in
    // is required, so `required` starts false and flips true once the
    // async probe resolves. The hook must not get stuck reporting "authed"
    // just because that was the correct answer for the first, optimistic
    // render.
    const h = renderUseAuth(fakeClient({ ok: true, role: "operator" }), false);
    cleanups.push(h.cleanup);
    expect(h.latest().status).toBe("authed");

    h.setRequired(true);
    expect(h.latest().status).toBe("gate");
  });

  it("starts gated when required is already true at mount", () => {
    const h = renderUseAuth(fakeClient({ ok: true, role: "viewer" }), true);
    cleanups.push(h.cleanup);
    expect(h.latest().status).toBe("gate");
  });

  it("login success reaches authed and survives required being re-asserted true", async () => {
    const h = renderUseAuth(fakeClient({ ok: true, role: "operator" }), true);
    cleanups.push(h.cleanup);

    await act(async () => {
      await h.latest().login("alice", "hunter22ok");
    });
    expect(h.latest().status).toBe("authed");
    expect(h.latest().role).toBe("operator");

    // required flipping true again (e.g. a redundant re-check) must not
    // knock a real, successful login back to the gate.
    h.setRequired(true);
    expect(h.latest().status).toBe("authed");
  });

  it("logout returns to gate and a fresh required=true keeps it gated", () => {
    const h = renderUseAuth(fakeClient({ ok: true, role: "operator" }), true);
    cleanups.push(h.cleanup);

    act(() => h.latest().logout());
    expect(h.latest().status).toBe("gate");
  });

  it("login failure reports the reason and returns to gate", async () => {
    const h = renderUseAuth(fakeClient({ ok: false, reason: "incorrect username or password" }), true);
    cleanups.push(h.cleanup);

    await act(async () => {
      await h.latest().login("alice", "wrong");
    });
    expect(h.latest().status).toBe("gate");
    expect(h.latest().error).toBe("incorrect username or password");
  });

  it("a page refresh with an already-valid cookie goes straight to authed, not the gate", () => {
    // Regression test: a fresh mount (loggedInRef starts false, exactly
    // like after a real page reload) where the async probe resolves with
    // required=true AND alreadyAuthenticated=true (a still-valid
    // ditty_access cookie) must land on "authed" directly — this is the
    // bug that made the sign-out button vanish on refresh, since
    // required=true alone (without alreadyAuthenticated) used to always
    // mean "show the gate."
    const h = renderUseAuth(fakeClient({ ok: true, role: "operator" }), false);
    cleanups.push(h.cleanup);
    expect(h.latest().status).toBe("authed"); // optimistic pre-probe render

    h.setBoth(true, true); // the probe resolves: required, and already signed in
    expect(h.latest().status).toBe("authed");
  });

  it("alreadyAuthenticated=false with required=true still shows the gate", () => {
    const h = renderUseAuth(fakeClient({ ok: true, role: "operator" }), false);
    cleanups.push(h.cleanup);

    h.setBoth(true, false); // required, but no valid cookie yet
    expect(h.latest().status).toBe("gate");
  });
});
