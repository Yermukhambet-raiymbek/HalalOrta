import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { api, ApiError, errorMessage, type Invite, type Load } from "../api";
import MemberFields from "../components/MemberFields";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

/** Landing page of an invite link: who invites you, and the join form. */
export default function JoinPage() {
  const { token = "" } = useParams();
  const { session, setMe } = useSession();
  const navigate = useNavigate();
  const [invite, setInvite] = useState<Load<Invite>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setInvite({ status: "loading" });
    api.invite(token).then(
      (data) => !cancelled && setInvite({ status: "ready", data }),
      (err: unknown) => !cancelled && setInvite({ status: "error", error: err }),
    );
    return () => {
      cancelled = true;
    };
  }, [token, attempt]);

  if (invite.status === "loading") return <Loading text="Checking your invite…" />;

  if (invite.status === "error") {
    if (invite.error instanceof ApiError && invite.error.status === 404) {
      return (
        <section className="card">
          <h1>This invite link isn't valid</h1>
          <p>Check that you copied the whole link, or ask the family member who invited you to send it again.</p>
          {session.status === "ready" ? (
            <Link to="/family">Go to your family</Link>
          ) : (
            <Link to="/">Start your own family</Link>
          )}
        </section>
      );
    }
    return <ErrorMessage message={errorMessage(invite.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      setMe(await api.joinFamily({ token, member_name: name, relationship }));
      navigate("/today");
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  }

  const { family_name: familyName, invited_by: invitedBy } = invite.data;
  return (
    <section className="card">
      <h1>Join {familyName}</h1>
      <p>
        <strong>{invitedBy}</strong> invited you to join their family on LittleVillage.
      </p>
      {session.status === "ready" && (
        <p className="hint">
          This browser already has <strong>{session.me.member.name}</strong>. Joining adds another family member
          here, and you can switch between members in the header.
        </p>
      )}
      <form onSubmit={(e) => void onSubmit(e)}>
        <MemberFields
          name={name}
          onNameChange={setName}
          relationship={relationship}
          onRelationshipChange={setRelationship}
          disabled={submitting}
        />
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button type="submit" className="primary" disabled={submitting}>
          {submitting ? "Joining…" : "Join family"}
        </button>
      </form>
    </section>
  );
}
