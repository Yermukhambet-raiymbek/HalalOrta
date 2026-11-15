import { useEffect, useRef, useState } from "react";
import { Navigate } from "react-router";
import { api, ApiError, errorMessage, type FamilyDetails, type Load, type Me } from "../api";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

/** The active member's family: members and the invite link. */
export default function FamilyPage() {
  const { session, reload } = useSession();

  if (session.status === "loading") return <Loading />;
  if (session.status === "anonymous") return <Navigate to="/" replace />;
  if (session.status === "error") return <ErrorMessage message={session.message} onRetry={reload} />;

  // A new key per member refetches the family after a switch.
  return <FamilyView key={session.me.member.id} me={session.me} />;
}

function FamilyView({ me }: { me: Me }) {
  const { reload } = useSession();
  const [family, setFamily] = useState<Load<FamilyDetails>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setFamily({ status: "loading" });
    api.family().then(
      (data) => !cancelled && setFamily({ status: "ready", data }),
      (err: unknown) => {
        if (cancelled) return;
        // The session ended (e.g. the cookie expired): let the app send the user home.
        if (err instanceof ApiError && err.status === 401) reload();
        else setFamily({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [attempt, reload]);

  if (family.status === "loading") return <Loading text="Loading your family…" />;
  if (family.status === "error") {
    return <ErrorMessage message={errorMessage(family.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const { family: info, members, invite } = family.data;
  return (
    <>
      <h1>{info.name}</h1>

      <section className="card">
        <h2>Members</h2>
        <ul className="member-list">
          {members.map((m) => (
            <li key={m.id}>
              <span className="member-name">
                {m.name}
                {m.id === me.member.id && <span className="badge">You</span>}
              </span>
              <span className="muted">
                {m.relationship}
                {m.is_owner && " · created the family"}
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section className="card">
        <h2>Invite your family</h2>
        <p>Send this link to anyone who helps with the baby. They open it, enter their name, and join.</p>
        <InviteLink path={invite.path} />
        <p className="hint">
          Trying LittleVillage on your own? Open this link in this same browser, add a second family member, then
          switch between members in the header.
        </p>
      </section>
    </>
  );
}

function InviteLink({ path }: { path: string }) {
  const url = `${window.location.origin}${path}`;
  const inputRef = useRef<HTMLInputElement>(null);
  const [copy, setCopy] = useState<"idle" | "copied" | "failed">("idle");

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(url);
      setCopy("copied");
      setTimeout(() => setCopy("idle"), 2000);
    } catch {
      // No clipboard access (e.g. plain HTTP): select the text for a manual copy.
      inputRef.current?.select();
      setCopy("failed");
    }
  }

  return (
    <>
      <div className="invite-row">
        <input ref={inputRef} readOnly value={url} aria-label="Invite link" onFocus={(e) => e.currentTarget.select()} />
        <button type="button" onClick={() => void copyLink()}>
          {copy === "copied" ? "Copied!" : "Copy"}
        </button>
      </div>
      {copy === "failed" && (
        <p className="muted" role="status">
          Couldn't copy automatically. The link is selected: press Ctrl+C (⌘C on a Mac) to copy it.
        </p>
      )}
    </>
  );
}
