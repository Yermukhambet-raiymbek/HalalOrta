import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, ApiError, errorMessage, illustrationUrl, localDate, type Load, type Me, type Today } from "../api";
import AddTask from "../components/AddTask";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import TaskItem from "../components/TaskItem";
import { useSession } from "../session";

/** The active member's day: tasks to do and the lesson to take next. */
export default function TodayPage() {
  // A new key per member refetches the day after a switch.
  return <RequireMember>{(me) => <TodayView key={me.member.id} me={me} />}</RequireMember>;
}

function TodayView({ me }: { me: Me }) {
  const { reload } = useSession();
  const [today, setToday] = useState<Load<Today>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    // Keep showing the current day while it is refreshed after a change.
    setToday((prev) => (prev.status === "ready" ? prev : { status: "loading" }));
    api.today(localDate(0)).then(
      (data) => !cancelled && setToday({ status: "ready", data }),
      (err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) reload();
        else setToday({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [attempt, reload]);

  if (today.status === "loading") return <Loading text="Loading your day…" />;
  if (today.status === "error") {
    return <ErrorMessage message={errorMessage(today.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const { tasks, next_lesson: next } = today.data;
  const open = tasks.filter((t) => t.status === "open").length;
  const dateLabel = new Date().toLocaleDateString("en-US", { weekday: "long", month: "long", day: "numeric" });

  return (
    <>
      <h1>Today</h1>
      <p className="muted">
        {dateLabel}. {me.member.name}, {summary(tasks.length, open)}
      </p>

      <div className="today-grid">
      <section className="card">
        <h2>Your day</h2>
        {tasks.length === 0 ? (
          <p className="muted">
            Nothing is assigned to you for today. Anyone in the family can give you a task with the Add task
            button.
          </p>
        ) : (
          <ul className="task-list">
            {tasks.map((t) => (
              <TaskItem
                key={t.id}
                task={t}
                meId={me.member.id}
                // Refetch: finishing a task can change the next lesson.
                onChanged={() => setAttempt((n) => n + 1)}
              />
            ))}
          </ul>
        )}
        <p>
          See the whole <Link to="/schedule">family schedule</Link>.
        </p>
      </section>

      <section className="card">
        <h2>Your next lesson</h2>
        {next ? (
          <div className="next-lesson">
            {next.cover_image && (
              <img src={illustrationUrl(next.cover_image)} alt="" width={240} height={180} className="lesson-cover" />
            )}
            <div>
              <p>
                <strong>{next.title}</strong>
                <br />
                <span className="muted">
                  {next.question_count} questions.{" "}
                  {next.needed_for_task
                    ? "One of your open tasks needs this lesson."
                    : `Prepares you to: ${next.related_task}.`}
                </span>
              </p>
              <div className="button-row">
                <Link to={`/learn/${next.slug}`} className="button-link">
                  Start lesson
                </Link>
              </div>
            </div>
          </div>
        ) : (
          <p className="result-pass">✓ You have passed every lesson.</p>
        )}
      </section>
      </div>

      <AddTask me={me} onCreated={() => setAttempt((n) => n + 1)} />
    </>
  );
}

function summary(total: number, open: number): string {
  if (total === 0) return "you have no tasks today.";
  if (open === 0) return "all your tasks for today are done.";
  return open === 1 ? "you have 1 task to do." : `you have ${open} tasks to do.`;
}
