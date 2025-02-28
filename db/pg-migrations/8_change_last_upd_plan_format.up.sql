ALTER TABLE users ALTER COLUMN usr_last_updated_plan_at TYPE date USING usr_last_updated_plan_at::date;
