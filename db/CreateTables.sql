CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE user_role AS ENUM ('ADMIN', 'USER');
CREATE TYPE attempt_state AS ENUM ('submitted', 'graded');
-- NOTE(nrydanov): Need to think of other types together
CREATE TYPE block_type AS ENUM ('task', 'text');

CREATE TABLE unit_type (
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

CREATE TABLE unit (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varchar(128) NOT NULL,
    unit_type uuid REFERENCES unit_type(id),
    parent_id uuid REFERENCES unit(id),
    owner_id uuid REFERENCES users(id)
);

CREATE TABLE block (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    block_type TEXT NOT NULL,
    data jsonb NOT NULL,
    course_id uuid, -- REFERENCES course(id)
    position integer NOT NULL
);

-- NOTE(mchernigin): for example "Programming languages"
CREATE TABLE discipline (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varchar(64) NOT NULL
);


-- NOTE(mchernigin): for example "Programming languages (2024)"
CREATE TABLE course (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    discipline_id uuid NOT NULL REFERENCES discipline(id),
    owner_id uuid NOT NULL REFERENCES users(id),
    name varchar(128) NOT NULL,
    -- Service info
    created_at timestamp NOT NULL,

    -- NOTE(Ezhkin-Kot): unique course names in one discipline
    CONSTRAINT course_discipline_name_unique
        UNIQUE (discipline_id, name)
);


-- NOTE(nrydanov): Specific student data
CREATE TABLE student (
    user_id uuid NOT NULL REFERENCES users(id),
    course_id uuid NOT NULL REFERENCES course(id),
    -- NOTE(mchernigin): leaf unit
    unit_id uuid NOT NULL REFERENCES unit(id),
    admission_date date NOT NULL,
    expelled boolean NOT NULL,

    PRIMARY KEY (user_id, course_id)
);

-- NOTE(nrydanov): Specific teacher data
CREATE TABLE teacher (
    user_id uuid NOT NULL REFERENCES users(id),
    course_id uuid NOT NULL REFERENCES course(id),
    promoted_by uuid NOT NULL REFERENCES users(id),
    promoted_at timestamp NOT NULL,
    -- TODO(nrydanov): Add additional required teacher data if required

    PRIMARY KEY (user_id, course_id)
);

-- NOTE(nrydanov): Specific admin data
CREATE table admin (
    user_id uuid NOT NULL REFERENCES users(id),
    promoted_by uuid NOT NULL REFERENCES users(id),
    promoted_at timestamp NOT NULL
);


-- NOTE(mchernigin): probably should give names to milestones
-- NOTE(nrydanov): This relation should be used to determine in what milestone
-- certain discipline become available for a student from some unit
CREATE TABLE discipline_milestones (
    discipline_id uuid NOT NULL REFERENCES discipline(id),
    -- NOTE(nrydanov): Leaf unit, most of the time
    unit_id uuid NOT NULL REFERENCES unit(id),
    milestone integer NOT NULL,

    PRIMARY KEY (discipline_id, unit_id)
);

-- NOTE(nrydanov): This relation should be used to add transition day/month
-- for certain milestone for a student from some unit
CREATE TABLE milestone_transitions (
    milestone integer NOT NULL,
    -- NOTE(nrydanov): Leaf unit, most of the time
    unit_id uuid NOT NULL REFERENCES unit(id),
    transition_date date,

    PRIMARY KEY (milestone, unit_id)
);


CREATE TABLE groups (
    id uuid NOT NULL PRIMARY KEY DEFAULT uuidv7(),
    name varchar(64) NOT NULL
);


CREATE TABLE course_users_groups (
    user_id uuid NOT NULL REFERENCES users(id),
    course_id uuid NOT NULL REFERENCES course(id),
    group_id uuid NOT NULL REFERENCES groups(id),

    PRIMARY KEY (user_id, course_id, group_id)
);



CREATE TABLE task (
    block_id uuid PRIMARY KEY REFERENCES block(id),
    available_at timestamp,
    deadline_at timestamp,
    max_grade real NOT NULL,
    max_attempts integer NOT NULL,
    lead_time time
);


CREATE TABLE attempt (
    id uuid NOT NULL PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users(id),
    task_id uuid NOT NULL REFERENCES task(block_id)
);

CREATE TABLE attempt_transitions (
    attempt_id uuid NOT NULL REFERENCES attempt(id),
    state attempt_state NOT NULL,
    transition_at timestamp NOT NULL,
    -- Can be used to save additional transition data (unpredictable for now)
    transition_data jsonb
);

CREATE TABLE ssh_keys (
    owner_id uuid NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    key TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    created_at timestamp NOT NULL,

    PRIMARY KEY (owner_id, fingerprint)
);
