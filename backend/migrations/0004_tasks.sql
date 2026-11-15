-- 0004_tasks: care tasks that family members assign to each other.

CREATE TABLE tasks (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id    uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    title        text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 80),
    -- The lesson that prepares for this task, if any. Whether the assignee
    -- has passed it is never stored: it is read from quiz_attempts.
    lesson_id    uuid        REFERENCES lessons (id) ON DELETE SET NULL,
    assignee_id  uuid        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    -- A calendar day in the family's own time zone, sent by the browser.
    due_date     date        NOT NULL,
    status       text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
    completed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'done') = (completed_at IS NOT NULL))
);
CREATE INDEX tasks_family_id_idx ON tasks (family_id);
CREATE INDEX tasks_assignee_id_idx ON tasks (assignee_id);
