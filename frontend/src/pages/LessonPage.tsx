import { useEffect, useRef, useState, type CSSProperties, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import { api, ApiError, errorMessage, illustrationUrl, type AttemptResult, type Lesson, type Load } from "../api";
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
        <p className="swipe-hint muted">Swipe sideways to go through the steps.</p>
        <ol className="step-grid">
          {l.steps.map((s, i) => (
            <li key={s} className="step-card">
              {l.step_images[i] && (
                // Decorative: the step text next to it says the same thing.
                <img src={illustrationUrl(l.step_images[i])} alt="" width={240} height={180} loading="lazy" />
              )}
              <p>
                <span className="step-number">{i + 1}</span>
                {s}
              </p>
            </li>
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
  // One question is shown at a time.
  const [current, setCurrent] = useState(0);
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
    setCurrent(0);
    setError(null);
    topRef.current?.scrollIntoView({ block: "start" });
  }

  if (result) {
    return (
      <section className="card result-card" ref={topRef}>
        {result.passed && <Confetti />}
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
          <Link to="/today" className="button-link">
            Back to Today
          </Link>
        </div>
      </section>
    );
  }

  const total = lesson.questions.length;
  const question = lesson.questions[current];
  const isLast = current === total - 1;

  function choose(option: number) {
    setAnswers((prev) => prev.map((a, i) => (i === current ? option : a)));
    // Move on by itself after a short pause, so the choice is seen first.
    if (!isLast) window.setTimeout(() => setCurrent((c) => (c === current ? c + 1 : c)), 320);
  }

  return (
    <section className="card" ref={topRef}>
      <h2>Quiz</h2>
      <form onSubmit={(e) => void onSubmit(e)}>
        <div className="quiz-progress">
          <span>
            Question {current + 1} of {total}
          </span>
          <progress value={total - unanswered} max={total} aria-label="Questions answered" />
        </div>
        {/* A new key per question replays the slide-in. */}
        <fieldset key={current} className="question question-step" disabled={submitting}>
          <legend>{question.prompt}</legend>
          {question.options.map((option, oi) => (
            <label key={option} className="option">
              <input
                type="radio"
                name={`question-${current}`}
                checked={answers[current] === oi}
                onChange={() => choose(oi)}
              />
              <span>{option}</span>
            </label>
          ))}
        </fieldset>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="quiz-nav">
          {current > 0 && (
            <button type="button" onClick={() => setCurrent(current - 1)} disabled={submitting}>
              Back
            </button>
          )}
          {isLast ? (
            <button type="submit" className="primary" disabled={submitting || unanswered > 0}>
              {submitting ? "Checking…" : "Submit answers"}
            </button>
          ) : (
            <button type="button" onClick={() => setCurrent(current + 1)} disabled={answers[current] === null}>
              Next
            </button>
          )}
        </div>
      </form>
    </section>
  );
}

/** A short burst of paper pieces for a passed quiz. Decorative only. */
function Confetti() {
  const colors = ["#ffc533", "#7c3aed", "#16a34a", "#2563eb", "#db2777"];
  return (
    <div className="confetti" aria-hidden="true">
      {Array.from({ length: 18 }, (_, i) => (
        <i
          key={i}
          style={
            {
              "--x": `${(i * 53) % 100}%`,
              "--c": colors[i % colors.length],
              "--d": `${(i % 6) * 0.07}s`,
              "--r": `${(i % 2 ? 1 : -1) * (240 + i * 20)}deg`,
            } as CSSProperties
          }
        />
      ))}
    </div>
  );
}
