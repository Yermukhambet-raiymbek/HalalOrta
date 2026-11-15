import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, errorMessage, type Load, type PlanInterest } from "../api";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

const FREE = [
  "One family space for up to 10 people, joined by a link",
  "Starter lessons with quizzes: safe sleep, bathing, crying",
  "Everyone's lesson progress in one table",
  "Shared care tasks that show whether the person doing them has passed the lesson",
  "A color-coded family schedule: feeds, health, appointments, shopping",
  "Tasks that repeat every day for a week",
  "A Today page with your tasks and your next lesson",
];

const PREMIUM = [
  "Everything in Free",
  "New lessons for each stage of the first year",
  "Routines that repeat every week without planning them again",
  "Reminders by text or email when a task is due or a lesson is waiting",
];

/** What is free today and what the paid family plan will add. Open to everyone. */
export default function PricingPage() {
  const { session } = useSession();
  return (
    <>
      <h1>Pricing</h1>
      <p className="muted">One plan per family, not per person. Grandparents and other helpers join for free.</p>

      <section className="card">
        <h2>Free</h2>
        <p className="price">
          $0 <span className="muted">· what the app does today</span>
        </p>
        <ul className="plain-list">
          {FREE.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>

      <section className="card">
        <h2>Family Premium</h2>
        <p className="price">
          $14.99 <span className="muted">per family per month, planned, not on sale yet</span>
        </p>
        <ul className="plain-list">
          {PREMIUM.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
        <p className="hint">
          Premium is not available yet and LittleVillage takes no payments. If your family would use it, say so
          below: it tells us what to build first.
        </p>
        {session.status === "ready" ? (
          <Interest key={session.me.member.id} />
        ) : (
          <p>
            <Link to="/">Create a family</Link> to tell us you are interested.
          </p>
        )}
      </section>
    </>
  );
}

/** The active member's saved interest in the paid plan, with their family's count. */
function Interest() {
  const [state, setState] = useState<Load<PlanInterest>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    api.planInterest().then(
      (data) => !cancelled && setState({ status: "ready", data }),
      (err: unknown) => !cancelled && setState({ status: "error", error: err }),
    );
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  async function register() {
    setSaving(true);
    setError(null);
    try {
      setState({ status: "ready", data: await api.registerPlanInterest() });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") {
    return <ErrorMessage message={errorMessage(state.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const { interested, family_interested: count, family_members: members } = state.data;
  return (
    <>
      {interested ? (
        <p className="readiness readiness-ok" role="status">
          ✓ Thanks, your interest is saved.
        </p>
      ) : (
        <button type="button" className="primary" onClick={() => void register()} disabled={saving}>
          {saving ? "Saving…" : "I would use Family Premium"}
        </button>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <p className="muted interest-count">
        {count} of {members} {members === 1 ? "member" : "members"} of your family {count === 1 ? "has" : "have"} said
        they would use it.
      </p>
    </>
  );
}
