ALTER TABLE aura.stats
UPDATE request_type = CASE
    WHEN chain = 'solana' THEN 'RPC'
    WHEN chain = 'solana-das' THEN 'DAS'
    WHEN chain = 'getProgramAccounts' THEN 'GPA'
    WHEN chain = 'eclipse' THEN 'RPC'
    WHEN chain = 'eclipse-das' THEN 'DAS'
    ELSE 'RPC' -- Default value for any unexpected cases
END
where request_type  = '';

alter table aura.stats
update chain = case
    WHEN chain = 'solana-das' THEN 'solana'
    WHEN chain = 'getProgramAccounts' THEN 'solana'
    WHEN chain = 'eclipse-das' THEN 'eclipse'
    ELSE chain
end
where true;

alter table aura.stats
update method_cost = 13
where request_type='Websocket' and method_cost=0 and subscription_id=2;
