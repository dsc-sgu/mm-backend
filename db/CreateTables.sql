CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE user_role AS ENUM ('ADMIN', 'USER');
CREATE TYPE course_member_role AS ENUM ('STUDENT', 'TEACHER');
CREATE TYPE attempt_state AS ENUM ('submitted', 'graded');

CREATE TABLE unit_types (
    id uuid PRIMARY KEY,
    name varchar(64)
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at TIMESTAMPTZ NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    patronymic TEXT,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    role user_role NOT NULL,
    avatar_url TEXT,
    password_hash BYTEA NOT NULL,
    password_salt BYTEA NOT NULL
);

CREATE TABLE units (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varchar(128) NOT NULL,
    unit_type uuid REFERENCES unit_types(id),
    parent_id uuid REFERENCES units(id),
    owner_id uuid REFERENCES users(id)
);

-- NOTE(mchernigin): for example "Programming languages"
CREATE TABLE disciplines (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varchar(64) NOT NULL
);

-- NOTE(mchernigin): for example "Programming languages (2024)"
CREATE TABLE courses (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    discipline_id uuid,
    owner_id uuid NOT NULL REFERENCES users(id),
    name varchar(128) NOT NULL,
    display_name varchar(128) NOT NULL,
    -- Service info
    created_at timestamp NOT NULL,
    deleted_at timestamp,

    -- NOTE(Ezhkin-Kot): unique course names in one discipline
    CONSTRAINT course_discipline_name_unique
        UNIQUE (discipline_id, name)
);

-- NOTE(nrydanov): a block is a stack of versions under one id. The course a
-- student sees is the latest published (draft = false) version of every
-- block that is not deleted. A published version is never changed; an edit
-- creates a draft version owned by its author, and publishing turns the
-- author's drafts into published versions, merging them with whatever was
-- published since base_version. A deleted block keeps its versions and gets
-- one more with deleted = true.
CREATE TABLE blocks (
    id uuid NOT NULL DEFAULT uuidv7(),
    version integer NOT NULL CHECK (version > 0),
    course_id uuid NOT NULL REFERENCES courses(id),
    -- NOTE(nrydanov): the frontend names block types; "task" is the only one
    -- the backend gives a meaning to
    block_type varchar(64) NOT NULL CHECK (block_type ~ '^[a-z0-9-]+$'),
    -- content the backend does not read, NULL for a task block
    data jsonb,
    -- NOTE(Ezhkin-Kot): fractional indexing
    position varchar(64) NOT NULL COLLATE "C",
    draft boolean NOT NULL,
    deleted boolean NOT NULL DEFAULT false,
    created_by uuid NOT NULL REFERENCES users(id),
    -- the published version a draft was copied from, NULL for a new block;
    -- publishing compares base, head and the draft field by field
    base_version integer,

    PRIMARY KEY (id, version)
);

CREATE INDEX idx_blocks_published ON blocks(course_id, id, version DESC) WHERE (NOT draft);
CREATE INDEX idx_blocks_drafts ON blocks(course_id, created_by) WHERE (draft);

-- NOTE(nrydanov): a checkpoint records which block versions a publish made
-- visible, as the full map of the course's live blocks at that moment. It is
-- history only: the current state is read from blocks, and a checkpoint is
-- used to list publishes and to restore one of them.
CREATE TABLE checkpoints (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    course_id uuid NOT NULL REFERENCES courses(id),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamp NOT NULL
);

CREATE TABLE checkpoint_blocks (
    checkpoint_id uuid NOT NULL REFERENCES checkpoints(id),
    block_id uuid NOT NULL,
    version integer NOT NULL,

    PRIMARY KEY (checkpoint_id, block_id),
    FOREIGN KEY (block_id, version) REFERENCES blocks(id, version)
);

-- NOTE(Ezhkin-Kot): data for invite links to courses
CREATE TABLE invites (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    course_id uuid NOT NULL REFERENCES courses(id),
    -- NOTE(Ezhkin-Kot): which role will be given to invited users
    provided_role course_member_role NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamp NOT NULL,
    expires_at timestamp,
    is_revoked boolean NOT NULL
);

-- NOTE(Ezhkin-Kot): roles of users in courses
-- Their specific data is stored in separate tables for each role
CREATE TABLE course_members (
  user_id uuid NOT NULL REFERENCES users(id),
  course_id uuid NOT NULL REFERENCES courses(id),
  role course_member_role NOT NULL,
  invited_by uuid REFERENCES invites(id),
  is_active boolean NOT NULL,

  PRIMARY KEY (user_id, course_id)
);

-- NOTE(nrydanov): Specific student data
CREATE TABLE students (
    user_id uuid NOT NULL,
    course_id uuid NOT NULL,
    -- NOTE(mchernigin): leaf unit
    admission_date date NOT NULL,
    is_active boolean NOT NULL,

    PRIMARY KEY (user_id, course_id),
    FOREIGN KEY (user_id, course_id) REFERENCES course_members(user_id, course_id)
);

-- NOTE(nrydanov): Specific teacher data
CREATE TABLE teachers (
    user_id uuid NOT NULL,
    course_id uuid NOT NULL,
    promoted_by uuid NOT NULL REFERENCES users(id),
    promoted_at timestamp NOT NULL,
    is_active boolean NOT NULL,
    -- TODO(nrydanov): Add additional required teacher data if required

    PRIMARY KEY (user_id, course_id),
    FOREIGN KEY (user_id, course_id) REFERENCES course_members(user_id, course_id)
);

-- NOTE(nrydanov): Specific admin data
CREATE TABLE admins (
    user_id uuid NOT NULL REFERENCES users(id),
    promoted_by uuid NOT NULL REFERENCES users(id),
    promoted_at timestamp NOT NULL
);

-- NOTE(mchernigin): probably should give names to milestones
-- NOTE(nrydanov): This relation should be used to determine in what milestone
-- certain discipline become available for a student from some unit
CREATE TABLE discipline_milestones (
    discipline_id uuid NOT NULL REFERENCES disciplines(id),
    -- NOTE(nrydanov): Leaf unit, most of the time
    unit_id uuid NOT NULL REFERENCES units(id),
    milestone integer NOT NULL,

    PRIMARY KEY (discipline_id, unit_id)
);

-- NOTE(nrydanov): This relation should be used to add transition day/month
-- for certain milestone for a student from some unit
CREATE TABLE milestone_transitions (
    milestone integer NOT NULL,
    -- NOTE(nrydanov): Leaf unit, most of the time
    unit_id uuid NOT NULL REFERENCES units(id),
    transition_date date,

    PRIMARY KEY (milestone, unit_id)
);

CREATE TABLE groups (
    id uuid NOT NULL PRIMARY KEY DEFAULT uuidv7(),
    name varchar(64) NOT NULL
);

CREATE TABLE course_users_groups (
    user_id uuid NOT NULL REFERENCES users(id),
    course_id uuid NOT NULL REFERENCES courses(id),
    group_id uuid NOT NULL REFERENCES groups(id),

    PRIMARY KEY (user_id, course_id, group_id)
);

-- NOTE(nrydanov): ephemeral entity (not a block) — groups tasks into one shared
-- git repo; course_id is here so the SSH layer resolves <course>/<group name>
CREATE TABLE task_groups (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    course_id uuid NOT NULL REFERENCES courses(id),
    name varchar(128) NOT NULL,
    deleted_at timestamp
);

-- NOTE(Ezhkin-Kot): task_groups is soft-deleted rather than hard-deleted,
-- since tasks linked from old block versions may still reference a group; a
-- partial index keeps names unique only among the still-live groups, so a
-- deleted name can be reused.
CREATE UNIQUE INDEX idx_task_groups_course_name ON task_groups(course_id, name) WHERE (deleted_at IS NULL);

-- NOTE(nrydanov): a task lives outside block versions, so its id stays the
-- same while its block is edited and attempts keep pointing at it. A course
-- reaches its tasks through task_blocks. A task name is unique among the
-- tasks of its group in one course; the database cannot express that, so
-- the block service checks it.
CREATE TABLE tasks (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    task_group_id uuid NOT NULL REFERENCES task_groups(id),
    name varchar(128) NOT NULL,
    statement jsonb,
    -- glob patterns matching the files that count as this task's solution
    patterns text[] NOT NULL DEFAULT '{}',
    max_grade real NOT NULL,
    -- NULL means attempts are not limited
    max_attempts integer CHECK (max_attempts > 0),
    available_at timestamptz,
    deadline_at timestamptz,
    deleted_at timestamp
);

-- NOTE(nrydanov): links a task-type block to its task. The link is per block
-- id, shared by all the block's versions, so it cannot reference blocks(id,
-- version).
CREATE TABLE task_blocks (
    block_id uuid PRIMARY KEY,
    task_id uuid NOT NULL REFERENCES tasks(id)
);

CREATE TABLE attempts (
    id uuid NOT NULL PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users(id),
    task_id uuid NOT NULL REFERENCES tasks(id)
);

CREATE TABLE attempt_transitions (
    attempt_id uuid NOT NULL REFERENCES attempts(id),
    state attempt_state NOT NULL,
    transition_at timestamp NOT NULL,
    -- Can be used to save additional transition data (unpredictable for now)
    transition_data jsonb
);

CREATE TABLE ssh_keys (
    fingerprint TEXT PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    key TEXT NOT NULL,
    created_at timestamp NOT NULL
);
