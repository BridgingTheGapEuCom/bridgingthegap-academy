-- +goose Up
CREATE SCHEMA community;
CREATE TABLE community.course_community (
 course_id uuid PRIMARY KEY REFERENCES courses.course(id) ON DELETE RESTRICT,
 mode text NOT NULL DEFAULT 'ENABLED' CHECK (mode IN ('ENABLED','DISABLED')),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE community.thread (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), course_id uuid NOT NULL REFERENCES community.course_community(course_id) ON DELETE RESTRICT,
 title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 240), created_by_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 state text NOT NULL DEFAULT 'VISIBLE' CHECK (state IN ('VISIBLE','HIDDEN')), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE community.post (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), thread_id uuid NOT NULL REFERENCES community.thread(id) ON DELETE RESTRICT,
 author_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT, body text NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 20000),
 state text NOT NULL DEFAULT 'VISIBLE' CHECK (state IN ('VISIBLE','HIDDEN')), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX community_thread_course_order_idx ON community.thread(course_id,updated_at DESC,id DESC);
CREATE INDEX community_post_thread_order_idx ON community.post(thread_id,created_at ASC,id ASC);
-- +goose Down
DROP SCHEMA community CASCADE;
