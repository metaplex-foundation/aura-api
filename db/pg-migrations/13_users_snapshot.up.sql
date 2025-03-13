CREATE TABLE users_snapshot (
    urs_day date not null,
    urs_pro_subscriptions BIGINT not null,
    urs_advanced_subscriptions BIGINT not null,
    urs_pay_as_you_go BIGINT not null,
    urs_active_users BIGINT not null,
    urs_total_users BIGINT not null,
	CONSTRAINT users_snapshot_pk PRIMARY KEY (urs_day)
);
