package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const validLesson = `{
  "slug": "test-lesson",
  "title": "Test",
  "objectives": ["a"],
  "steps": ["b"],
  "source_title": "Source",
  "source_url": "https://example.org/page",
  "related_task": "Task",
  "disclaimer": "Not medical advice.",
  "questions": [
    {"prompt": "q1", "options": ["a", "b", "c", "d"], "correct_index": 0, "explanation": "e"},
    {"prompt": "q2", "options": ["a", "b", "c", "d"], "correct_index": 3, "explanation": "e"},
    {"prompt": "q3", "options": ["a", "b", "c", "d"], "correct_index": 1, "explanation": "e"}
  ]
}`

func load(files map[string]string) ([]Lesson, error) {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return Load(fsys)
}

func TestEmbeddedLessonsAreValid(t *testing.T) {
	lessons, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(lessons) == 0 {
		t.Fatal("no embedded lessons")
	}
}

func TestLoadValidLesson(t *testing.T) {
	lessons, err := load(map[string]string{"01.json": validLesson})
	if err != nil {
		t.Fatal(err)
	}
	if len(lessons) != 1 || lessons[0].Slug != "test-lesson" || lessons[0].Questions[1].CorrectIndex != 3 {
		t.Fatalf("unexpected lessons: %+v", lessons)
	}
}

func TestLoadRejectsInvalidLessonFiles(t *testing.T) {
	fourthQuestion := `{"prompt": "q3", "options": ["a", "b", "c", "d"], "correct_index": 1, "explanation": "e"}`
	cases := []struct {
		name    string
		old     string
		new     string
		wantErr string
	}{
		{"two questions", ",\n    " + fourthQuestion, "", "at least 3 questions"},
		{"three options", `"q1", "options": ["a", "b", "c", "d"]`, `"q1", "options": ["a", "b", "c"]`, "exactly 4 options"},
		{"five options", `"q1", "options": ["a", "b", "c", "d"]`, `"q1", "options": ["a", "b", "c", "d", "e"]`, "exactly 4 options"},
		{"correct_index too large", `"correct_index": 3`, `"correct_index": 4`, "out of range"},
		{"correct_index negative", `"correct_index": 3`, `"correct_index": -1`, "out of range"},
		{"correct_index missing", `"correct_index": 3, `, ``, "correct_index is missing"},
		{"empty title", `"title": "Test"`, `"title": "  "`, "title must not be empty"},
		{"empty prompt", `"prompt": "q2"`, `"prompt": ""`, "prompt must not be empty"},
		{"empty explanation", `"correct_index": 3, "explanation": "e"`, `"correct_index": 3, "explanation": ""`, "explanation must not be empty"},
		{"empty option", `"q2", "options": ["a", "b", "c", "d"]`, `"q2", "options": ["a", "", "c", "d"]`, "options: item 2"},
		{"no steps", `"steps": ["b"]`, `"steps": []`, "steps must not be empty"},
		{"no objectives", `"objectives": ["a"]`, `"objectives": []`, "objectives must not be empty"},
		{"bad slug", `"slug": "test-lesson"`, `"slug": "Test Lesson"`, "slug"},
		{"bad source url", `"https://example.org/page"`, `"example.org"`, "source_url"},
		{"two images for one step", `"steps": ["b"]`, `"steps": ["b"], "step_images": ["a.svg", "b.svg"]`, "one image per step"},
		{"step image with a folder", `"steps": ["b"]`, `"steps": ["b"], "step_images": ["../a.svg"]`, "step_images: item 1"},
		{"step image is a URL", `"steps": ["b"]`, `"steps": ["b"], "step_images": ["https://example.org/a.png"]`, "step_images: item 1"},
		{"cover image is not a picture", `"title": "Test"`, `"title": "Test", "cover_image": "cover.exe"`, "cover_image"},
		{"unknown field", `"title": "Test"`, `"title": "Test", "extra": 1`, "invalid JSON"},
		{"broken JSON", `"questions": [`, `"questions": `, "invalid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(validLesson, tc.old, tc.new, 1)
			if body == validLesson {
				t.Fatal("test case did not change the lesson")
			}
			_, err := load(map[string]string{"bad.json": body})
			if err == nil {
				t.Fatal("invalid lesson was accepted")
			}
			if !strings.Contains(err.Error(), "bad.json") || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q should name the file and contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadKeepsLessonImages(t *testing.T) {
	body := strings.Replace(validLesson, `"steps": ["b"]`, `"steps": ["b"], "step_images": ["step-one.svg"], "cover_image": "cover.jpg"`, 1)
	lessons, err := load(map[string]string{"01.json": body})
	if err != nil {
		t.Fatal(err)
	}
	if l := lessons[0]; l.CoverImage != "cover.jpg" || len(l.StepImages) != 1 || l.StepImages[0] != "step-one.svg" {
		t.Fatalf("images were not loaded: %+v", l)
	}
}

// Every picture named by an embedded lesson must exist in the frontend.
func TestEmbeddedLessonImagesExist(t *testing.T) {
	const dir = "../../frontend/public/illustrations"
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("frontend illustrations are not available here: %v", err)
	}
	lessons, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lessons {
		if len(l.StepImages) != len(l.Steps) || l.CoverImage == "" {
			t.Errorf("lesson %s: want a cover and one image per step, got cover %q and %d images for %d steps",
				l.Slug, l.CoverImage, len(l.StepImages), len(l.Steps))
		}
		for _, img := range append([]string{l.CoverImage}, l.StepImages...) {
			if _, err := os.Stat(filepath.Join(dir, img)); err != nil {
				t.Errorf("lesson %s: image %s is missing: %v", l.Slug, img, err)
			}
		}
	}
}

func TestLoadRejectsDuplicateSlugAndEmptyFolder(t *testing.T) {
	if _, err := load(map[string]string{"a.json": validLesson, "b.json": validLesson}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate slug: err = %v", err)
	}
	if _, err := load(nil); err == nil {
		t.Fatal("empty folder was accepted")
	}
}
