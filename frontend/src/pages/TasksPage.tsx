import { useEffect, useId, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router";
import {
  addDays,
  api,
  ApiError,
  errorMessage,
  localDate,
  TASK_CATEGORIES,
  type TaskCategory,
  type FamilyMember,
  type LessonSummary,
  type Load,
  type Me,
  type Task,
} from "../api";
import RequireMember from "../components/RequireMember";
import { ErrorMessage, Loading } from "../components/Status";
import TaskItem from "../components/TaskItem";
import { useSession } from "../session";

type TasksData = { members: FamilyMember[]; lessons: LessonSummary[]; tasks: Task[] };

/** Value of the task picker for a task typed by hand. */
const CUSTOM = "custom";

/** Family tasks: assign one, see who has passed the lesson it needs, mark done. */
export default function TasksPage() {
  // A new key per member refetches after a switch.
  return <RequireMember>{(me) => <TasksView key={me.member.id} me={me} />}</RequireMember>;
}

function TasksView({ me }: { me: Me }) {
  const { reload } = useSession();
  const [state, setState] = useState<Load<TasksData>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
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
          onCreated={(task) => setTasks((all) => [task, ...all])}
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

type FormProps = {
  me: Me;
  members: FamilyMember[];
  lessons: LessonSummary[];
  onCreated: (task: Task) => void;
};

function TaskForm({ me, members, lessons, onCreated }: FormProps) {
  const id = useId();
  // Each lesson prepares for one task; those tasks are the suggestions.
  const [choice, setChoice] = useState(lessons[0]?.slug ?? CUSTOM);
  const [customTitle, setCustomTitle] = useState("");
  const [customLesson, setCustomLesson] = useState("");
  const [assigneeId, setAssigneeId] = useState(members.find((m) => m.id !== me.member.id)?.id ?? me.member.id);
  const [customCategory, setCustomCategory] = useState<TaskCategory>("care");
  // The schedule links here with ?date=YYYY-MM-DD to plan a specific day.
  const [params] = useSearchParams();
  const today = localDate(0);
  const linkedDate = params.get("date") ?? "";
  const [dueDate, setDueDate] = useState(/^\d{4}-\d{2}-\d{2}$/.test(linkedDate) && linkedDate >= today ? linkedDate : today);
  const [dueTime, setDueTime] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const suggested = lessons.find((l) => l.slug === choice);
  const title = suggested ? suggested.related_task : customTitle;
  const lessonSlug = suggested ? suggested.slug : customLesson;
  // Tasks that come from a lesson are baby care; other tasks choose.
  const category: TaskCategory = suggested ? "care" : customCategory;
  const lesson = lessons.find((l) => l.slug === lessonSlug);
  const assignee = members.find((m) => m.id === assigneeId);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      onCreated(
        await api.createTask({
          title,
          assignee_id: assigneeId,
          lesson_slug: lessonSlug,
          category,
          due_date: dueDate,
          due_time: dueTime,
        }),
      );
      setCustomTitle("");
      setDueTime("");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={(e) => void onSubmit(e)}>
      <div className="field">
        <label htmlFor={`${id}-task`}>Task</label>
        <select id={`${id}-task`} value={choice} onChange={(e) => setChoice(e.target.value)} disabled={saving}>
          {lessons.map((l) => (
            <option key={l.slug} value={l.slug}>
              {l.related_task}
            </option>
          ))}
          <option value={CUSTOM}>Another task…</option>
        </select>
      </div>

      {!suggested && (
        <>
          <div className="field">
            <label htmlFor={`${id}-title`}>What needs to be done</label>
            <input
              id={`${id}-title`}
              value={customTitle}
              onChange={(e) => setCustomTitle(e.target.value)}
              maxLength={80}
              required
              placeholder="Buy diapers"
              disabled={saving}
            />
          </div>
          <div className="field">
            <label htmlFor={`${id}-category`}>Category</label>
            <select
              id={`${id}-category`}
              value={customCategory}
              onChange={(e) => setCustomCategory(e.target.value as TaskCategory)}
              disabled={saving}
            >
              {TASK_CATEGORIES.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.label}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor={`${id}-lesson`}>Lesson that prepares for it</label>
            <select
              id={`${id}-lesson`}
              value={customLesson}
              onChange={(e) => setCustomLesson(e.target.value)}
              disabled={saving}
            >
              <option value="">No lesson needed</option>
              {lessons.map((l) => (
                <option key={l.slug} value={l.slug}>
                  {l.title}
                </option>
              ))}
            </select>
          </div>
        </>
      )}

      <div className="field">
        <label htmlFor={`${id}-assignee`}>Who does it</label>
        <select
          id={`${id}-assignee`}
          value={assigneeId}
          onChange={(e) => setAssigneeId(e.target.value)}
          disabled={saving}
        >
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.id === me.member.id ? `${m.name} (you)` : `${m.name} (${m.relationship})`}
            </option>
          ))}
        </select>
      </div>

      <div className="field-row">
        <div className="field">
          <label htmlFor={`${id}-date`}>Day</label>
          <input
            id={`${id}-date`}
            type="date"
            value={dueDate}
            min={today}
            max={addDays(today, 365)}
            onChange={(e) => setDueDate(e.target.value)}
            required
            disabled={saving}
          />
        </div>
        <div className="field">
          <label htmlFor={`${id}-time`}>
            Time <span className="muted optional">optional</span>
          </label>
          <input
            id={`${id}-time`}
            type="time"
            value={dueTime}
            onChange={(e) => setDueTime(e.target.value)}
            disabled={saving}
          />
        </div>
      </div>

      {lesson && assignee && (
        <Readiness
          lessonTitle={lesson.title}
          lessonSlug={lesson.slug}
          assignee={assignee}
          isMe={assignee.id === me.member.id}
        />
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="primary" disabled={saving}>
        {saving ? "Assigning…" : "Assign task"}
      </button>
    </form>
  );
}

/** Before assigning: has this person passed the lesson the task needs? */
function Readiness({
  lessonTitle,
  lessonSlug,
  assignee,
  isMe,
}: {
  lessonTitle: string;
  lessonSlug: string;
  assignee: FamilyMember;
  isMe: boolean;
}) {
  const passed = assignee.progress.some((p) => p.lesson_slug === lessonSlug && p.passed);
  const who = isMe ? "You have" : `${assignee.name} has`;
  return passed ? (
    <p className="readiness readiness-ok" role="status">
      ✓ {who} passed the lesson “{lessonTitle}”.
    </p>
  ) : (
    <p className="readiness readiness-missing" role="status">
      {who} not passed the lesson “{lessonTitle}” yet. You can still assign the task:{" "}
      {isMe ? "you" : assignee.name} will see the lesson first on {isMe ? "your" : "their"} Today page.
    </p>
  );
}
