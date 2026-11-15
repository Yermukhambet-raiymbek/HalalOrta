package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"littlevillage/content"
)

// lessonSyncLockID serializes lesson syncs when several instances start.
const lessonSyncLockID int64 = 7_271_000_002

// bestAttemptOrder picks a member's best attempt for a lesson: a pass
// first, then the highest score, then the earliest time it was reached.
const bestAttemptOrder = `a.passed DESC, a.score DESC, a.created_at ASC`

// LessonSummary is a lesson in the list with the member's own result.
// BestScore, BestTotal and AchievedAt are nil when the member has no attempt.
type LessonSummary struct {
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	RelatedTask   string     `json:"related_task"`
	CoverImage    string     `json:"cover_image"`
	QuestionCount int        `json:"question_count"`
	Status        string     `json:"status"` // not_started, in_progress or passed
	BestScore     *int       `json:"best_score"`
	BestTotal     *int       `json:"best_total"`
	AchievedAt    *time.Time `json:"achieved_at"`
}

// Lesson is a full lesson. Questions include the answer key, so handlers
// must not serialize this type directly.
type Lesson struct {
	ID          string
	Slug        string
	Title       string
	Objectives  []string
	Steps       []string
	SourceTitle string
	SourceURL   string
	RelatedTask string
	Disclaimer  string
	CoverImage  string
	StepImages  []string
	Questions   []content.Question
}

// LessonRef names a lesson.
type LessonRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// MemberProgress is a member's best result for one lesson.
type MemberProgress struct {
	MemberID   string    `json:"-"`
	LessonSlug string    `json:"lesson_slug"`
	Passed     bool      `json:"passed"`
	BestScore  int       `json:"best_score"`
	Total      int       `json:"total"`
	AchievedAt time.Time `json:"achieved_at"`
}

// SyncLessons makes the database match the lesson files: lessons are
// upserted by slug in the given order, their questions are replaced, and
// lessons whose slug is no longer present are deleted (with their attempts).
func (s *Store) SyncLessons(ctx context.Context, lessons []content.Lesson) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lessonSyncLockID); err != nil {
			return fmt.Errorf("lock lesson sync: %w", err)
		}
		slugs := make([]string, 0, len(lessons))
		for i, l := range lessons {
			slugs = append(slugs, l.Slug)
			stepImages := l.StepImages
			if stepImages == nil {
				stepImages = []string{} // the column is NOT NULL
			}
			var lessonID string
			err := tx.QueryRow(ctx, `
				INSERT INTO lessons (slug, position, title, objectives, steps, source_title, source_url, related_task,
				                     disclaimer, cover_image, step_images)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				ON CONFLICT (slug) DO UPDATE SET
					position = EXCLUDED.position, title = EXCLUDED.title, objectives = EXCLUDED.objectives,
					steps = EXCLUDED.steps, source_title = EXCLUDED.source_title, source_url = EXCLUDED.source_url,
					related_task = EXCLUDED.related_task, disclaimer = EXCLUDED.disclaimer,
					cover_image = EXCLUDED.cover_image, step_images = EXCLUDED.step_images, updated_at = now()
				RETURNING id`,
				l.Slug, i, l.Title, l.Objectives, l.Steps, l.SourceTitle, l.SourceURL, l.RelatedTask, l.Disclaimer,
				l.CoverImage, stepImages,
			).Scan(&lessonID)
			if err != nil {
				return fmt.Errorf("upsert lesson %s: %w", l.Slug, err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM questions WHERE lesson_id = $1`, lessonID); err != nil {
				return fmt.Errorf("clear questions of %s: %w", l.Slug, err)
			}
			for pos, q := range l.Questions {
				if _, err := tx.Exec(ctx, `
					INSERT INTO questions (lesson_id, position, prompt, options, correct_index, explanation)
					VALUES ($1, $2, $3, $4, $5, $6)`,
					lessonID, pos, q.Prompt, q.Options, q.CorrectIndex, q.Explanation,
				); err != nil {
					return fmt.Errorf("insert question %d of %s: %w", pos+1, l.Slug, err)
				}
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM lessons WHERE slug <> ALL($1)`, slugs); err != nil {
			return fmt.Errorf("delete removed lessons: %w", err)
		}
		return nil
	})
}

// ListLessons returns all lessons in order with memberID's best result.
func (s *Store) ListLessons(ctx context.Context, memberID string) ([]LessonSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.slug, l.title, l.related_task, l.cover_image,
		       (SELECT count(*) FROM questions q WHERE q.lesson_id = l.id),
		       b.score, b.total, b.passed, b.created_at
		FROM lessons l
		LEFT JOIN LATERAL (
			SELECT a.score, a.total, a.passed, a.created_at
			FROM quiz_attempts a
			WHERE a.member_id = $1 AND a.lesson_id = l.id
			ORDER BY `+bestAttemptOrder+`
			LIMIT 1
		) b ON true
		ORDER BY l.position`, memberID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (LessonSummary, error) {
		var ls LessonSummary
		var passed *bool
		if err := row.Scan(&ls.Slug, &ls.Title, &ls.RelatedTask, &ls.CoverImage, &ls.QuestionCount,
			&ls.BestScore, &ls.BestTotal, &passed, &ls.AchievedAt); err != nil {
			return ls, err
		}
		switch {
		case passed == nil:
			ls.Status = "not_started"
		case *passed:
			ls.Status = "passed"
		default:
			ls.Status = "in_progress"
		}
		return ls, nil
	})
}

// LessonRefs returns the slug and title of every lesson in order.
func (s *Store) LessonRefs(ctx context.Context) ([]LessonRef, error) {
	rows, err := s.pool.Query(ctx, `SELECT slug, title FROM lessons ORDER BY position`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[LessonRef])
}

// LessonBySlug returns a lesson with its questions and answer key, or
// ErrNotFound.
func (s *Store) LessonBySlug(ctx context.Context, slug string) (Lesson, error) {
	var l Lesson
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, title, objectives, steps, source_title, source_url, related_task, disclaimer,
		       cover_image, step_images
		FROM lessons WHERE slug = $1`, slug,
	).Scan(&l.ID, &l.Slug, &l.Title, &l.Objectives, &l.Steps, &l.SourceTitle, &l.SourceURL, &l.RelatedTask, &l.Disclaimer,
		&l.CoverImage, &l.StepImages)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT prompt, options, correct_index, explanation
		FROM questions WHERE lesson_id = $1 ORDER BY position`, l.ID)
	if err != nil {
		return Lesson{}, err
	}
	l.Questions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[content.Question])
	return l, err
}

// SaveAttempt records one graded quiz submission.
func (s *Store) SaveAttempt(ctx context.Context, memberID, lessonID string, score, total int, answers []int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO quiz_attempts (member_id, lesson_id, score, total, passed, answers)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		memberID, lessonID, score, total, score == total, answers)
	return err
}

// FamilyProgress returns the best result of every member of the family for
// every lesson they attempted. Callers must pass the family ID from the
// session.
func (s *Store) FamilyProgress(ctx context.Context, familyID string) ([]MemberProgress, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (a.member_id, a.lesson_id)
		       a.member_id, l.slug, a.passed, a.score, a.total, a.created_at
		FROM quiz_attempts a
		JOIN members m ON m.id = a.member_id
		JOIN lessons l ON l.id = a.lesson_id
		WHERE m.family_id = $1
		ORDER BY a.member_id, a.lesson_id, `+bestAttemptOrder, familyID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[MemberProgress])
}
