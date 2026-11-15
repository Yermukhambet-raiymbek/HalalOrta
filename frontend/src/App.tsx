import { useCallback, useEffect, useState } from "react";

/** Shape of GET /api/health (see backend/internal/httpapi/router.go). */
type Health = {
  status: string;
  database: string;
};

type CheckState =
  | { kind: "loading" }
  | { kind: "done"; ok: boolean; health: Health }
  | { kind: "error" };

export default function App() {
  const [state, setState] = useState<CheckState>({ kind: "loading" });

  const check = useCallback(async () => {
    setState({ kind: "loading" });
    try {
      const res = await fetch("/api/health", {
        headers: { Accept: "application/json" },
      });
      const health = (await res.json()) as Health;
      setState({ kind: "done", ok: res.ok, health });
    } catch {
      // Network failure or a non-JSON reply (e.g. the dev proxy cannot reach the server).
      setState({ kind: "error" });
    }
  }, []);

  useEffect(() => {
    void check();
  }, [check]);

  return (
    <main className="page">
      <h1>LittleVillage</h1>
      <p className="tagline">Learn baby care together as a family.</p>

      <section className="card" aria-live="polite">
        <h2>System status</h2>
        <StatusLine state={state} />
        <button type="button" onClick={() => void check()} disabled={state.kind === "loading"}>
          Check again
        </button>
      </section>
    </main>
  );
}

function StatusLine({ state }: { state: CheckState }) {
  switch (state.kind) {
    case "loading":
      return <p className="status">Checking…</p>;
    case "error":
      return <p className="status status-bad">Server unreachable.</p>;
    case "done":
      return (
        <p className={`status ${state.ok ? "status-ok" : "status-bad"}`}>
          Server: {state.health.status}. Database: {state.health.database}.
        </p>
      );
  }
}
