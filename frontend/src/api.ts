// Typed client for the Go API. Shapes mirror backend/internal/store and
// backend/internal/httpapi. The session lives in an HttpOnly cookie that the
// browser sends automatically; JavaScript never sees it.

export const RELATIONSHIPS = ["Mom", "Dad", "Grandma", "Grandpa", "Other"] as const;
export type Relationship = (typeof RELATIONSHIPS)[number];

export type Member = {
  id: string;
  name: string;
  relationship: Relationship;
  is_owner: boolean;
};

export type FamilyRef = {
  id: string;
  name: string;
};

/** A member created in this browser, i.e. one the header can switch to. */
export type DeviceMember = {
  id: string;
  name: string;
  relationship: Relationship;
  family_name: string;
};

export type Me = {
  member: Member;
  family: FamilyRef;
  device_members: DeviceMember[];
};

/** What GET /api/me returns for a visitor with no active member. */
export type Anonymous = { member: null };

/** A member's best result for one lesson. */
export type MemberProgress = {
  lesson_slug: string;
  passed: boolean;
  best_score: number;
  total: number;
  achieved_at: string;
};

export type FamilyMember = Member & { progress: MemberProgress[] };

export type LessonRef = { slug: string; title: string };

export type FamilyDetails = {
  family: FamilyRef;
  members: FamilyMember[];
  lessons: LessonRef[];
  invite: { token: string; path: string };
};

export type LessonSummary = {
  slug: string;
  title: string;
  related_task: string;
  /** Picture file name, or "" when the lesson has none. */
  cover_image: string;
  question_count: number;
  status: "not_started" | "in_progress" | "passed";
  best_score: number | null;
  best_total: number | null;
  achieved_at: string | null;
};

/** A lesson before answering: questions carry no answer key. */
export type Lesson = {
  slug: string;
  title: string;
  objectives: string[];
  steps: string[];
  source_title: string;
  source_url: string;
  related_task: string;
  disclaimer: string;
  cover_image: string;
  /** Empty, or one picture file name per step, in step order. */
  step_images: string[];
  questions: { prompt: string; options: string[] }[];
};

/** The server's grading of one quiz submission. */
export type AttemptResult = {
  score: number;
  total: number;
  passed: boolean;
  results: { correct: boolean; selected_index: number; correct_index: number; explanation: string }[];
};

export type TaskStatus = "open" | "done";

/** Task categories, each with its own color in the schedule. */
export const TASK_CATEGORIES = [
  { id: "care", label: "Baby care" },
  { id: "feeding", label: "Feeding" },
  { id: "health", label: "Health" },
  { id: "appointment", label: "Appointment" },
  { id: "shopping", label: "Shopping" },
] as const;
export type TaskCategory = (typeof TASK_CATEGORIES)[number]["id"];

export function categoryLabel(id: TaskCategory): string {
  return TASK_CATEGORIES.find((c) => c.id === id)?.label ?? id;
}

/**
 * A care task. required_lesson is the lesson that prepares for it and
 * lesson_passed tells whether the assignee has passed that lesson; both are
 * null for a task without a lesson.
 */
export type Task = {
  id: string;
  title: string;
  status: TaskStatus;
  category: TaskCategory;
  /** A calendar day, YYYY-MM-DD. */
  due_date: string;
  /** Time of day as HH:MM, or null for any time that day. */
  due_time: string | null;
  assignee: { id: string; name: string };
  created_by: { id: string; name: string };
  required_lesson: LessonRef | null;
  lesson_passed: boolean | null;
  completed_at: string | null;
  created_at: string;
};

export type NextLesson = {
  slug: string;
  title: string;
  related_task: string;
  cover_image: string;
  question_count: number;
  /** True when one of the member's open tasks requires this lesson. */
  needed_for_task: boolean;
};

/** The active member's day: their tasks and the lesson to take next. */
/** What the family has done so far, for the first-day checklist. */
export type Setup = {
  members: number;
  tasks: number;
  lessons_passed: number;
  tasks_done: number;
};

export type Today = {
  date: string;
  tasks: Task[];
  next_lesson: NextLesson | null;
  setup: Setup;
};

/** How much of the family has asked for the paid plan. */
export type PlanInterest = {
  interested: boolean;
  family_interested: number;
  family_members: number;
};

export type Invite = {
  family_name: string;
  invited_by: string;
};

/** State of one request, for loading and error UI. */
export type Load<T> =
  | { status: "loading" }
  | { status: "error"; error: unknown }
  | { status: "ready"; data: T };

/** An error reply in the API's {"error": {"code", "message"}} format. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: "GET" | "POST" | "PATCH", path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers:
        body === undefined
          ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network_error", "Can't reach the server. Check your connection and try again.");
  }

  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(
      res.status,
      err?.code ?? "http_error",
      err?.message ?? `The server returned an error (${res.status}). Please try again.`,
    );
  }
  if (data === null) {
    throw new ApiError(res.status, "bad_response", "The server sent an unexpected reply. Please try again.");
  }
  return data as T;
}

export const api = {
  me: () => request<Me | Anonymous>("GET", "/api/me"),
  lessons: () => request<{ lessons: LessonSummary[] }>("GET", "/api/lessons"),
  lesson: (slug: string) => request<Lesson>("GET", `/api/lessons/${encodeURIComponent(slug)}`),
  submitAttempt: (slug: string, answers: number[]) =>
    request<AttemptResult>("POST", `/api/lessons/${encodeURIComponent(slug)}/attempts`, { answers }),
  tasks: () => request<{ tasks: Task[] }>("GET", "/api/tasks"),
  createTask: (input: {
    title: string;
    assignee_id: string;
    lesson_slug: string;
    category: TaskCategory;
    due_date: string;
    due_time: string;
    /** Create the task on this many days in a row; 1 means once. */
    repeat_days: number;
  }) =>
    request<Task>("POST", "/api/tasks", input),
  setTaskStatus: (taskId: string, status: TaskStatus) =>
    request<Task>("PATCH", `/api/tasks/${encodeURIComponent(taskId)}`, { status }),
  today: (date: string) => request<Today>("GET", `/api/today?date=${encodeURIComponent(date)}`),
  planInterest: () => request<PlanInterest>("GET", "/api/plan/interest"),
  registerPlanInterest: () => request<PlanInterest>("POST", "/api/plan/interest"),
  family: () => request<FamilyDetails>("GET", "/api/family"),
  invite: (token: string) => request<Invite>("GET", `/api/invites/${encodeURIComponent(token)}`),
  createFamily: (input: { family_name: string; member_name: string; relationship: string }) =>
    request<Me>("POST", "/api/families", input),
  joinFamily: (input: { token: string; member_name: string; relationship: string }) =>
    request<Me>("POST", "/api/families/join", input),
  switchMember: (memberId: string) => request<Me>("POST", "/api/session/switch", { member_id: memberId }),
};

/** URL of a lesson picture; the files live in public/illustrations. */
export function illustrationUrl(fileName: string): string {
  return `/illustrations/${encodeURIComponent(fileName)}`;
}

/** "Nov 15" style date for an API timestamp. */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

/** The browser's own calendar day as YYYY-MM-DD, offsetDays from today. */
export function localDate(offsetDays = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** The calendar day `days` after the YYYY-MM-DD day `day`. */
export function addDays(day: string, days: number): string {
  const [y, m, d] = day.split("-").map(Number);
  const date = new Date(y, m - 1, d + days);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** "2:00 PM" for an HH:MM time of day. */
export function formatTime(time: string): string {
  const [h, m] = time.split(":").map(Number);
  return new Date(2000, 0, 1, h, m).toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}

/** "Today", "Tomorrow" or "Nov 15" for a YYYY-MM-DD calendar day. */
export function formatDay(day: string): string {
  if (day === localDate(0)) return "Today";
  if (day === localDate(1)) return "Tomorrow";
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

/** A message that is safe to show the user for any thrown value. */
export function errorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : "Something went wrong. Please try again.";
}
