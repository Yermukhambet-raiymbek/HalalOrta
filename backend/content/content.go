// Package content embeds the lesson files and validates them. Lessons are
// JSON files in lessons/, one per lesson, loaded in file-name order. The
// server refuses to start when a file is invalid.
package content

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

//go:embed lessons/*.json
var embedded embed.FS

const (
	// MinQuestions is the fewest questions a lesson quiz may have.
	MinQuestions = 3
	// OptionsPerQuestion is the exact number of answer options.
	OptionsPerQuestion = 4
)

// Lesson is one validated lesson with its quiz, including the answer key.
type Lesson struct {
	Slug        string
	Title       string
	Objectives  []string
	Steps       []string
	SourceTitle string
	SourceURL   string
	RelatedTask string
	Disclaimer  string
	Questions   []Question
}

// Question is one quiz question. CorrectIndex and Explanation are the
// answer key and must never be sent to the client before grading.
type Question struct {
	Prompt       string
	Options      []string
	CorrectIndex int
	Explanation  string
}

// lessonFile mirrors the JSON format. correct_index is a pointer so a
// missing key is detected instead of silently becoming 0.
type lessonFile struct {
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Objectives  []string       `json:"objectives"`
	Steps       []string       `json:"steps"`
	SourceTitle string         `json:"source_title"`
	SourceURL   string         `json:"source_url"`
	RelatedTask string         `json:"related_task"`
	Disclaimer  string         `json:"disclaimer"`
	Questions   []questionFile `json:"questions"`
}

type questionFile struct {
	Prompt       string   `json:"prompt"`
	Options      []string `json:"options"`
	CorrectIndex *int     `json:"correct_index"`
	Explanation  string   `json:"explanation"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// LoadEmbedded loads the lessons built into the binary.
func LoadEmbedded() ([]Lesson, error) {
	sub, err := fs.Sub(embedded, "lessons")
	if err != nil {
		return nil, err
	}
	return Load(sub)
}

// Load reads and validates every *.json file in fsys, in file-name order.
// It fails on the first invalid file, naming the file and the problem.
func Load(fsys fs.FS) ([]Lesson, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("no lesson files found")
	}
	sort.Strings(names)

	lessons := make([]Lesson, 0, len(names))
	seen := make(map[string]string)
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("lesson file %s: %w", name, err)
		}
		lesson, err := parse(raw)
		if err != nil {
			return nil, fmt.Errorf("lesson file %s: %w", name, err)
		}
		if other, dup := seen[lesson.Slug]; dup {
			return nil, fmt.Errorf("lesson file %s: slug %q is already used by %s", name, lesson.Slug, other)
		}
		seen[lesson.Slug] = name
		lessons = append(lessons, lesson)
	}
	return lessons, nil
}

func parse(raw []byte) (Lesson, error) {
	var f lessonFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Lesson{}, fmt.Errorf("invalid JSON: %w", err)
	}

	if !slugPattern.MatchString(f.Slug) {
		return Lesson{}, fmt.Errorf("slug %q must be lowercase letters, digits and hyphens", f.Slug)
	}
	for field, value := range map[string]string{
		"title": f.Title, "source_title": f.SourceTitle, "related_task": f.RelatedTask, "disclaimer": f.Disclaimer,
	} {
		if strings.TrimSpace(value) == "" {
			return Lesson{}, fmt.Errorf("%s must not be empty", field)
		}
	}
	if !strings.HasPrefix(f.SourceURL, "https://") {
		return Lesson{}, fmt.Errorf("source_url %q must start with https://", f.SourceURL)
	}
	if err := nonEmptyList("objectives", f.Objectives); err != nil {
		return Lesson{}, err
	}
	if err := nonEmptyList("steps", f.Steps); err != nil {
		return Lesson{}, err
	}
	if len(f.Questions) < MinQuestions {
		return Lesson{}, fmt.Errorf("needs at least %d questions, has %d", MinQuestions, len(f.Questions))
	}

	lesson := Lesson{
		Slug: f.Slug, Title: f.Title, Objectives: f.Objectives, Steps: f.Steps,
		SourceTitle: f.SourceTitle, SourceURL: f.SourceURL, RelatedTask: f.RelatedTask, Disclaimer: f.Disclaimer,
	}
	for i, q := range f.Questions {
		n := i + 1
		if strings.TrimSpace(q.Prompt) == "" {
			return Lesson{}, fmt.Errorf("question %d: prompt must not be empty", n)
		}
		if strings.TrimSpace(q.Explanation) == "" {
			return Lesson{}, fmt.Errorf("question %d: explanation must not be empty", n)
		}
		if len(q.Options) != OptionsPerQuestion {
			return Lesson{}, fmt.Errorf("question %d: needs exactly %d options, has %d", n, OptionsPerQuestion, len(q.Options))
		}
		if err := nonEmptyList(fmt.Sprintf("question %d: options", n), q.Options); err != nil {
			return Lesson{}, err
		}
		if q.CorrectIndex == nil {
			return Lesson{}, fmt.Errorf("question %d: correct_index is missing", n)
		}
		if *q.CorrectIndex < 0 || *q.CorrectIndex >= len(q.Options) {
			return Lesson{}, fmt.Errorf("question %d: correct_index %d is out of range 0-%d", n, *q.CorrectIndex, len(q.Options)-1)
		}
		lesson.Questions = append(lesson.Questions, Question{
			Prompt: q.Prompt, Options: q.Options, CorrectIndex: *q.CorrectIndex, Explanation: q.Explanation,
		})
	}
	return lesson, nil
}

func nonEmptyList(field string, items []string) error {
	if len(items) == 0 {
		return fmt.Errorf("%s must not be empty", field)
	}
	for i, item := range items {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("%s: item %d must not be empty", field, i+1)
		}
	}
	return nil
}
