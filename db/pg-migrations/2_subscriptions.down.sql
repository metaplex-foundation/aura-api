ALTER TABLE users DROP column IF EXISTS usr_last_updated_plan_at;
DELETE FROM subscriptions WHERE sbs_name IN ('Developer', 'Advanced', 'Pro');
ALTER TABLE subscriptions DROP column IF EXISTS sbs_priority;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_minimum_balance integer not null;
ALTER TABLE subscriptions  ADD column IF NOT EXISTS sbs_request_per_second integer not null;
