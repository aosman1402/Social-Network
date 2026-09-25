-- Records which member sent a group invitation, so they can be told when
-- the invitee accepts or declines (any member can invite, not just the
-- group creator). NULL for join requests and pre-existing rows.
ALTER TABLE group_members ADD COLUMN invited_by INTEGER;

-- Declined invitations/join requests used to be kept as 'declined' rows,
-- which permanently blocked that user from being re-invited or asking to
-- join again. Declines now delete the row; clear out any left behind.
DELETE FROM group_members WHERE status = 'declined';
