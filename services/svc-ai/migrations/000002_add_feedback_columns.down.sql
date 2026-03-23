ALTER TABLE analysis_tasks
  DROP COLUMN IF EXISTS feedback_rating,
  DROP COLUMN IF EXISTS feedback_helpful,
  DROP COLUMN IF EXISTS feedback_comment,
  DROP COLUMN IF EXISTS feedback_correct_root_cause,
  DROP COLUMN IF EXISTS feedback_at;
