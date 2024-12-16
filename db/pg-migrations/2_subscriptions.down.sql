ALTER TABLE subscriptions DROP column IF EXISTS sbs_priority;
ALTER TABLE subscriptions DROP column IF EXISTS sbs_included_websocket;
ALTER TABLE subscriptions ADD column IF NOT EXISTS sbs_minimum_balance integer not null;
