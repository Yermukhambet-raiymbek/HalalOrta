package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"littlevillage/internal/auth"
	"littlevillage/internal/store"
)

const (
	maxTaskTitleLength = 80
	dateLayout         = "2006-01-02"
	// The browser sends its own calendar day. It can differ from the
	// server's by a day in either direction, so allow a small margin.
	dateMargin = 48 * time.Hour
	// Tasks can be planned up to a year ahead.
	maxDueAhead = 366 * 24 * time.Hour
)

// timePattern is a 24-hour time of day, HH:MM.
var timePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

type taskHandlers struct {
	store *store.Store
}

type createTaskRequest struct {
	Title      string `json:"title"`
	AssigneeID string `json:"assignee_id"`
	// LessonSlug is the lesson that prepares for the task. Optional.
	LessonSlug string `json:"lesson_slug"`
	// Category is one of store.TaskCategories. Optional: defaults to "care".
	Category string `json:"category"`
	DueDate  string `json:"due_date"`
	// DueTime is the time of day as HH:MM. Optional: empty means any time.
	DueTime string `json:"due_time"`
}

type updateTaskRequest struct {
	Status string `json:"status"`
}

type todayResponse struct {
	Date       string            `json:"date"`
	Tasks      []store.Task      `json:"tasks"`
	NextLesson *store.NextLesson `json:"next_lesson"`
}

// GET /api/tasks (private): every task of the session's family.
func (h *taskHandlers) list(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	tasks, err := h.store.ListTasks(r.Context(), sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

// POST /api/tasks (private): assign a task to a member of the same family.
func (h *taskHandlers) create(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	var req createTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	title := strings.TrimSpace(req.Title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTaskTitleLength {
		writeError(w, http.StatusBadRequest, codeValidation, "Task title must be 1 to 80 characters.")
		return
	}
	if !isUUID(req.AssigneeID) {
		writeError(w, http.StatusBadRequest, codeValidation, "Choose who the task is for.")
		return
	}
	if req.Category == "" {
		req.Category = store.TaskCategories[0]
	}
	if !slices.Contains(store.TaskCategories, req.Category) {
		writeError(w, http.StatusBadRequest, codeValidation,
			"Category must be one of: "+strings.Join(store.TaskCategories, ", ")+".")
		return
	}
	now := time.Now()
	if !validDate(req.DueDate, now.Add(-dateMargin), now.Add(maxDueAhead)) {
		writeError(w, http.StatusBadRequest, codeValidation, "Due date must be today or a later day, as YYYY-MM-DD.")
		return
	}
	if req.DueTime != "" && !timePattern.MatchString(req.DueTime) {
		writeError(w, http.StatusBadRequest, codeValidation, "Time must be a time of day as HH:MM, for example 14:00.")
		return
	}

	task, err := h.store.CreateTask(r.Context(), sess.FamilyID, sess.MemberID, store.NewTask{
		Title: title, AssigneeID: req.AssigneeID, LessonSlug: req.LessonSlug,
		Category: req.Category, DueDate: req.DueDate, DueTime: req.DueTime,
	})
	switch {
	case errors.Is(err, store.ErrAssigneeNotInFamily):
		writeError(w, http.StatusBadRequest, codeValidation, "The task can only be assigned to a member of your family.")
		return
	case errors.Is(err, store.ErrLessonNotFound):
		writeError(w, http.StatusBadRequest, codeValidation, "The lesson chosen for this task does not exist.")
		return
	case errors.Is(err, store.ErrTooManyTasks):
		writeError(w, http.StatusConflict, codeTooManyTasks, "This family already has the maximum of 200 tasks.")
		return
	case err != nil:
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

// PATCH /api/tasks/{id} (private): the assignee marks the task done, or
// open again.
func (h *taskHandlers) update(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	var req updateTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Status != store.TaskOpen && req.Status != store.TaskDone {
		writeError(w, http.StatusBadRequest, codeValidation, `Status must be "open" or "done".`)
		return
	}
	taskID := chi.URLParam(r, "id")
	if !isUUID(taskID) {
		writeTaskNotFound(w)
		return
	}

	task, err := h.store.SetTaskStatus(r.Context(), sess.FamilyID, sess.MemberID, taskID, req.Status)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeTaskNotFound(w)
		return
	case errors.Is(err, store.ErrNotAssignee):
		writeError(w, http.StatusForbidden, codeForbidden, "Only the person the task is assigned to can change its status.")
		return
	case err != nil:
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// GET /api/today?date=YYYY-MM-DD (private): the active member's tasks for
// their calendar day and the lesson to take next.
func (h *taskHandlers) today(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	date := r.URL.Query().Get("date")
	now := time.Now()
	if !validDate(date, now.Add(-dateMargin), now.Add(dateMargin)) {
		writeError(w, http.StatusBadRequest, codeValidation, "date must be today's date as YYYY-MM-DD.")
		return
	}
	tasks, err := h.store.TodayTasks(r.Context(), sess.MemberID, date)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	next, err := h.store.NextLessonFor(r.Context(), sess.MemberID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, todayResponse{Date: date, Tasks: tasks, NextLesson: next})
}

func writeTaskNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, codeTaskNotFound, "This task does not exist.")
}

func isUUID(s string) bool {
	var id pgtype.UUID
	return id.Scan(s) == nil
}

// validDate reports whether s is a YYYY-MM-DD calendar day whose UTC
// midnight lies between from and to.
func validDate(s string, from, to time.Time) bool {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return false
	}
	return !d.Before(from.UTC().Truncate(24*time.Hour)) && !d.After(to)
}
