ALTER TABLE users DROP column IF EXISTS usr_last_updated_plan_at;
ALTER TABLE users ADD column IF NOT EXISTS usr_last_updated_plan_at TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS usr_sbs_ends_on TIMESTAMP;

ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS sbs_price_mplx integer not null default 0;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS sbs_period_days integer not null default 0;

UPDATE subscriptions SET sbs_price_mplx = 500000000 WHERE sbs_priority = 2;
UPDATE subscriptions SET sbs_price_mplx = 1500000000 WHERE sbs_priority = 3;
UPDATE subscriptions SET sbs_period_days = 30 WHERE sbs_priority = 2;
UPDATE subscriptions SET sbs_period_days = 30 WHERE sbs_priority = 3;

CREATE OR REPLACE FUNCTION check_and_update_subscription()
    RETURNS TRIGGER AS
$$
DECLARE
    new_subscription_price INTEGER;
    new_subscription_period INTEGER;
    new_subscription_priority INTEGER;
    old_subscription_priority INTEGER;
BEGIN
    -- Check if the sbs_id field is being updated
    IF NEW.sbs_id IS DISTINCT FROM OLD.sbs_id THEN
        -- Retrieve data for the new subscription
        SELECT sbs_price_mplx, sbs_period_days, sbs_priority
        INTO new_subscription_price, new_subscription_period, new_subscription_priority
        FROM subscriptions
        WHERE sbs_id = NEW.sbs_id;
        -- Retrieve the priority of the current subscription
        SELECT sbs_priority
        INTO old_subscription_priority
        FROM subscriptions
        WHERE sbs_id = OLD.sbs_id;

        -- Check if the user has sufficient balance
        IF OLD.usr_mplx_balance <= 0 OR OLD.usr_mplx_balance < new_subscription_price THEN
            RAISE EXCEPTION 'Insufficient balance to change subscription.';
        END IF;

        -- Condition 1: Allow switching to a subscription with a higher priority
        IF new_subscription_priority > old_subscription_priority THEN
            -- Deduct the subscription price from the user's balance and update the expiration date
        -- Condition 2: Allow switching if usr_sbs_ends_on IS NULL
        ELSIF OLD.usr_sbs_ends_on IS NULL THEN
            -- No additional checks or updates needed
        -- Condition 3: Allow switching to a subscription with priority 0 (Free subscription)
        ELSIF new_subscription_priority = 0 AND (OLD.usr_sbs_ends_on IS NULL OR OLD.usr_sbs_ends_on < NOW()) THEN
            -- Clear the expiration date
            NEW.usr_sbs_ends_on := NULL;

        -- Otherwise, block the subscription change
        ELSE
            RAISE EXCEPTION 'Cannot switch to the selected subscription.';
        END IF;

        NEW.usr_mplx_balance := OLD.usr_mplx_balance - new_subscription_price;
        IF new_subscription_period > 0 THEN
            NEW.usr_sbs_ends_on := NOW() + (new_subscription_period || ' days')::INTERVAL;
        ELSE
            NEW.usr_sbs_ends_on := NULL; -- Clear the expiration date if the period is 0 (Pay as you Go subscription type)
        END IF;
        -- Update the last updated plan timestamp
        NEW.usr_last_updated_plan_at := NOW();
    END IF;

    RETURN NEW;
END;
$$
    LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_check_and_update_subscription
    BEFORE UPDATE OF sbs_id ON users
    FOR EACH ROW
    EXECUTE FUNCTION check_and_update_subscription();