-- Remove the demo data from 000022_seed_demo_data.up.sql. Every demo row
-- hangs off a demo user (posts, comments, likes, follows, groups and
-- everything in them, events, messages, notifications all use
-- ON DELETE CASCADE), so deleting the five users removes it all.
DELETE FROM users WHERE email IN (
    'alice@demo.test', 'bob@demo.test', 'carol@demo.test', 'dave@demo.test', 'eve@demo.test'
);
