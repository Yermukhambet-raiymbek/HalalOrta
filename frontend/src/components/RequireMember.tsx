import type { ReactNode } from "react";
import { Navigate } from "react-router";
import type { Me } from "../api";
import { useSession } from "../session";
import { ErrorMessage, Loading } from "./Status";

/** Renders a page only for an active member; visitors go to the start page. */
export default function RequireMember({ children }: { children: (me: Me) => ReactNode }) {
  const { session, reload } = useSession();
  if (session.status === "loading") return <Loading />;
  if (session.status === "anonymous") return <Navigate to="/" replace />;
  if (session.status === "error") return <ErrorMessage message={session.message} onRetry={reload} />;
  return <>{children(session.me)}</>;
}
