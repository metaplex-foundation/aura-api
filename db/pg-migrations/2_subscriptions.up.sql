ALTER TABLE subscriptions DROP column IF EXISTS sbs_minimum_balance;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_priority INT NOT NULL DEFAULT 0;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_included_websocket BOOL NOT NULL DEFAULT false;
