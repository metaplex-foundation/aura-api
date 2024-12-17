ALTER TABLE subscriptions DROP column IF EXISTS sbs_minimum_balance;
ALTER TABLE subscriptions DROP column IF EXISTS sbs_request_per_second;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_priority INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD column IF NOT EXISTS usr_last_updated_plan_at timestamp default now() not null;
INSERT INTO subscriptions (sbs_name, sbs_tokens_limit, sbs_priority) VALUES ('Developer', 5, 1), ('Advanced', 20, 2), ('Pro', 20, 3);
