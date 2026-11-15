package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"littlevillage/content"
	"littlevillage/internal/auth"
	"littlevillage/internal/store"
)

type lessonHandlers struct {
	store *store.Store
}

// lessonResponse is a lesson as the client sees it before answering:
// questions carry no correct_index and no explanation.
type lessonResponse struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Objectives  []string `json:"objectives"`
	Steps       []string `json:"steps"`
	SourceTitle string   `json:"source_title"`
	SourceURL   string   `json:"source_url"`
	RelatedTask string   `json:"related_task"`
	Disclaimer  string   `json:"disclaimer"`
	CoverImage  string   `json:"cover_image"`
	// StepImages is empty or has one image file name per step.
	StepImages []string         `json:"step_images"`
	Questions  []publicQuestion `json:"questions"`
}

type publicQuestion struct {
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
}

type attemptRequest struct {
	// Pointers tell a missing or null answer apart from option 0.
	Answers []*int `json:"answers"`
}

type attemptResponse struct {
	Score   int              `json:"score"`
	Total   int              `json:"total"`
	Passed  bool             `json:"passed"`
	Results []questionResult `json:"results"`
}

type questionResult struct {
	Correct       bool   `json:"correct"`
	SelectedIndex int    `json:"selected_index"`
	CorrectIndex  int    `json:"correct_index"`
	Explanation   string `json:"explanation"`
}

// GET /api/lessons (private)
func (h *lessonHandlers) list(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	lessons, err := h.store.ListLessons(r.Context(), sess.MemberID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lessons": lessons})
}

// GET /api/lessons/{slug} (private)
func (h *lessonHandlers) get(w http.ResponseWriter, r *http.Request) {
	lesson, ok := h.lesson(w, r)
	if !ok {
		return
	}
	questions := make([]publicQuestion, len(lesson.Questions))
	for i, q := range lesson.Questions {
		questions[i] = publicQuestion{Prompt: q.Prompt, Options: q.Options}
	}
	writeJSON(w, http.StatusOK, lessonResponse{
		Slug: lesson.Slug, Title: lesson.Title, Objectives: lesson.Objectives, Steps: lesson.Steps,
		SourceTitle: lesson.SourceTitle, SourceURL: lesson.SourceURL, RelatedTask: lesson.RelatedTask,
		Disclaimer: lesson.Disclaimer, CoverImage: lesson.CoverImage, StepImages: lesson.StepImages,
		Questions: questions,
	})
}

// POST /api/lessons/{slug}/attempts (private). Grading happens here, on
// the server; the answer key is revealed only in this reply.
func (h *lessonHandlers) attempt(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	var req attemptRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	lesson, ok := h.lesson(w, r)
	if !ok {
		return
	}

	answers, problem := validateAnswers(req.Answers, lesson.Questions)
	if problem != "" {
		writeError(w, http.StatusBadRequest, codeValidation, problem)
		return
	}
	resp := grade(lesson.Questions, answers)
	if err := h.store.SaveAttempt(r.Context(), sess.MemberID, lesson.ID, resp.Score, resp.Total, answers); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// lesson loads the lesson named in the URL, or writes a 404.
func (h *lessonHandlers) lesson(w http.ResponseWriter, r *http.Request) (store.Lesson, bool) {
	lesson, err := h.store.LessonBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeLessonNotFound, "This lesson does not exist.")
		return store.Lesson{}, false
	}
	if err != nil {
		writeInternal(w, r, err)
		return store.Lesson{}, false
	}
	return lesson, true
}

// validateAnswers checks there is one in-range answer per question.
func validateAnswers(raw []*int, questions []content.Question) (answers []int, problem string) {
	if len(raw) != len(questions) {
		return nil, fmt.Sprintf("This quiz has %d questions, but %d answers were sent.", len(questions), len(raw))
	}
	answers = make([]int, len(raw))
	for i, a := range raw {
		if a == nil || *a < 0 || *a >= len(questions[i].Options) {
			return nil, fmt.Sprintf("The answer to question %d must be an option number from 0 to %d.", i+1, len(questions[i].Options)-1)
		}
		answers[i] = *a
	}
	return answers, ""
}

// grade compares answers with the key. The lesson is passed only when
// every answer is correct.
func grade(questions []content.Question, answers []int) attemptResponse {
	resp := attemptResponse{Total: len(questions), Results: make([]questionResult, len(questions))}
	for i, q := range questions {
		correct := answers[i] == q.CorrectIndex
		if correct {
			resp.Score++
		}
		resp.Results[i] = questionResult{
			Correct: correct, SelectedIndex: answers[i], CorrectIndex: q.CorrectIndex, Explanation: q.Explanation,
		}
	}
	resp.Passed = resp.Score == resp.Total
	return resp
}
