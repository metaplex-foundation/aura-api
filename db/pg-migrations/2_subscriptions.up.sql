ALTER TABLE subscriptions DROP column IF EXISTS sbs_minimum_balance;
ALTER TABLE subscriptions DROP column IF EXISTS sbs_request_per_second;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_priority INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD column IF NOT EXISTS usr_last_updated_plan_at timestamp default now() not null;