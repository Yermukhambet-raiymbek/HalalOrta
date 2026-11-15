package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"littlevillage/internal/store"
)

type todayView struct {
	Date       string            `json:"date"`
	Tasks      []store.Task      `json:"tasks"`
	NextLesson *store.NextLesson `json:"next_lesson"`
}

// day returns the UTC calendar day offset days from today as YYYY-MM-DD.
func day(offset int) string {
	return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02")
}

func (b *browser) createTask(title, assigneeID, lessonSlug, due string) store.Task {
	b.t.Helper()
	var task store.Task
	body := map[string]string{"title": title, "assignee_id": assigneeID, "lesson_slug": lessonSlug, "due_date": due}
	if status := b.call(http.MethodPost, "/api/tasks", body, &task); status != http.StatusCreated {
		b.t.Fatalf("create task %q: status %d", title, status)
	}
	return task
}

func (b *browser) tasks() []store.Task {
	b.t.Helper()
	var reply struct {
		Tasks []store.Task `json:"tasks"`
	}
	if status := b.call(http.MethodGet, "/api/tasks", nil, &reply); status != http.StatusOK {
		b.t.Fatalf("list tasks: status %d", status)
	}
	return reply.Tasks
}

func (b *browser) today(date string) todayView {
	b.t.Helper()
	var v todayView
	if status := b.call(http.MethodGet, "/api/today?date="+date, nil, &v); status != http.StatusOK {
		b.t.Fatalf("today %s: status %d", date, status)
	}
	return v
}

func (b *browser) setStatus(taskID, status string) (int, apiError) {
	b.t.Helper()
	var e apiError
	code := b.call(http.MethodPatch, "/api/tasks/"+taskID, map[string]string{"status": status}, &e)
	return code, e
}

// The demo scenario: Mom assigns a task that needs a lesson, the app shows
// whether Grandma has passed it, Grandma passes and completes the task.
func TestTaskShowsWhetherAssigneePassedTheLesson(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	mom := newBrowser(t, srv)
	mom.createFamily("Smiths", "Anna", "Mom")
	grandma := newBrowser(t, srv)
	mary := grandma.join(mom.family().Invite.Token, "Mary", "Grandma")

	task := mom.createTask("  Put baby down for a nap  ", mary.Member.ID, "test-lesson", day(0))
	if task.Title != "Put baby down for a nap" || task.Status != "open" || task.DueDate != day(0) ||
		task.Assignee.Name != "Mary" || task.CreatedBy.Name != "Anna" || task.CompletedAt != nil {
		t.Fatalf("new task: %+v", task)
	}
	if task.RequiredLesson == nil || task.RequiredLesson.Slug != "test-lesson" || task.LessonPassed == nil || *task.LessonPassed {
		t.Fatalf("new task must require the lesson and show it as not passed: %+v", task)
	}

	// A failed attempt does not count.
	grandma.attempt("test-lesson", 0, 0, 0)
	if got := mom.tasks(); len(got) != 1 || *got[0].LessonPassed {
		t.Fatalf("after a failed attempt: %+v", got)
	}
	// Passing the quiz flips the flag for everyone, with no task update.
	grandma.attempt("test-lesson", 1, 0, 3)
	if got := mom.tasks(); !*got[0].LessonPassed {
		t.Fatalf("after passing: %+v", got[0])
	}

	// A task without a lesson has no flag.
	plain := mom.createTask("Buy diapers", mary.Member.ID, "", day(0))
	if plain.RequiredLesson != nil || plain.LessonPassed != nil {
		t.Fatalf("task without a lesson: %+v", plain)
	}

	// Grandma completes the task; both see it after a fresh read.
	if code, e := grandma.setStatus(task.ID, "done"); code != http.StatusOK {
		t.Fatalf("assignee marks done: status %d %+v", code, e)
	}
	for name, b := range map[string]*browser{"mom": mom, "grandma": grandma} {
		var done *store.Task
		for _, x := range b.tasks() {
			if x.ID == task.ID {
				done = &x
			}
		}
		if done == nil || done.Status != "done" || done.CompletedAt == nil {
			t.Fatalf("%s does not see the task as done: %+v", name, done)
		}
	}
	// Reopening clears the completion time.
	var reopened store.Task
	if code := grandma.call(http.MethodPatch, "/api/tasks/"+task.ID, map[string]string{"status": "open"}, &reopened); code != http.StatusOK ||
		reopened.Status != "open" || reopened.CompletedAt != nil {
		t.Fatalf("reopen: status %d %+v", code, reopened)
	}
}

func TestOnlyAssigneeChangesTaskStatus(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	mom := newBrowser(t, srv)
	mom.createFamily("Smiths", "Anna", "Mom")
	grandma := newBrowser(t, srv)
	mary := grandma.join(mom.family().Invite.Token, "Mary", "Grandma")
	task := mom.createTask("Give baby a bath", mary.Member.ID, "", day(0))

	// The creator is in the family but is not the assignee.
	if code, e := mom.setStatus(task.ID, "done"); code != http.StatusForbidden || e.Error.Code != "forbidden" {
		t.Fatalf("creator marks done: status %d code %q", code, e.Error.Code)
	}
	if got := mom.tasks(); got[0].Status != "open" {
		t.Fatalf("task changed by a non-assignee: %+v", got[0])
	}
	if code, e := grandma.setStatus(task.ID, "finished"); code != http.StatusBadRequest || e.Error.Code != "validation_error" {
		t.Fatalf("unknown status: status %d code %q", code, e.Error.Code)
	}
	for _, id := range []string{"not-a-uuid", "00000000-0000-4000-8000-000000000000"} {
		if code, e := grandma.setStatus(id, "done"); code != http.StatusNotFound || e.Error.Code != "task_not_found" {
			t.Fatalf("task %s: status %d code %q", id, code, e.Error.Code)
		}
	}
}

func TestTasksStayInsideTheFamily(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	a := newBrowser(t, srv)
	anna := a.createFamily("Family A", "Anna", "Mom")
	b := newBrowser(t, srv)
	bob := b.createFamily("Family B", "Bob", "Dad")
	task := a.createTask("Private task of family A", anna.Member.ID, "", day(0))

	if got := b.tasks(); len(got) != 0 {
		t.Fatalf("family B sees tasks of family A: %+v", got)
	}
	if got := b.today(day(0)); len(got.Tasks) != 0 {
		t.Fatalf("family B sees family A's tasks on Today: %+v", got.Tasks)
	}
	// To another family the task does not exist.
	if code, e := b.setStatus(task.ID, "done"); code != http.StatusNotFound || e.Error.Code != "task_not_found" {
		t.Fatalf("other family changes the task: status %d code %q", code, e.Error.Code)
	}
	// A task cannot be assigned to a member of another family.
	var e apiError
	body := map[string]string{"title": "Sneaky", "assignee_id": bob.Member.ID, "due_date": day(0)}
	if code := a.call(http.MethodPost, "/api/tasks", body, &e); code != http.StatusBadRequest || e.Error.Code != "validation_error" {
		t.Fatalf("assign to another family's member: status %d code %q", code, e.Error.Code)
	}
	if got := b.tasks(); len(got) != 0 {
		t.Fatalf("a task leaked into family B: %+v", got)
	}
	if got := a.tasks(); len(got) != 1 || got[0].Status != "open" {
		t.Fatalf("family A's tasks: %+v", got)
	}
}

func TestTaskValidation(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	b := newBrowser(t, srv)
	me := b.createFamily("Validators", "Ann", "Mom")
	id := me.Member.ID

	cases := []struct {
		name string
		body map[string]string
	}{
		{"empty title", map[string]string{"title": "   ", "assignee_id": id, "due_date": day(0)}},
		{"title too long", map[string]string{"title": strings.Repeat("x", 81), "assignee_id": id, "due_date": day(0)}},
		{"no assignee", map[string]string{"title": "Task", "due_date": day(0)}},
		{"assignee is not a UUID", map[string]string{"title": "Task", "assignee_id": "mary", "due_date": day(0)}},
		{"unknown assignee", map[string]string{"title": "Task", "assignee_id": "00000000-0000-4000-8000-000000000000", "due_date": day(0)}},
		{"no due date", map[string]string{"title": "Task", "assignee_id": id}},
		{"bad due date", map[string]string{"title": "Task", "assignee_id": id, "due_date": "tomorrow"}},
		{"due date long ago", map[string]string{"title": "Task", "assignee_id": id, "due_date": "2020-01-01"}},
		{"due date too far ahead", map[string]string{"title": "Task", "assignee_id": id, "due_date": day(800)}},
		{"unknown lesson", map[string]string{"title": "Task", "assignee_id": id, "lesson_slug": "no-such-lesson", "due_date": day(0)}},
	}
	for _, tc := range cases {
		var e apiError
		if code := b.call(http.MethodPost, "/api/tasks", tc.body, &e); code != http.StatusBadRequest || e.Error.Code != "validation_error" {
			t.Errorf("%s: status %d code %q, want 400 validation_error", tc.name, code, e.Error.Code)
		}
	}
	if got := b.tasks(); len(got) != 0 {
		t.Fatalf("invalid tasks were saved: %+v", got)
	}
	// 80 characters is allowed.
	if task := b.createTask(strings.Repeat("x", 80), id, "", day(1)); task.DueDate != day(1) {
		t.Fatalf("80-character title: %+v", task)
	}
}

func TestTodayShowsMyTasksAndNextLesson(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	mom := newBrowser(t, srv)
	anna := mom.createFamily("Smiths", "Anna", "Mom")
	grandma := newBrowser(t, srv)
	mary := grandma.join(mom.family().Invite.Token, "Mary", "Grandma")

	// With no tasks, the next lesson is the first one in order.
	if got := grandma.today(day(0)); len(got.Tasks) != 0 || got.NextLesson == nil ||
		got.NextLesson.Slug != "test-lesson" || got.NextLesson.NeededForTask {
		t.Fatalf("empty today: %+v next=%+v", got.Tasks, got.NextLesson)
	}

	dueToday := mom.createTask("Due today", mary.Member.ID, "other-lesson", day(0))
	overdue := mom.createTask("Overdue", mary.Member.ID, "", day(-1))
	mom.createTask("Due tomorrow", mary.Member.ID, "", day(1))
	mom.createTask("Mom's own task", anna.Member.ID, "", day(0))

	got := grandma.today(day(0))
	if len(got.Tasks) != 2 || got.Tasks[0].ID != overdue.ID || got.Tasks[1].ID != dueToday.ID {
		t.Fatalf("grandma's today must hold the overdue task, then today's: %+v", got.Tasks)
	}
	// A lesson needed by an open task comes before earlier lessons.
	if got.NextLesson == nil || got.NextLesson.Slug != "other-lesson" || !got.NextLesson.NeededForTask {
		t.Fatalf("next lesson: %+v", got.NextLesson)
	}
	if momToday := mom.today(day(0)); len(momToday.Tasks) != 1 || momToday.Tasks[0].Title != "Mom's own task" {
		t.Fatalf("mom's today: %+v", momToday.Tasks)
	}

	// A task finished today stays on today's list; an overdue one that is
	// finished leaves it.
	grandma.setStatus(dueToday.ID, "done")
	grandma.setStatus(overdue.ID, "done")
	got = grandma.today(day(0))
	if len(got.Tasks) != 1 || got.Tasks[0].ID != dueToday.ID || got.Tasks[0].Status != "done" {
		t.Fatalf("today after finishing both: %+v", got.Tasks)
	}

	// After passing every lesson there is no next lesson.
	grandma.attempt("other-lesson", 2, 2, 2)
	grandma.attempt("test-lesson", 1, 0, 3)
	if got := grandma.today(day(0)); got.NextLesson != nil {
		t.Fatalf("next lesson after passing all: %+v", got.NextLesson)
	}

	for _, bad := range []string{"", "today", "2020-01-01", day(30)} {
		var e apiError
		if code := grandma.call(http.MethodGet, "/api/today?date="+bad, nil, &e); code != http.StatusBadRequest {
			t.Errorf("today with date %q: status %d, want 400", bad, code)
		}
	}
}
