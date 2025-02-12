drop table if exists aura.aggregated_usage_data;

drop table if exists aura.aggregated_providers_stats;

alter table aura.stats drop column subscription_id Int64;
