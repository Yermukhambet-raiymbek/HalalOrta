import { useRef, useState } from "react";
import { api, errorMessage, type FamilyMember, type LessonSummary, type Load, type Me, type Task } from "../api";
import { ErrorMessage, Loading } from "./Status";
import TaskForm from "./TaskForm";

type Props = {
  me: Me;
  /** The day the form starts on, YYYY-MM-DD. */
  date?: string;
  onCreated: (task: Task) => void;
};

type FormData = { members: FamilyMember[]; lessons: LessonSummary[] };

/**
 * A floating "Add task" button that opens the task form in a panel sliding
 * in from the right (from the bottom on a phone). It is a native <dialog>,
 * so focus stays inside and Escape closes it.
 */
export default function AddTask({ me, date, onCreated }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [data, setData] = useState<Load<FormData>>({ status: "loading" });

  function load() {
    setData({ status: "loading" });
    Promise.all([api.family(), api.lessons()]).then(
      ([family, lessons]) => setData({ status: "ready", data: { members: family.members, lessons: lessons.lessons } }),
      (err: unknown) => setData({ status: "error", error: err }),
    );
  }

  function show() {
    setOpen(true);
    load();
    dialog.current?.showModal();
  }

  function close() {
    const el = dialog.current;
    if (!el) return;
    // Let the slide-out animation play before the dialog leaves the page.
    el.classList.add("closing");
    window.setTimeout(() => {
      el.classList.remove("closing");
      el.close();
      setOpen(false);
    }, 190);
  }

  return (
    <>
      <button type="button" className="fab" onClick={show}>
        <span aria-hidden="true">+</span> Add task
      </button>
      <dialog
        ref={dialog}
        className="drawer"
        aria-labelledby="add-task-title"
        onCancel={(e) => {
          e.preventDefault();
          close();
        }}
        // A click on the dimmed backdrop lands on the dialog element itself.
        onClick={(e) => e.target === e.currentTarget && close()}
      >
        <div className="drawer-panel">
          <div className="drawer-head">
            <h2 id="add-task-title">Add a task</h2>
            <button type="button" className="drawer-close" onClick={close} aria-label="Close">
              ×
            </button>
          </div>
          {open && data.status === "loading" && <Loading />}
          {open && data.status === "error" && <ErrorMessage message={errorMessage(data.error)} onRetry={load} />}
          {open && data.status === "ready" && (
            <TaskForm
              me={me}
              members={data.data.members}
              lessons={data.data.lessons}
              initialDate={date}
              onCreated={(task) => {
                onCreated(task);
                close();
              }}
            />
          )}
        </div>
      </dialog>
    </>
  );
}
