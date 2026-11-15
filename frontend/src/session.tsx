import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api, ApiError, errorMessage, type Me } from "./api";

/** Who is active in this browser, as reported by GET /api/me. */
export type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "ready"; me: Me }
  | { status: "error"; message: string };

type SessionContextValue = {
  session: SessionState;
  /** Use the Me returned by create, join and switch without another request. */
  setMe: (me: Me) => void;
  reload: () => void;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState>({ status: "loading" });

  const reload = useCallback(() => {
    setSession({ status: "loading" });
    api.me().then(
      (me) => setSession({ status: "ready", me }),
      (err: unknown) =>
        setSession(
          err instanceof ApiError && err.status === 401
            ? { status: "anonymous" }
            : { status: "error", message: errorMessage(err) },
        ),
    );
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const setMe = useCallback((me: Me) => setSession({ status: "ready", me }), []);
  const value = useMemo(() => ({ session, setMe, reload }), [session, setMe, reload]);

  return <SessionContext value={value}>{children}</SessionContext>;
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSession must be used inside <SessionProvider>");
  }
  return ctx;
}
