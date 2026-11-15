import { useState } from "react";
import { Link } from "react-router";
import { api, categoryLabel, errorMessage, formatDate, formatDay, formatTime, localDate, type Task } from "../api";

type Props = {
  task: Task;
  /** The active member. Only the assignee gets the button to change status. */
  meId: string;
  /** Called with the task as the server saved it. */
  onChanged: (task: Task) => void;
};

/** One task: who it is for, when, the lesson it needs, and its status. */
export default function TaskItem({ task, meId, onChanged }: Props) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mine = task.assignee.id === meId;
  const done = task.status === "done";
  const overdue = !done && task.due_date < localDate(0);

  async function setStatus(status: "open" | "done") {
    setSaving(true);
    setError(null);
    try {
      onChanged(await api.setTaskStatus(task.id, status));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <li className={`task category-${task.category}${done ? " task-done" : ""}`}>
      <div className="task-main">
        <span className="task-heading">
          {task.due_time && <span className="task-time">{formatTime(task.due_time)}</span>}
          <span className="task-title">{task.title}</span>
          <span className="category-tag">{categoryLabel(task.category)}</span>
        </span>
        <span className="muted">
          For {mine ? "you" : task.assignee.name} ·{" "}
          {done && task.completed_at ? (
            `done ${formatDate(task.completed_at)}`
          ) : overdue ? (
            <span className="result-fail">overdue since {formatDay(task.due_date)}</span>
          ) : (
            `due ${formatDay(task.due_date).toLowerCase()}`
          )}{" "}
          · assigned by {task.created_by.id === meId ? "you" : task.created_by.name}
        </span>
        <LessonNote task={task} mine={mine} />
        {error && (
          <span className="form-error" role="alert">
            {error}
          </span>
        )}
      </div>
      {mine ? (
        <button type="button" onClick={() => void setStatus(done ? "open" : "done")} disabled={saving}>
          {saving ? "Saving…" : done ? "Reopen" : "Mark as done"}
        </button>
      ) : (
        <span className={done ? "status-pill result-pass" : "status-pill"}>{done ? "✓ Done" : "Open"}</span>
      )}
    </li>
  );
}

/** Says whether the assignee has passed the lesson that prepares for the task. */
function LessonNote({ task, mine }: { task: Task; mine: boolean }) {
  const lesson = task.required_lesson;
  if (!lesson) return null;
  const link = <Link to={`/learn/${lesson.slug}`}>{lesson.title}</Link>;
  if (task.lesson_passed) {
    return (
      <span className="lesson-note result-pass">
        ✓ {mine ? "You" : task.assignee.name} passed the lesson {link}
      </span>
    );
  }
  if (task.status === "done") {
    return (
      <span className="lesson-note muted">
        Lesson for this task: {link} · {mine ? "you have" : `${task.assignee.name} has`} not passed it
      </span>
    );
  }
  return (
    <span className="lesson-note lesson-note-missing">
      {mine ? (
        <>Before you start, take the lesson {link}.</>
      ) : (
        <>
          {task.assignee.name} has not passed the lesson {link} yet.
        </>
      )}
    </span>
  );
}
