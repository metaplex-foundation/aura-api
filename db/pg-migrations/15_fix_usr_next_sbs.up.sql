# users with pay-as-you-go plan should also have pay-as-you-go as a next plan
UPDATE users SET usr_next_sbs_id = 2 WHERE sbs_id = 2;
