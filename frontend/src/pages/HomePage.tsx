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
  // A member who just created the family goes to the family page, where
  // the invite link is. A returning member goes to their day.
  if (session.status === "ready") return <Navigate to={submitting ? "/family" : "/today"} replace />;
  if (session.status === "error") return <ErrorMessage message={session.message} onRetry={reload} />;

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      // The new active member makes this page redirect to /family.
      // submitting stays true so the redirect knows the family is new.
      setMe(await api.createFamily({ family_name: familyName, member_name: name, relationship }));
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  }

  return (
    <>
      <section className="hero">
        <div>
          <h1>Care for your baby as a family</h1>
          <p className="hero-lead">
            LittleVillage gets parents, grandparents and other helpers ready to care for the baby, together.
          </p>
          <p className="muted">No sign-up and no passwords: this browser remembers you.</p>
        </div>
        <img
          src="/illustrations/family.svg"
          alt="A grandmother, a mother and a father with a baby under one roof"
          width={480}
          height={260}
        />
      </section>

      <ol className="feature-list">
        <li>
          <span className="step-number">1</span>
          <strong>Learn together</strong>
          <span>Short illustrated lessons on baby care, each with a quiz.</span>
        </li>
        <li>
          <span className="step-number">2</span>
          <strong>See who is prepared</strong>
          <span>Everyone's results in one family table.</span>
        </li>
        <li>
          <span className="step-number">3</span>
          <strong>Share the care</strong>
          <span>Each task shows whether the person doing it has passed the lesson for it.</span>
        </li>
      </ol>

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
