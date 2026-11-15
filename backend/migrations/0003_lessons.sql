-- 0003_lessons: lessons and questions (synced from the embedded content
-- files on startup) and quiz attempts.

CREATE TABLE lessons (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         text        NOT NULL UNIQUE,
    position     integer     NOT NULL,
    title        text        NOT NULL,
    objectives   text[]      NOT NULL,
    steps        text[]      NOT NULL,
    source_title text        NOT NULL,
    source_url   text        NOT NULL,
    related_task text        NOT NULL,
    disclaimer   text        NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE questions (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id     uuid    NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    position      integer NOT NULL,
    prompt        text    NOT NULL,
    options       text[]  NOT NULL,
    correct_index integer NOT NULL,
    explanation   text    NOT NULL,
    UNIQUE (lesson_id, position)
);

-- One row per quiz submission. A lesson is passed when a member has an
-- attempt with every answer correct. Retakes are unlimited.
CREATE TABLE quiz_attempts (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id  uuid        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    lesson_id  uuid        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    score      integer     NOT NULL,
    total      integer     NOT NULL,
    passed     boolean     NOT NULL,
    answers    integer[]   NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (score BETWEEN 0 AND total),
    CHECK (passed = (score = total))
);
CREATE INDEX quiz_attempts_member_lesson_idx ON quiz_attempts (member_id, lesson_id);
