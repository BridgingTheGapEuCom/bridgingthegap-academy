-- +goose Up
CREATE TABLE courses.course_version_assessment_binding (
    course_version_id uuid NOT NULL REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    assessment_key uuid NOT NULL,
    definition jsonb NOT NULL,
    PRIMARY KEY (course_version_id, assessment_key),
    CONSTRAINT course_version_assessment_definition CHECK (
        jsonb_typeof(definition) = 'object'
        AND jsonb_typeof(definition->'questions') = 'array'
        AND jsonb_array_length(definition->'questions') > 0
    )
);

-- +goose Down
DROP TABLE courses.course_version_assessment_binding;
