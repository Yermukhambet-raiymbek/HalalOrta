package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// MaxTasks is the most tasks one family can have.
const MaxTasks = 200

// MaxRepeatDays is the most days in a row one task can be repeated on.
const MaxRepeatDays = 7

// TaskCategories are the allowed values of tasks.category. The first one
// is the default.
var TaskCategories = []string{"care", "feeding", "health", "appointment", "shopping"}

// Task statuses.
const (
	TaskOpen = "open"
	TaskDone = "done"
)

var (
	// ErrAssigneeNotInFamily means the assignee is not a member of the family.
	ErrAssigneeNotInFamily = errors.New("assignee is not in the family")
	// ErrLessonNotFound means the task names a lesson that does not exist.
	ErrLessonNotFound = errors.New("lesson not found")
	// ErrTooManyTasks means the family already has MaxTasks tasks.
	ErrTooManyTasks = errors.New("too many tasks")
	// ErrNotAssignee means only the task's assignee may do this.
	ErrNotAssignee = errors.New("not the task's assignee")
)

// TaskMember names a member on a task.
type TaskMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Task is a care task. RequiredLesson is the lesson that prepares for it,
// and LessonPassed tells whether the assignee has passed that lesson; both
// are nil for a task without a lesson. LessonPassed is computed from quiz
// attempts on every read, so it changes as soon as the assignee passes.
type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Category string `json:"category"`
	DueDate  string `json:"due_date"` // YYYY-MM-DD
	// DueTime is the time of day as HH:MM, or nil for "any time that day".
	DueTime        *string    `json:"due_time"`
	Assignee       TaskMember `json:"assignee"`
	CreatedBy      TaskMember `json:"created_by"`
	RequiredLesson *LessonRef `json:"required_lesson"`
	LessonPassed   *bool      `json:"lesson_passed"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// NewTask is the input of CreateTask. LessonSlug may be empty.
type NewTask struct {
	Title      string
	AssigneeID string
	LessonSlug string
	Category   string
	DueDate    string // YYYY-MM-DD
	DueTime    string // HH:MM, or "" for any time
	// RepeatDays is how many days in a row to create the task on, starting
	// with DueDate. 0 and 1 both mean once.
	RepeatDays int
}

// Setup counts what a family has done so far. The first-day checklist is
// built from it.
type Setup struct {
	Members       int `json:"members"`
	Tasks         int `json:"tasks"`
	LessonsPassed int `json:"lessons_passed"`
	TasksDone     int `json:"tasks_done"`
}

// NextLesson is the lesson a member should take next.
type NextLesson struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	RelatedTask   string `json:"related_task"`
	CoverImage    string `json:"cover_image"`
	QuestionCount int    `json:"question_count"`
	// NeededForTask is true when one of the member's open tasks requires it.
	NeededForTask bool `json:"needed_for_task"`
}

const taskSelect = `
	SELECT t.id, t.title, t.status, t.category, to_char(t.due_date, 'YYYY-MM-DD'),
	       to_char(t.due_time, 'HH24:MI'),
	       a.id, a.name, c.id, c.name, l.slug, l.title,
	       CASE WHEN l.id IS NULL THEN NULL ELSE EXISTS (
	           SELECT 1 FROM quiz_attempts q
	           WHERE q.member_id = t.assignee_id AND q.lesson_id = l.id AND q.passed
	       ) END,
	       t.completed_at, t.created_at
	FROM tasks t
	JOIN members a ON a.id = t.assignee_id
	JOIN members c ON c.id = t.created_by
	LEFT JOIN lessons l ON l.id = t.lesson_id`

func scanTask(row pgx.CollectableRow) (Task, error) {
	var t Task
	var lessonSlug, lessonTitle *string
	err := row.Scan(&t.ID, &t.Title, &t.Status, &t.Category, &t.DueDate, &t.DueTime,
		&t.Assignee.ID, &t.Assignee.Name, &t.CreatedBy.ID, &t.CreatedBy.Name,
		&lessonSlug, &lessonTitle, &t.LessonPassed, &t.CompletedAt, &t.CreatedAt)
	if lessonSlug != nil && lessonTitle != nil {
		t.RequiredLesson = &LessonRef{Slug: *lessonSlug, Title: *lessonTitle}
	}
	return t, err
}

// ListTasks returns every task of the family: open ones first by due date
// and time, then done ones, most recently completed first. Callers must pass the
// family ID from the session.
func (s *Store) ListTasks(ctx context.Context, familyID string) ([]Task, error) {
	rows, err := s.pool.Query(ctx, taskSelect+`
		WHERE t.family_id = $1
		ORDER BY (t.status = 'done'), CASE WHEN t.status = 'open' THEN t.due_date END,
		         CASE WHEN t.status = 'open' THEN t.due_time END NULLS LAST,
		         t.completed_at DESC NULLS LAST, t.created_at`, familyID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTask)
}

// TodayTasks returns the member's tasks for the calendar day date
// (YYYY-MM-DD): open tasks due that day or earlier, and tasks due that day
// that are already done. Earlier days come first, then the time of day;
// tasks without a time come last within their day.
func (s *Store) TodayTasks(ctx context.Context, memberID, date string) ([]Task, error) {
	rows, err := s.pool.Query(ctx, taskSelect+`
		WHERE t.assignee_id = $1
		  AND ((t.status = 'open' AND t.due_date <= $2::date) OR t.due_date = $2::date)
		ORDER BY t.due_date, t.due_time NULLS LAST, t.created_at`, memberID, date)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTask)
}

// taskByID returns one task of the family, or ErrNotFound.
func (s *Store) taskByID(ctx context.Context, familyID, taskID string) (Task, error) {
	rows, err := s.pool.Query(ctx, taskSelect+` WHERE t.family_id = $1 AND t.id = $2`, familyID, taskID)
	if err != nil {
		return Task{}, err
	}
	task, err := pgx.CollectExactlyOneRow(rows, scanTask)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

// CreateTask adds an open task to the family, once or on several days in a
// row, and returns the first one. The assignee must be a member of that
// family. It returns ErrAssigneeNotInFamily, ErrLessonNotFound or
// ErrTooManyTasks when the input does not fit.
func (s *Store) CreateTask(ctx context.Context, familyID, createdBy string, in NewTask) (Task, error) {
	days := max(in.RepeatDays, 1)
	var taskID string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Lock the family row so concurrent requests cannot pass the limit.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM families WHERE id = $1 FOR UPDATE`, familyID); err != nil {
			return err
		}
		var inFamily bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM members WHERE id = $1 AND family_id = $2)`,
			in.AssigneeID, familyID).Scan(&inFamily); err != nil {
			return err
		}
		if !inFamily {
			return ErrAssigneeNotInFamily
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE family_id = $1`, familyID).Scan(&count); err != nil {
			return err
		}
		if count+days > MaxTasks {
			return ErrTooManyTasks
		}
		var dueTime *string
		if in.DueTime != "" {
			dueTime = &in.DueTime
		}
		var lessonID *string
		if in.LessonSlug != "" {
			var id string
			err := tx.QueryRow(ctx, `SELECT id FROM lessons WHERE slug = $1`, in.LessonSlug).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrLessonNotFound
			}
			if err != nil {
				return err
			}
			lessonID = &id
		}
		for day := 0; day < days; day++ {
			var id string
			if err := tx.QueryRow(ctx, `
				INSERT INTO tasks (family_id, title, lesson_id, assignee_id, created_by, category, due_date, due_time)
				VALUES ($1, $2, $3, $4, $5, $6, $7::date + $9::int, $8::time)
				RETURNING id`,
				familyID, in.Title, lessonID, in.AssigneeID, createdBy, in.Category, in.DueDate, dueTime, day,
			).Scan(&id); err != nil {
				return err
			}
			if day == 0 {
				taskID = id
			}
		}
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return s.taskByID(ctx, familyID, taskID)
}

// SetTaskStatus marks a task done or open again. Only the assignee may do
// it: ErrNotAssignee for other members of the family, ErrNotFound when the
// task is not in the family at all.
func (s *Store) SetTaskStatus(ctx context.Context, familyID, memberID, taskID, status string) (Task, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tasks
		SET status = $1::text,
		    completed_at = CASE
		        WHEN $1::text = 'open' THEN NULL
		        WHEN status = 'done' THEN completed_at
		        ELSE now()
		    END
		WHERE id = $2 AND family_id = $3 AND assignee_id = $4`,
		status, taskID, familyID, memberID)
	if err != nil {
		return Task{}, err
	}
	task, err := s.taskByID(ctx, familyID, taskID)
	if err != nil {
		return Task{}, err
	}
	if tag.RowsAffected() == 0 {
		return Task{}, ErrNotAssignee
	}
	return task, nil
}

// NextLessonFor returns the first lesson the member has not passed yet,
// preferring one that an open task of theirs requires. It returns nil when
// every lesson is passed.
func (s *Store) NextLessonFor(ctx context.Context, memberID string) (*NextLesson, error) {
	var n NextLesson
	err := s.pool.QueryRow(ctx, `
		SELECT l.slug, l.title, l.related_task, l.cover_image,
		       (SELECT count(*) FROM questions q WHERE q.lesson_id = l.id),
		       EXISTS (
		           SELECT 1 FROM tasks t
		           WHERE t.assignee_id = $1 AND t.lesson_id = l.id AND t.status = 'open'
		       ) AS needed
		FROM lessons l
		WHERE NOT EXISTS (
		    SELECT 1 FROM quiz_attempts a
		    WHERE a.member_id = $1 AND a.lesson_id = l.id AND a.passed
		)
		ORDER BY needed DESC, l.position
		LIMIT 1`, memberID,
	).Scan(&n.Slug, &n.Title, &n.RelatedTask, &n.CoverImage, &n.QuestionCount, &n.NeededForTask)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// FamilySetup counts the family's members, tasks, finished tasks and
// passed lessons (one per member and lesson). Callers must pass the family
// ID from the session.
func (s *Store) FamilySetup(ctx context.Context, familyID string) (Setup, error) {
	var st Setup
	err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM members WHERE family_id = $1),
		       (SELECT count(*) FROM tasks WHERE family_id = $1),
		       (SELECT count(DISTINCT (a.member_id, a.lesson_id))
		        FROM quiz_attempts a JOIN members m ON m.id = a.member_id
		        WHERE m.family_id = $1 AND a.passed),
		       (SELECT count(*) FROM tasks WHERE family_id = $1 AND status = 'done')`,
		familyID,
	).Scan(&st.Members, &st.Tasks, &st.LessonsPassed, &st.TasksDone)
	return st, err
}
