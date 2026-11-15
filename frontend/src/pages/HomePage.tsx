import { useState, type FormEvent } from "react";
import { Navigate } from "react-router";
import { api, errorMessage } from "../api";
import MemberFields from "../components/MemberFields";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

/** Welcome page with the "create a family" form. */
export default function HomePage() {
  const { session, setMe, reload } = useSession();
  const [familyName, setFamilyName] = useState("");
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (session.status === "loading") return <Loading />;
  if (session.status === "ready") return <Navigate to="/family" replace />;
  if (session.status === "error") return <ErrorMessage message={session.message} onRetry={reload} />;

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      // The new active member makes this page redirect to /family.
      setMe(await api.createFamily({ family_name: familyName, member_name: name, relationship }));
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  }

  return (
    <>
      <section className="intro">
        <h1>Care for your baby as a family</h1>
        <p>
          LittleVillage gives everyone who helps with the baby one shared family space. Start yours here, then
          invite the others with a link. No sign-up and no passwords: this browser remembers you.
        </p>
      </section>

      <section className="card">
        <h2>Create your family</h2>
        <form onSubmit={(e) => void onSubmit(e)}>
          <div className="field">
            <label htmlFor="family-name">Family name</label>
            <input
              id="family-name"
              value={familyName}
              onChange={(e) => setFamilyName(e.target.value)}
              placeholder="The Johnsons"
              maxLength={40}
              required
              disabled={submitting}
            />
          </div>
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
            {submitting ? "Creating…" : "Create family"}
          </button>
        </form>
      </section>
    </>
  );
}
