DROP TRIGGER IF EXISTS trg_check_and_update_subscription ON users;
DROP FUNCTION IF EXISTS check_and_update_subscription;

ALTER TABLE subscriptions DROP COLUMN IF EXISTS sbs_price_mplx;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS sbs_period_days;

ALTER TABLE users DROP COLUMN IF EXISTS usr_last_updated_plan_at;
ALTER TABLE users DROP COLUMN IF EXISTS usr_sbs_ends_on;
