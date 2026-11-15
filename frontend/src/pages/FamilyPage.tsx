import { useEffect, useRef, useState } from "react";
import { Link, Navigate } from "react-router";
import {
  api,
  ApiError,
  errorMessage,
  formatDate,
  type FamilyDetails,
  type FamilyMember,
  type LessonRef,
  type Load,
  type Me,
} from "../api";
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

  const { family: info, members, lessons, invite } = family.data;
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
        <h2>Learning progress</h2>
        <ProgressTable members={members} lessons={lessons} />
        <p>
          <Link to="/learn">Go to lessons</Link>
        </p>
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

/** Members down the side, lessons across the top, best result in each cell. */
function ProgressTable({ members, lessons }: { members: FamilyMember[]; lessons: LessonRef[] }) {
  return (
    <div className="table-scroll">
      <table className="progress-table">
        <thead>
          <tr>
            <th scope="col">Member</th>
            {lessons.map((l) => (
              <th scope="col" key={l.slug}>
                {l.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {members.map((m) => (
            <tr key={m.id}>
              <th scope="row">{m.name}</th>
              {lessons.map((l) => {
                const p = m.progress.find((x) => x.lesson_slug === l.slug);
                if (!p) {
                  return (
                    <td key={l.slug} data-label={l.title}>
                      <span className="muted">Not started</span>
                    </td>
                  );
                }
                // data-label names the lesson when the table is stacked on a phone.
                return (
                  <td key={l.slug} data-label={l.title}>
                    <span>
                      <span className={p.passed ? "result-pass" : "result-partial"}>
                        {p.passed ? "✓ Passed" : `${p.best_score}/${p.total}`}
                      </span>
                      <span className="cell-date">{formatDate(p.achieved_at)}</span>
                    </span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
