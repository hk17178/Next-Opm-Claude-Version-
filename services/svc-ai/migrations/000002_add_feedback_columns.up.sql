ALTER TABLE analysis_tasks
  ADD COLUMN IF NOT EXISTS feedback_rating             INTEGER,
  ADD COLUMN IF NOT EXISTS feedback_helpful             BOOLEAN,
  ADD COLUMN IF NOT EXISTS feedback_comment             TEXT,
  ADD COLUMN IF NOT EXISTS feedback_correct_root_cause  TEXT,
  ADD COLUMN IF NOT EXISTS feedback_at                  TIMESTAMPTZ;
