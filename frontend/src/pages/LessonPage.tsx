import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import { api, ApiError, errorMessage, type AttemptResult, type Lesson, type Load } from "../api";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

/** One lesson: what to learn, the steps, the source, then the quiz. */
export default function LessonPage() {
  const { slug = "" } = useParams();
  // A new key per member and lesson starts from a clean quiz.
  return <RequireMember>{(me) => <LessonView key={`${me.member.id}:${slug}`} slug={slug} />}</RequireMember>;
}

function LessonView({ slug }: { slug: string }) {
  const { reload } = useSession();
  const [lesson, setLesson] = useState<Load<Lesson>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLesson({ status: "loading" });
    api.lesson(slug).then(
      (data) => !cancelled && setLesson({ status: "ready", data }),
      (err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) reload();
        else setLesson({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [slug, attempt, reload]);

  if (lesson.status === "loading") return <Loading text="Loading the lesson…" />;
  if (lesson.status === "error") {
    if (lesson.error instanceof ApiError && lesson.error.status === 404) {
      return (
        <section className="card">
          <h1>Lesson not found</h1>
          <p>This lesson does not exist.</p>
          <Link to="/learn">See all lessons</Link>
        </section>
      );
    }
    return <ErrorMessage message={errorMessage(lesson.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const l = lesson.data;
  return (
    <>
      <p>
        <Link to="/learn">← All lessons</Link>
      </p>
      <h1>{l.title}</h1>
      <p className="muted">Prepares you to: {l.related_task}</p>

      <section className="card">
        <h2>What you will learn</h2>
        <ul className="plain-list">
          {l.objectives.map((o) => (
            <li key={o}>{o}</li>
          ))}
        </ul>
      </section>

      <section className="card">
        <h2>Steps</h2>
        <ol className="plain-list">
          {l.steps.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
        <p>
          Source:{" "}
          <a href={l.source_url} target="_blank" rel="noopener noreferrer">
            {l.source_title}
          </a>
        </p>
        <p className="hint">{l.disclaimer}</p>
      </section>

      <Quiz lesson={l} />
    </>
  );
}

function Quiz({ lesson }: { lesson: Lesson }) {
  const [answers, setAnswers] = useState<(number | null)[]>(() => lesson.questions.map(() => null));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<AttemptResult | null>(null);
  const topRef = useRef<HTMLElement>(null);

  const unanswered = answers.filter((a) => a === null).length;

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (answers.some((a) => a === null)) return;
    setSubmitting(true);
    setError(null);
    try {
      // The server grades the answers; the client never has the key.
      setResult(await api.submitAttempt(lesson.slug, answers as number[]));
      topRef.current?.scrollIntoView({ block: "start" });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  function tryAgain() {
    setAnswers(lesson.questions.map(() => null));
    setResult(null);
    setError(null);
    topRef.current?.scrollIntoView({ block: "start" });
  }

  if (result) {
    return (
      <section className="card" ref={topRef}>
        <h2>Your result</h2>
        <p className={`result-banner ${result.passed ? "result-pass" : "result-partial"}`} role="status">
          {result.passed
            ? `✓ Passed: ${result.score} of ${result.total} correct.`
            : `${result.score} of ${result.total} correct. Answer every question correctly to pass.`}
        </p>
        <ol className="plain-list review-list">
          {lesson.questions.map((q, i) => {
            const r = result.results[i];
            return (
              <li key={q.prompt}>
                <p className="question-prompt">{q.prompt}</p>
                <p className={r.correct ? "result-pass" : "result-fail"}>
                  {r.correct ? "✓ Correct" : "✗ Not correct"}. Your answer: {q.options[r.selected_index]}
                </p>
                {!r.correct && <p>Correct answer: {q.options[r.correct_index]}</p>}
                <p className="muted">{r.explanation}</p>
              </li>
            );
          })}
        </ol>
        <div className="button-row">
          <button type="button" onClick={tryAgain}>
            Try again
          </button>
          <Link to="/family" className="button-link">
            Back to family
          </Link>
        </div>
      </section>
    );
  }

  return (
    <section className="card" ref={topRef}>
      <h2>Quiz</h2>
      <form onSubmit={(e) => void onSubmit(e)}>
        {lesson.questions.map((q, qi) => (
          <fieldset key={q.prompt} className="question" disabled={submitting}>
            <legend>
              {qi + 1}. {q.prompt}
            </legend>
            {q.options.map((option, oi) => (
              <label key={option} className="option">
                <input
                  type="radio"
                  name={`question-${qi}`}
                  checked={answers[qi] === oi}
                  onChange={() => setAnswers((prev) => prev.map((a, i) => (i === qi ? oi : a)))}
                />
                <span>{option}</span>
              </label>
            ))}
          </fieldset>
        ))}
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        {unanswered > 0 && (
          <p className="muted">
            {unanswered === 1 ? "1 question" : `${unanswered} questions`} left to answer.
          </p>
        )}
        <button type="submit" className="primary" disabled={submitting || unanswered > 0}>
          {submitting ? "Checking…" : "Submit answers"}
        </button>
      </form>
    </section>
  );
}
