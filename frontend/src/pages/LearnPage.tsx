import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, ApiError, errorMessage, formatDate, type LessonSummary, type Load } from "../api";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import { useSession } from "../session";

/** Lesson list with the active member's own status. */
export default function LearnPage() {
  // A new key per member refetches the list after a switch.
  return <RequireMember>{(me) => <LessonList key={me.member.id} name={me.member.name} />}</RequireMember>;
}

function LessonList({ name }: { name: string }) {
  const { reload } = useSession();
  const [lessons, setLessons] = useState<Load<LessonSummary[]>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLessons({ status: "loading" });
    api.lessons().then(
      (data) => !cancelled && setLessons({ status: "ready", data: data.lessons }),
      (err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) reload();
        else setLessons({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [attempt, reload]);

  if (lessons.status === "loading") return <Loading text="Loading lessons…" />;
  if (lessons.status === "error") {
    return <ErrorMessage message={errorMessage(lessons.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  return (
    <>
      <h1>Learn</h1>
      <p className="muted">
        Short lessons with a quiz. Answer every question correctly to pass. This is {name}'s progress.
      </p>
      <ul className="lesson-list">
        {lessons.data.map((l) => (
          <li key={l.slug}>
            <Link to={`/learn/${l.slug}`} className="card lesson-card">
              <span className="lesson-title">{l.title}</span>
              <span className="muted">
                {l.question_count} questions · prepares you to: {l.related_task}
              </span>
              <LessonStatus lesson={l} />
            </Link>
          </li>
        ))}
      </ul>
    </>
  );
}

function LessonStatus({ lesson }: { lesson: LessonSummary }) {
  if (lesson.status === "not_started" || lesson.best_score === null || lesson.achieved_at === null) {
    return <span className="status-pill">Not started</span>;
  }
  if (lesson.status === "passed") {
    return <span className="status-pill result-pass">✓ Passed · {formatDate(lesson.achieved_at)}</span>;
  }
  return (
    <span className="status-pill result-partial">
      Best result: {lesson.best_score}/{lesson.best_total} · not passed yet
    </span>
  );
}
