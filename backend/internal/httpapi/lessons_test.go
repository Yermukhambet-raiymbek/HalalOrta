package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"littlevillage/content"
	"littlevillage/internal/store"
)

// testLesson has a known answer key: 1, 0, 3.
var testLesson = content.Lesson{
	Slug: "test-lesson", Title: "Test Lesson",
	Objectives: []string{"objective"}, Steps: []string{"step one", "step two"},
	SourceTitle: "Source", SourceURL: "https://example.org/source",
	RelatedTask: "Test task", Disclaimer: "Not medical advice.",
	CoverImage: "cover.svg", StepImages: []string{"step-one.svg", "step-two.svg"},
	Questions: []content.Question{
		{Prompt: "Q1", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 1, Explanation: "SECRET-EXPLANATION-1"},
		{Prompt: "Q2", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 0, Explanation: "SECRET-EXPLANATION-2"},
		{Prompt: "Q3", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 3, Explanation: "SECRET-EXPLANATION-3"},
	},
}

var otherLesson = content.Lesson{
	Slug: "other-lesson", Title: "Other Lesson",
	Objectives: []string{"objective"}, Steps: []string{"step"},
	SourceTitle: "Source", SourceURL: "https://example.org/other",
	RelatedTask: "Other task", Disclaimer: "Not medical advice.",
	Questions: []content.Question{
		{Prompt: "Q1", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 2, Explanation: "e"},
		{Prompt: "Q2", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 2, Explanation: "e"},
		{Prompt: "Q3", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 2, Explanation: "e"},
	},
}

type attemptView struct {
	Score   int  `json:"score"`
	Total   int  `json:"total"`
	Passed  bool `json:"passed"`
	Results []struct {
		Correct       bool   `json:"correct"`
		SelectedIndex int    `json:"selected_index"`
		CorrectIndex  int    `json:"correct_index"`
		Explanation   string `json:"explanation"`
	} `json:"results"`
}

func (b *browser) attempt(slug string, answers ...int) attemptView {
	b.t.Helper()
	var res attemptView
	status := b.call(http.MethodPost, "/api/lessons/"+slug+"/attempts", map[string]any{"answers": answers}, &res)
	if status != http.StatusCreated {
		b.t.Fatalf("attempt %v: status %d", answers, status)
	}
	return res
}

func (b *browser) lessons() []store.LessonSummary {
	b.t.Helper()
	var reply struct {
		Lessons []store.LessonSummary `json:"lessons"`
	}
	if status := b.call(http.MethodGet, "/api/lessons", nil, &reply); status != http.StatusOK {
		b.t.Fatalf("list lessons: status %d", status)
	}
	return reply.Lessons
}

func progressOf(t *testing.T, fam familyView, memberID, slug string) *store.MemberProgress {
	t.Helper()
	for _, m := range fam.Members {
		if m.ID != memberID {
			continue
		}
		for i := range m.Progress {
			if m.Progress[i].LessonSlug == slug {
				return &m.Progress[i]
			}
		}
		return nil
	}
	t.Fatalf("member %s is not in the family reply", memberID)
	return nil
}

func TestQuizGrading(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	b := newBrowser(t, srv)
	b.createFamily("Graders", "Ann", "Mom")

	if ls := b.lessons(); len(ls) != 2 || ls[0].Slug != "test-lesson" || ls[0].Status != "not_started" ||
		ls[0].BestScore != nil || ls[0].QuestionCount != 3 {
		t.Fatalf("lessons before any attempt: %+v", ls)
	}

	// Partly correct: question 1 right, 2 and 3 wrong.
	res := b.attempt("test-lesson", 1, 2, 0)
	if res.Score != 1 || res.Total != 3 || res.Passed {
		t.Fatalf("partly correct: %+v", res)
	}
	wantCorrect := []bool{true, false, false}
	for i, r := range res.Results {
		q := testLesson.Questions[i]
		if r.Correct != wantCorrect[i] || r.CorrectIndex != q.CorrectIndex || r.Explanation != q.Explanation {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
	if ls := b.lessons(); ls[0].Status != "in_progress" || *ls[0].BestScore != 1 || *ls[0].BestTotal != 3 {
		t.Fatalf("after a failed attempt: %+v", ls[0])
	}

	// All correct passes the lesson.
	if res := b.attempt("test-lesson", 1, 0, 3); res.Score != 3 || !res.Passed {
		t.Fatalf("all correct: %+v", res)
	}
	// A worse retake does not undo the pass.
	if res := b.attempt("test-lesson", 0, 0, 0); res.Score != 1 || res.Passed {
		t.Fatalf("retake: %+v", res)
	}
	ls := b.lessons()
	if ls[0].Status != "passed" || *ls[0].BestScore != 3 || ls[0].AchievedAt == nil {
		t.Fatalf("after passing: %+v", ls[0])
	}
	if ls[1].Status != "not_started" {
		t.Fatalf("untouched lesson: %+v", ls[1])
	}
}

func TestQuizRejectsBadAnswers(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	b := newBrowser(t, srv)
	b.createFamily("Validators", "Ann", "Mom")

	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"too few answers", `{"answers":[1,0]}`, "validation_error"},
		{"too many answers", `{"answers":[1,0,3,0]}`, "validation_error"},
		{"no answers", `{"answers":[]}`, "validation_error"},
		{"answers missing", `{}`, "validation_error"},
		{"index too large", `{"answers":[1,0,4]}`, "validation_error"},
		{"negative index", `{"answers":[-1,0,3]}`, "validation_error"},
		{"null answer", `{"answers":[1,null,3]}`, "validation_error"},
		{"fractional index", `{"answers":[1,0,2.5]}`, "invalid_json"},
		{"string index", `{"answers":["1","0","3"]}`, "invalid_json"},
		{"not JSON", `answers=1,0,3`, "invalid_json"},
	}
	for _, tc := range cases {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/lessons/test-lesson/attempts", strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := b.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var e apiError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || e.Error.Code != tc.wantCode {
			t.Errorf("%s: status %d code %q, want 400 %s", tc.name, resp.StatusCode, e.Error.Code, tc.wantCode)
		}
	}
	// Nothing invalid was saved.
	if ls := b.lessons(); ls[0].Status != "not_started" {
		t.Fatalf("invalid submissions were saved: %+v", ls[0])
	}

	var e apiError
	if status := b.call(http.MethodPost, "/api/lessons/no-such-lesson/attempts", map[string]any{"answers": []int{0, 0, 0}}, &e); status != http.StatusNotFound || e.Error.Code != "lesson_not_found" {
		t.Fatalf("unknown lesson: status %d code %q", status, e.Error.Code)
	}
	if status := b.call(http.MethodGet, "/api/lessons/no-such-lesson", nil, &e); status != http.StatusNotFound {
		t.Fatalf("get unknown lesson: status %d", status)
	}
}

// A lesson without pictures answers with an empty list, not null.
func TestLessonWithoutImages(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	b := newBrowser(t, srv)
	b.createFamily("Plain", "Ann", "Mom")

	resp, err := b.http.Get(srv.URL + "/api/lessons/other-lesson")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(raw), `"step_images":[]`) || !strings.Contains(string(raw), `"cover_image":""`) {
		t.Fatalf("lesson without images: %s", raw)
	}
	if ls := b.lessons(); ls[0].CoverImage != "cover.svg" || ls[1].CoverImage != "" {
		t.Fatalf("covers in the lesson list: %+v", ls)
	}
}

func TestLessonReplyHasNoAnswerKey(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	b := newBrowser(t, srv)
	b.createFamily("Peekers", "Ann", "Mom")

	for _, path := range []string{"/api/lessons/test-lesson", "/api/lessons"} {
		resp, err := b.http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		for _, leak := range []string{"correct_index", "CorrectIndex", "explanation", "Explanation", "SECRET-EXPLANATION"} {
			if strings.Contains(string(raw), leak) {
				t.Errorf("%s leaks %q: %s", path, leak, raw)
			}
		}
		if path != "/api/lessons/test-lesson" {
			continue
		}
		// Each question has exactly the prompt and the options.
		var lesson struct {
			Title      string                       `json:"title"`
			Steps      []string                     `json:"steps"`
			CoverImage string                       `json:"cover_image"`
			StepImages []string                     `json:"step_images"`
			Questions  []map[string]json.RawMessage `json:"questions"`
		}
		if err := json.Unmarshal(raw, &lesson); err != nil {
			t.Fatal(err)
		}
		if lesson.Title != "Test Lesson" || len(lesson.Steps) != 2 || len(lesson.Questions) != 3 {
			t.Fatalf("lesson reply: %s", raw)
		}
		// One picture per step, in step order, plus the cover.
		if lesson.CoverImage != "cover.svg" || len(lesson.StepImages) != 2 || lesson.StepImages[1] != "step-two.svg" {
			t.Fatalf("lesson images: %s", raw)
		}
		for i, q := range lesson.Questions {
			if len(q) != 2 || q["prompt"] == nil || q["options"] == nil {
				t.Errorf("question %d has keys other than prompt and options: %v", i+1, q)
			}
		}
	}
}

func TestAttemptShowsInFamilyProgressOnlyForOwnFamily(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)

	mom := newBrowser(t, srv)
	momMe := mom.createFamily("Family A", "Anna", "Mom")
	token := mom.family().Invite.Token
	gran := newBrowser(t, srv)
	granMe := gran.join(token, "Mary", "Grandma")

	outsider := newBrowser(t, srv)
	outsiderMe := outsider.createFamily("Family B", "Bob", "Dad")

	// Grandma fails, then passes; the outsider fails once.
	gran.attempt("test-lesson", 0, 0, 0)
	fam := mom.family()
	if p := progressOf(t, fam, granMe.Member.ID, "test-lesson"); p == nil || p.Passed || p.BestScore != 1 || p.Total != 3 {
		t.Fatalf("after Grandma's failed attempt Mom sees %+v", p)
	}
	gran.attempt("test-lesson", 1, 0, 3)
	outsider.attempt("test-lesson", 1, 1, 1)

	fam = mom.family()
	if len(fam.Lessons) != 2 || fam.Lessons[0].Slug != "test-lesson" {
		t.Fatalf("family lessons: %+v", fam.Lessons)
	}
	p := progressOf(t, fam, granMe.Member.ID, "test-lesson")
	if p == nil || !p.Passed || p.BestScore != 3 || p.Total != 3 || p.AchievedAt.IsZero() {
		t.Fatalf("Mom sees Grandma's progress as %+v", p)
	}
	if p := progressOf(t, fam, granMe.Member.ID, "other-lesson"); p != nil {
		t.Fatalf("progress for a lesson Grandma never took: %+v", p)
	}
	if p := progressOf(t, fam, momMe.Member.ID, "test-lesson"); p != nil {
		t.Fatalf("Mom has progress without an attempt: %+v", p)
	}

	// Family A's reply holds only Family A's members.
	for _, m := range fam.Members {
		if m.ID == outsiderMe.Member.ID {
			t.Fatal("Family A's reply contains a member of Family B")
		}
	}
	// Family B sees only its own member and that member's own result.
	famB := outsider.family()
	if len(famB.Members) != 1 || famB.Members[0].ID != outsiderMe.Member.ID {
		t.Fatalf("Family B members: %+v", famB.Members)
	}
	if p := progressOf(t, famB, outsiderMe.Member.ID, "test-lesson"); p == nil || p.Passed || p.BestScore != 1 {
		t.Fatalf("outsider's own progress: %+v", p)
	}
	raw, _ := json.Marshal(famB)
	if strings.Contains(string(raw), granMe.Member.ID) || strings.Contains(string(raw), "Mary") {
		t.Fatalf("Family B's reply mentions Family A's member: %s", raw)
	}
	// The outsider's own lesson list is not affected by Grandma's pass.
	if ls := outsider.lessons(); ls[0].Status != "in_progress" {
		t.Fatalf("outsider's lesson status: %+v", ls[0])
	}
}
