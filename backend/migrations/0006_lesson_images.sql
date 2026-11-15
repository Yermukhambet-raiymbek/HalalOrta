-- 0006_lesson_images: a cover picture per lesson and one picture per step.
-- Both hold file names served by the frontend from /illustrations/.

ALTER TABLE lessons
    ADD COLUMN cover_image text   NOT NULL DEFAULT '',
    ADD COLUMN step_images text[] NOT NULL DEFAULT '{}';
