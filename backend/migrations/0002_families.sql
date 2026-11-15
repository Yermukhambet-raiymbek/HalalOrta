-- 0002_families: families, members, invite links and browser sessions.

CREATE TABLE families (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE members (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id    uuid        NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    relationship text        NOT NULL CHECK (relationship IN ('Mom', 'Dad', 'Grandma', 'Grandpa', 'Other')),
    is_owner     boolean     NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX members_family_id_idx ON members (family_id);
CREATE UNIQUE INDEX members_one_owner_per_family ON members (family_id) WHERE is_owner;

-- One active invite link per family (UNIQUE family_id).
CREATE TABLE invitations (
    token      text        PRIMARY KEY,
    family_id  uuid        NOT NULL UNIQUE REFERENCES families (id) ON DELETE CASCADE,
    created_by uuid        NOT NULL REFERENCES members (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A device is one browser. Only the SHA-256 of its cookie token is stored.
CREATE TABLE devices (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash       bytea       NOT NULL UNIQUE,
    active_member_id uuid,
    created_at       timestamptz NOT NULL DEFAULT now()
);

-- Members created on a device: the ones this browser may switch between.
CREATE TABLE device_members (
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    PRIMARY KEY (device_id, member_id)
);
CREATE INDEX device_members_member_id_idx ON device_members (member_id);

-- The active member must be one of the device's own members.
ALTER TABLE devices
    ADD CONSTRAINT devices_active_member_fk
    FOREIGN KEY (id, active_member_id) REFERENCES device_members (device_id, member_id);
