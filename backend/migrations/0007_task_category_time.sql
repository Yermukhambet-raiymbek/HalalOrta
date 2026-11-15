-- 0007_task_category_time: a category and an optional time of day for tasks,
-- so the family's day can be shown as a color-coded schedule.

ALTER TABLE tasks
    ADD COLUMN category text NOT NULL DEFAULT 'care'
        CHECK (category IN ('care', 'feeding', 'health', 'appointment', 'shopping')),
    -- Local time of day in the family's own time zone, sent by the browser.
    ADD COLUMN due_time time;
