-- +goose Up
ALTER TABLE assessments.assessment_attempt
    ADD COLUMN correct_count integer,
    ADD COLUMN total_count integer,
    ADD CONSTRAINT assessment_attempt_result_state CHECK (
        (state = 'IN_PROGRESS' AND correct_count IS NULL AND total_count IS NULL)
        OR
        (state = 'SUBMITTED' AND correct_count IS NOT NULL AND total_count IS NOT NULL
            AND correct_count >= 0 AND total_count > 0 AND correct_count <= total_count)
    );

-- +goose Down
ALTER TABLE assessments.assessment_attempt
    DROP CONSTRAINT assessment_attempt_result_state,
    DROP COLUMN total_count,
    DROP COLUMN correct_count;
