-- Lets a user record the educational body/provider they study or trained
-- with, the qualification they're pursuing, and its Regulated Qualifications
-- Framework (RQF) level - shown and edited from the Settings "Education"
-- tab. Free-text columns rather than lookup tables: the option lists are
-- small, fixed, and enforced client-side/server-side against a hardcoded
-- allowlist (see educationProviderOptions/qualificationNameOptions/
-- rqfLevelOptions in user.go), the same pattern already used for avatar
-- presets.
ALTER TABLE users ADD COLUMN education_provider TEXT;
ALTER TABLE users ADD COLUMN qualification_name TEXT;
ALTER TABLE users ADD COLUMN rqf_level TEXT;
