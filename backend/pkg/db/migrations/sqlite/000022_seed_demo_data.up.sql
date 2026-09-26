-- Demo data, so a fresh install has something to look at and every feature
-- can be tried straight away. All five demo accounts use the password
--     Demo-Pass-2026
-- and an @demo.test email (a reserved domain, so it can never clash with a
-- real address):
--   alice@demo.test  public   - creator of "Weekend Hikers"
--   bob@demo.test    public   - Alice's friend, hiker
--   carol@demo.test  PRIVATE  - has a pending follow request (from Eve) and
--                               a pending invitation to "Weekend Hikers"
--   dave@demo.test   public   - hiker
--   eve@demo.test    public   - has asked to join "Weekend Hikers"
--
-- Safe on a database that already has real users: nothing uses fixed ids
-- (everything is looked up by the demo emails), a nickname that's already
-- taken is simply left empty, and every row that links two things is only
-- inserted when both exist. The down migration removes all of it.

-- Users -------------------------------------------------------------------
INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at, updated_at)
SELECT 'alice@demo.test', '$2a$10$YyymeiLZLua47e72GKx/Y.IepWlqYujMg0UmZh6QjWy.4.WjifBKC', 'Alice', 'Walker', '1996-03-14',
       'https://api.dicebear.com/10.x/clay/svg?seed=Nova&backgroundColor=ff2e63',
       CASE WHEN EXISTS (SELECT 1 FROM users WHERE nickname = 'AliceW') THEN NULL ELSE 'AliceW' END,
       'Weekend hiker and coffee enthusiast. I run the Weekend Hikers group.', 1,
       datetime('now', '-30 days'), datetime('now', '-30 days');

INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at, updated_at)
SELECT 'bob@demo.test', '$2a$10$YyymeiLZLua47e72GKx/Y.IepWlqYujMg0UmZh6QjWy.4.WjifBKC', 'Bob', 'Martin', '1994-11-02',
       'https://api.dicebear.com/10.x/clay/svg?seed=Comet&backgroundColor=00c2a8',
       CASE WHEN EXISTS (SELECT 1 FROM users WHERE nickname = 'BobM') THEN NULL ELSE 'BobM' END,
       'Photographer. Always carrying one camera too many.', 1,
       datetime('now', '-29 days'), datetime('now', '-29 days');

INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at, updated_at)
SELECT 'carol@demo.test', '$2a$10$YyymeiLZLua47e72GKx/Y.IepWlqYujMg0UmZh6QjWy.4.WjifBKC', 'Carol', 'Nguyen', '1998-07-21',
       'https://api.dicebear.com/10.x/clay/svg?seed=Lumen&backgroundColor=ffb300',
       CASE WHEN EXISTS (SELECT 1 FROM users WHERE nickname = 'CarolN') THEN NULL ELSE 'CarolN' END,
       'Private profile - follow me to see my posts. Book lover.', 0,
       datetime('now', '-28 days'), datetime('now', '-28 days');

INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at, updated_at)
SELECT 'dave@demo.test', '$2a$10$YyymeiLZLua47e72GKx/Y.IepWlqYujMg0UmZh6QjWy.4.WjifBKC', 'Dave', 'Okafor', '1992-01-30',
       'https://api.dicebear.com/10.x/clay/svg?seed=Aster&backgroundColor=3d5afe',
       CASE WHEN EXISTS (SELECT 1 FROM users WHERE nickname = 'DaveO') THEN NULL ELSE 'DaveO' END,
       'Trail runner. Will walk if there is cake at the top.', 1,
       datetime('now', '-27 days'), datetime('now', '-27 days');

INSERT OR IGNORE INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at, updated_at)
SELECT 'eve@demo.test', '$2a$10$YyymeiLZLua47e72GKx/Y.IepWlqYujMg0UmZh6QjWy.4.WjifBKC', 'Eve', 'Rossi', '2000-05-09',
       'https://api.dicebear.com/10.x/clay/svg?seed=Orbit&backgroundColor=8e24aa',
       CASE WHEN EXISTS (SELECT 1 FROM users WHERE nickname = 'EveR') THEN NULL ELSE 'EveR' END,
       'New in town, looking for people to explore with.', 1,
       datetime('now', '-26 days'), datetime('now', '-26 days');

-- Follows -----------------------------------------------------------------
-- Alice <-> Bob and Alice <-> Dave follow each other, Bob follows Carol
-- (accepted - Carol is private), Eve's request to Carol is still pending.
INSERT OR IGNORE INTO follows (follower_id, following_id, status, created_at, updated_at)
SELECT f.id, t.id, x.status, datetime('now', '-20 days'), datetime('now', '-20 days')
FROM (SELECT 'alice@demo.test' AS a, 'bob@demo.test'   AS b, 'accepted' AS status
      UNION ALL SELECT 'bob@demo.test',   'alice@demo.test', 'accepted'
      UNION ALL SELECT 'alice@demo.test', 'dave@demo.test',  'accepted'
      UNION ALL SELECT 'dave@demo.test',  'alice@demo.test', 'accepted'
      UNION ALL SELECT 'bob@demo.test',   'carol@demo.test', 'accepted'
      UNION ALL SELECT 'eve@demo.test',   'alice@demo.test', 'accepted'
      UNION ALL SELECT 'eve@demo.test',   'carol@demo.test', 'pending') x
JOIN users f ON f.email = x.a
JOIN users t ON t.email = x.b;

-- Posts (one of each privacy level) --------------------------------------
INSERT INTO posts (user_id, content, privacy_level, created_at, updated_at)
SELECT id, 'Hello everyone! 👋 Just joined - looking forward to meeting people who love the outdoors.', 'public',
       datetime('now', '-6 days'), datetime('now', '-6 days')
FROM users WHERE email = 'alice@demo.test';

INSERT INTO posts (user_id, content, privacy_level, created_at, updated_at)
SELECT id, 'Sunrise from the ridge this morning 🌄 Worth the 5am alarm.', 'public',
       datetime('now', '-5 days'), datetime('now', '-5 days')
FROM users WHERE email = 'bob@demo.test';

INSERT INTO posts (user_id, content, privacy_level, created_at, updated_at)
SELECT id, 'Followers only: planning a surprise for Bob''s birthday - ideas welcome! 🎂', 'almost_private',
       datetime('now', '-4 days'), datetime('now', '-4 days')
FROM users WHERE email = 'alice@demo.test';

INSERT INTO posts (user_id, content, privacy_level, created_at, updated_at)
SELECT id, 'Just for you Bob: the spare tent is in my car, grab it whenever.', 'private',
       datetime('now', '-3 days'), datetime('now', '-3 days')
FROM users WHERE email = 'alice@demo.test';

INSERT OR IGNORE INTO post_allowed_users (post_id, user_id)
SELECT p.id, b.id
FROM posts p
JOIN users a ON a.id = p.user_id AND a.email = 'alice@demo.test'
JOIN users b ON b.email = 'bob@demo.test'
WHERE p.content = 'Just for you Bob: the spare tent is in my car, grab it whenever.';

INSERT INTO posts (user_id, content, privacy_level, created_at, updated_at)
SELECT id, 'Finished my 12th book of the year 📚 Only my followers get the reviews.', 'almost_private',
       datetime('now', '-2 days'), datetime('now', '-2 days')
FROM users WHERE email = 'carol@demo.test';

-- Comments and likes ------------------------------------------------------
INSERT INTO comments (post_id, user_id, content, created_at, updated_at)
SELECT p.id, u.id, x.content, datetime('now', x.ago), datetime('now', x.ago)
FROM (SELECT 'bob@demo.test' AS who, 'Welcome Alice! 🎉' AS content,
             'Hello everyone! 👋 Just joined - looking forward to meeting people who love the outdoors.' AS post, '-143 hours' AS ago
      UNION ALL SELECT 'dave@demo.test', 'Welcome! Come to the next hike.',
             'Hello everyone! 👋 Just joined - looking forward to meeting people who love the outdoors.', '-142 hours'
      UNION ALL SELECT 'alice@demo.test', 'Stunning shot 😍',
             'Sunrise from the ridge this morning 🌄 Worth the 5am alarm.', '-118 hours') x
JOIN users u ON u.email = x.who
JOIN posts p ON p.content = x.post
JOIN users author ON author.id = p.user_id AND author.email LIKE '%@demo.test';

INSERT OR IGNORE INTO post_likes (post_id, user_id)
SELECT p.id, u.id
FROM posts p
JOIN users author ON author.id = p.user_id AND author.email LIKE '%@demo.test'
JOIN users u ON u.email IN ('alice@demo.test', 'bob@demo.test', 'dave@demo.test', 'eve@demo.test')
WHERE p.privacy_level = 'public' AND p.group_id IS NULL AND u.id <> p.user_id;

-- Groups ------------------------------------------------------------------
INSERT INTO groups (creator_id, title, description, created_at, updated_at)
SELECT id, 'Weekend Hikers', 'Easy-to-moderate hikes every weekend. All paces welcome! 🥾',
       datetime('now', '-15 days'), datetime('now', '-15 days')
FROM users WHERE email = 'alice@demo.test';

INSERT INTO groups (creator_id, title, description, created_at, updated_at)
SELECT id, 'Book Club', 'One book a month, discussed over coffee.',
       datetime('now', '-12 days'), datetime('now', '-12 days')
FROM users WHERE email = 'carol@demo.test';

-- Weekend Hikers: Alice (creator), Bob and Dave are members, Carol has a
-- pending invitation from Alice, Eve has asked to join.
INSERT OR IGNORE INTO group_members (group_id, user_id, status, invited_by, created_at, updated_at)
SELECT g.id, u.id, x.status,
       CASE WHEN x.status = 'invited' THEN g.creator_id END,
       datetime('now', x.ago), datetime('now', x.ago)
FROM (SELECT 'alice@demo.test' AS who, 'active' AS status, '-15 days' AS ago
      UNION ALL SELECT 'bob@demo.test',   'active',  '-14 days'
      UNION ALL SELECT 'dave@demo.test',  'active',  '-13 days'
      UNION ALL SELECT 'carol@demo.test', 'invited', '-2 days'
      UNION ALL SELECT 'eve@demo.test',   'pending', '-1 days') x
JOIN users u ON u.email = x.who
JOIN groups g ON g.title = 'Weekend Hikers'
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test';

INSERT OR IGNORE INTO group_members (group_id, user_id, status, created_at, updated_at)
SELECT g.id, u.id, 'active', datetime('now', '-12 days'), datetime('now', '-12 days')
FROM groups g
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'carol@demo.test'
JOIN users u ON u.email IN ('carol@demo.test', 'bob@demo.test')
WHERE g.title = 'Book Club';

-- Group post + comment
INSERT INTO posts (user_id, content, privacy_level, group_id, created_at, updated_at)
SELECT u.id, 'Who''s in for Sunday? I can drive three people from the station 🚗', 'public', g.id,
       datetime('now', '-1 days'), datetime('now', '-1 days')
FROM users u
JOIN groups g ON g.title = 'Weekend Hikers'
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test'
WHERE u.email = 'bob@demo.test';

INSERT INTO comments (post_id, user_id, content, created_at, updated_at)
SELECT p.id, u.id, 'Me please! I''ll bring snacks 🍫', datetime('now', '-20 hours'), datetime('now', '-20 hours')
FROM posts p
JOIN users u ON u.email = 'dave@demo.test'
WHERE p.content = 'Who''s in for Sunday? I can drive three people from the station 🚗';

-- Event with votes --------------------------------------------------------
INSERT INTO events (group_id, creator_id, title, description, event_time, created_at, updated_at)
SELECT g.id, g.creator_id, 'Sunday hike: Box Hill loop',
       'About 10 km, 3 hours. Meet at the station car park. Bring water and a jacket.',
       datetime('now', '+5 days', 'start of day', '+9 hours'),
       datetime('now', '-1 days'), datetime('now', '-1 days')
FROM groups g
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test'
WHERE g.title = 'Weekend Hikers';

INSERT OR IGNORE INTO event_responses (event_id, user_id, response)
SELECT e.id, u.id, x.response
FROM (SELECT 'alice@demo.test' AS who, 'going' AS response
      UNION ALL SELECT 'bob@demo.test', 'going'
      UNION ALL SELECT 'dave@demo.test', 'not_going') x
JOIN users u ON u.email = x.who
JOIN events e ON e.title = 'Sunday hike: Box Hill loop';

-- Chat --------------------------------------------------------------------
INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at, read_at)
SELECT s.id, r.id, x.content, 0, datetime('now', x.ago), datetime('now', x.ago)
FROM (SELECT 'alice@demo.test' AS s, 'bob@demo.test' AS r, 'Hey Bob! Are you coming on Sunday? 🙂' AS content, '-10 hours' AS ago
      UNION ALL SELECT 'bob@demo.test', 'alice@demo.test', 'Wouldn''t miss it! Bringing the new camera 📷', '-9 hours'
      UNION ALL SELECT 'alice@demo.test', 'bob@demo.test', 'Perfect, see you at the station at 9 👍', '-8 hours') x
JOIN users s ON s.email = x.s
JOIN users r ON r.email = x.r;

INSERT INTO messages (sender_id, group_id, content, is_group_message, created_at)
SELECT u.id, g.id, x.content, 1, datetime('now', x.ago)
FROM (SELECT 'alice@demo.test' AS who, 'Welcome to Weekend Hikers everyone! 🥾' AS content, '-14 days' AS ago
      UNION ALL SELECT 'bob@demo.test', 'Thanks for setting this up Alice!', '-14 days'
      UNION ALL SELECT 'dave@demo.test', 'Weather looks great for Sunday ☀️', '-6 hours') x
JOIN users u ON u.email = x.who
JOIN groups g ON g.title = 'Weekend Hikers'
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test';

-- Notifications matching the pending items above ---------------------------
-- Carol: Eve's follow request (related_id = the requester) and Alice's
-- group invitation; Alice: Eve's join request; Bob and Dave: the new event.
INSERT INTO notifications (user_id, type, content, related_id, actor_id, is_read, created_at)
SELECT carol.id, 'follow_request', 'Eve Rossi wants to follow you', eve.id, eve.id, 0, datetime('now', '-3 days')
FROM users carol JOIN users eve ON eve.email = 'eve@demo.test'
WHERE carol.email = 'carol@demo.test';

INSERT INTO notifications (user_id, type, content, related_id, is_read, created_at)
SELECT carol.id, 'group_invitation', 'Alice Walker invited you to join: Weekend Hikers', g.id, 0, datetime('now', '-2 days')
FROM users carol
JOIN groups g ON g.title = 'Weekend Hikers'
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test'
WHERE carol.email = 'carol@demo.test';

INSERT INTO notifications (user_id, type, content, related_id, actor_id, is_read, created_at)
SELECT g.creator_id, 'group_request', 'Eve Rossi requested to join: Weekend Hikers', g.id, eve.id, 0, datetime('now', '-1 days')
FROM groups g
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test'
JOIN users eve ON eve.email = 'eve@demo.test'
WHERE g.title = 'Weekend Hikers';

INSERT INTO notifications (user_id, type, content, related_id, is_read, created_at)
SELECT u.id, 'event_created', 'New event in Weekend Hikers: Sunday hike: Box Hill loop', g.id, 0, datetime('now', '-1 days')
FROM groups g
JOIN users creator ON creator.id = g.creator_id AND creator.email = 'alice@demo.test'
JOIN users u ON u.email IN ('bob@demo.test', 'dave@demo.test')
WHERE g.title = 'Weekend Hikers';
