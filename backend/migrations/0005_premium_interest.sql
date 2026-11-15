-- 0005_premium_interest: members who said they would use the paid plan.
-- Payments are not live; this records demand only.

CREATE TABLE premium_interest (
    member_id  uuid        PRIMARY KEY REFERENCES members (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
