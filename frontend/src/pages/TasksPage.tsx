import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router";
import {
  api,
  ApiError,
  errorMessage,
  type FamilyMember,
  type LessonSummary,
  type Load,
  type Me,
  type Task,
} from "../api";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import TaskForm from "../components/TaskForm";
import TaskItem from "../components/TaskItem";
import { useSession } from "../session";

type TasksData = { members: FamilyMember[]; lessons: LessonSummary[]; tasks: Task[] };

/** Family tasks: assign one, see who has passed the lesson it needs, mark done. */
export default function TasksPage() {
  // A new key per member refetches after a switch.
  return <RequireMember>{(me) => <TasksView key={me.member.id} me={me} />}</RequireMember>;
}

function TasksView({ me }: { me: Me }) {
  const { reload } = useSession();
  const [state, setState] = useState<Load<TasksData>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [params] = useSearchParams();

  useEffect(() => {
    let cancelled = false;
    // Keep the page on screen while it is refreshed after a change.
    setState((prev) => (prev.status === "ready" ? prev : { status: "loading" }));
    Promise.all([api.family(), api.lessons(), api.tasks()]).then(
      ([family, lessons, tasks]) =>
        !cancelled &&
        setState({ status: "ready", data: { members: family.members, lessons: lessons.lessons, tasks: tasks.tasks } }),
      (err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) reload();
        else setState({ status: "error", error: err });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [attempt, reload]);

  if (state.status === "loading") return <Loading text="Loading tasks…" />;
  if (state.status === "error") {
    return <ErrorMessage message={errorMessage(state.error)} onRetry={() => setAttempt((n) => n + 1)} />;
  }

  const { members, lessons, tasks } = state.data;
  const setTasks = (update: (tasks: Task[]) => Task[]) =>
    setState((prev) =>
      prev.status === "ready" ? { ...prev, data: { ...prev.data, tasks: update(prev.data.tasks) } } : prev,
    );
  const replace = (task: Task) => setTasks((all) => all.map((t) => (t.id === task.id ? task : t)));
  const open = tasks.filter((t) => t.status === "open");
  const done = tasks.filter((t) => t.status === "done");

  return (
    <>
      <h1>Tasks</h1>
      <p className="muted">
        Share baby care across the family. When a task has a lesson, everyone sees whether the person doing it has
        passed that lesson. See the week on the <Link to="/schedule">Schedule</Link>.
      </p>

      <section className="card">
        <h2>Assign a task</h2>
        <TaskForm
          me={me}
          members={members}
          lessons={lessons}
          // The schedule can link here with ?date=YYYY-MM-DD to plan a day.
          initialDate={params.get("date") ?? ""}
          // Refetch: one request can create the task on several days.
          onCreated={() => setAttempt((n) => n + 1)}
        />
      </section>

      <section className="card">
        <h2>To do</h2>
        {open.length === 0 ? (
          <p className="muted">No open tasks right now.</p>
        ) : (
          <ul className="task-list">
            {open.map((t) => (
              <TaskItem key={t.id} task={t} meId={me.member.id} onChanged={replace} />
            ))}
          </ul>
        )}
      </section>

      {done.length > 0 && (
        <section className="card">
          <h2>Done</h2>
          <ul className="task-list">
            {done.map((t) => (
              <TaskItem key={t.id} task={t} meId={me.member.id} onChanged={replace} />
            ))}
          </ul>
        </section>
      )}
    </>
  );
}
