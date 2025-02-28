ALTER TABLE users ALTER COLUMN usr_last_updated_plan_at TYPE timestamp USING usr_last_updated_plan_at::timestamp;
