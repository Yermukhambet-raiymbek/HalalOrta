import { useEffect, useState } from "react";
import {
  addDays,
  api,
  ApiError,
  errorMessage,
  formatTime,
  localDate,
  TASK_CATEGORIES,
  type Load,
  type Me,
  type Task,
} from "../api";
import AddTask from "../components/AddTask";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import TaskItem from "../components/TaskItem";
import { useSession } from "../session";

/** The family's week: every task by day and time, colored by category. */
export default function SchedulePage() {
  // A new key per member refetches after a switch.
  return <RequireMember>{(me) => <ScheduleView key={me.member.id} me={me} />}</RequireMember>;
}

function ScheduleView({ me }: { me: Me }) {
  const { reload } = useSession();
  const [tasks, setTasks] = useState<Load<Task[]>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const today = localDate(0);
  // The seven days shown start today, so the days ahead are always in view.
  const [firstDay, setFirstDay] = useState(today);
  const [selected, setSelected] = useState(today);

  useEffect(() => {
    let cancelled = false;
    // Keep the week on screen while it is refreshed after a change.
    setTasks((prev) => (prev.status === "ready" ? prev : { status: "loading" }));
    api.tasks().then(
      (data) => !cancelled && setTasks({ status: "ready", data: data.tasks }),
      (err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) reload();
        else setTasks({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [attempt, reload]);

  if (tasks.status === "loading") return <Loading text="Loading the schedule…" />;
  if (tasks.status === "error") {
    return <ErrorMessage message={errorMessage(tasks.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const days = Array.from({ length: 7 }, (_, i) => addDays(firstDay, i));
  const byDay = (day: string) => tasks.data.filter((t) => t.due_date === day).sort(byTime);
  const showWeek = (start: string) => {
    setFirstDay(start);
    // Keep a day of the shown week selected: today if it is in it.
    setSelected(today >= start && today <= addDays(start, 6) ? today : start);
  };
  const replace = (task: Task) =>
    setTasks((prev) =>
      prev.status === "ready" ? { ...prev, data: prev.data.map((t) => (t.id === task.id ? task : t)) } : prev,
    );
  const selectedTasks = byDay(selected);

  return (
    <>
      <h1>Schedule</h1>
      <p className="muted">
        The baby's week for the whole family: who does what, and when. Colors show the kind of task.
      </p>

      <div className="week-nav">
        <button type="button" onClick={() => showWeek(addDays(firstDay, -7))} aria-label="Previous 7 days">
          ‹
        </button>
        <strong>
          {longDay(firstDay)} – {longDay(addDays(firstDay, 6))}
        </strong>
        <button type="button" onClick={() => showWeek(addDays(firstDay, 7))} aria-label="Next 7 days">
          ›
        </button>
        {firstDay !== today && (
          <button type="button" onClick={() => showWeek(today)}>
            Back to today
          </button>
        )}
      </div>

      <ul className="legend" aria-label="Task categories">
        {TASK_CATEGORIES.map((c) => (
          <li key={c.id} className={`category-${c.id}`}>
            <span className="legend-dot" />
            {c.label}
          </li>
        ))}
      </ul>

      <ol className="week-grid">
        {days.map((day) => {
          const dayTasks = byDay(day);
          const classes = ["week-day", day === today && "is-today", day === selected && "is-selected"]
            .filter(Boolean)
            .join(" ");
          return (
            <li key={day} className={classes}>
              <button
                type="button"
                className="week-day-head"
                onClick={() => setSelected(day)}
                aria-pressed={day === selected}
              >
                <span>{weekday(day)}</span>
                <strong>{dayOfMonth(day)}</strong>
              </button>
              {dayTasks.length === 0 ? (
                <span className="week-empty">No tasks</span>
              ) : (
                <ul className="chip-list">
                  {dayTasks.map((t) => (
                    <li
                      key={t.id}
                      className={`chip category-${t.category}${t.status === "done" ? " chip-done" : ""}`}
                      title={`${t.title} · ${t.assignee.name}`}
                    >
                      <span className="chip-time">{t.due_time ? formatTime(t.due_time) : "Any time"}</span>
                      <span className="chip-title">{t.title}</span>
                      <span className="chip-who">{t.assignee.name}</span>
                    </li>
                  ))}
                </ul>
              )}
            </li>
          );
        })}
      </ol>

      <section className="card">
        <h2>{longDay(selected, true)}</h2>
        {selectedTasks.length === 0 ? (
          <p className="muted">Nothing is planned for this day.</p>
        ) : (
          <ul className="task-list">
            {selectedTasks.map((t) => (
              <TaskItem key={t.id} task={t} meId={me.member.id} onChanged={replace} />
            ))}
          </ul>
        )}
      </section>

      <AddTask
        // Reopen the form on the day that is selected now.
        key={selected}
        me={me}
        date={selected}
        // Refetch: one request can create the task on several days.
        onCreated={() => setAttempt((n) => n + 1)}
      />
    </>
  );
}

/** Timed tasks in order of time, then tasks for any time of the day. */
function byTime(a: Task, b: Task): number {
  if (a.due_time === b.due_time) return a.created_at.localeCompare(b.created_at);
  if (a.due_time === null) return 1;
  if (b.due_time === null) return -1;
  return a.due_time.localeCompare(b.due_time);
}

function toDate(day: string): Date {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d);
}

function weekday(day: string): string {
  return toDate(day).toLocaleDateString("en-US", { weekday: "short" });
}

function dayOfMonth(day: string): number {
  return toDate(day).getDate();
}

function longDay(day: string, withWeekday = false): string {
  return toDate(day).toLocaleDateString(
    "en-US",
    withWeekday ? { weekday: "long", month: "long", day: "numeric" } : { month: "short", day: "numeric" },
  );
}
