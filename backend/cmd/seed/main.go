// Command seed fills the database with mock users and activity: 5 users for
// each of the 79 education combinations (395 users), each batch following
// the same pattern of groups, posts, events, likes, comments, follows and
// messages.
//
// Run it from backend/ after the server has created the database once (so
// every migration is applied):
//
//	go run ./cmd/seed
//
// It uses DB_PATH if set, otherwise pkg/db/sqlite/social-network.db. Every
// seed user has an @seed.test email and the password Seed-Pass-2026. If seed
// users already exist it does nothing, so running it twice is safe.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const (
	seedPassword = "Seed-Pass-2026"
	seedDomain   = "@seed.test"
)

// The 79 (education provider, qualification, RQF level) combinations - the
// same values the Settings page offers.
var combinations = []string{
	"Secondary School - GCSE - Level 1", "Secondary School - GCSE - Level 2", "Secondary School - A Level - Level 3",
	"Secondary School - BTEC - Level 1", "Secondary School - BTEC - Level 2", "Secondary School - BTEC - Level 3",
	"Sixth Form - GCSE - Level 2", "Sixth Form - A Level - Level 3", "Sixth Form - BTEC - Level 3",
	"Sixth Form - T Level - Level 3", "Sixth Form - V Level - Level 3",
	"Further Education (FE) College - GCSE - Level 1", "Further Education (FE) College - GCSE - Level 2",
	"Further Education (FE) College - A Level - Level 3", "Further Education (FE) College - T Level - Level 3",
	"Further Education (FE) College - V Level - Level 3", "Further Education (FE) College - Access to HE Diploma - Level 3",
	"Further Education (FE) College - BTEC - Entry Level", "Further Education (FE) College - BTEC - Level 1",
	"Further Education (FE) College - BTEC - Level 2", "Further Education (FE) College - BTEC - Level 3",
	"Further Education (FE) College - BTEC - Level 4", "Further Education (FE) College - BTEC - Level 5",
	"Further Education (FE) College - NVQ - Level 1", "Further Education (FE) College - NVQ - Level 2",
	"Further Education (FE) College - NVQ - Level 3", "Further Education (FE) College - NVQ - Level 4",
	"Further Education (FE) College - NVQ - Level 5",
	"Further Education (FE) College - Higher Technical Qualification (HTQ) - Level 4",
	"Further Education (FE) College - Higher Technical Qualification (HTQ) - Level 5",
	"Further Education (FE) College - Apprenticeship - Level 2", "Further Education (FE) College - Apprenticeship - Level 3",
	"Further Education (FE) College - Apprenticeship - Level 4", "Further Education (FE) College - Apprenticeship - Level 5",
	"University - Higher Technical Qualification (HTQ) - Level 4", "University - Higher Technical Qualification (HTQ) - Level 5",
	"University - Bachelor’s Degree - Level 6", "University - Master’s Degree - Level 7",
	"University - PhD - Level 8", "University - Doctorate - Level 8",
	"University - Apprenticeship - Level 6", "University - Apprenticeship - Level 7",
	"Higher Education Institution - Higher Technical Qualification (HTQ) - Level 4",
	"Higher Education Institution - Higher Technical Qualification (HTQ) - Level 5",
	"Higher Education Institution - Bachelor’s Degree - Level 6", "Higher Education Institution - Master’s Degree - Level 7",
	"Higher Education Institution - PhD - Level 8", "Higher Education Institution - Doctorate - Level 8",
	"Higher Education Institution - Apprenticeship - Level 6", "Higher Education Institution - Apprenticeship - Level 7",
	"Independent Training Provider (ITPs) - T Level - Level 3", "Independent Training Provider (ITPs) - V Level - Level 3",
	"Independent Training Provider (ITPs) - BTEC - Level 1", "Independent Training Provider (ITPs) - BTEC - Level 2",
	"Independent Training Provider (ITPs) - BTEC - Level 3", "Independent Training Provider (ITPs) - BTEC - Level 4",
	"Independent Training Provider (ITPs) - BTEC - Level 5",
	"Independent Training Provider (ITPs) - NVQ - Level 1", "Independent Training Provider (ITPs) - NVQ - Level 2",
	"Independent Training Provider (ITPs) - NVQ - Level 3", "Independent Training Provider (ITPs) - NVQ - Level 4",
	"Independent Training Provider (ITPs) - NVQ - Level 5",
	"Independent Training Provider (ITPs) - Apprenticeship - Level 2", "Independent Training Provider (ITPs) - Apprenticeship - Level 3",
	"Independent Training Provider (ITPs) - Apprenticeship - Level 4", "Independent Training Provider (ITPs) - Apprenticeship - Level 5",
	"Independent Training Provider (ITPs) - Apprenticeship - Level 6", "Independent Training Provider (ITPs) - Apprenticeship - Level 7",
	"Employer Provider - NVQ - Level 1", "Employer Provider - NVQ - Level 2", "Employer Provider - NVQ - Level 3",
	"Employer Provider - NVQ - Level 4", "Employer Provider - NVQ - Level 5",
	"Employer Provider - Apprenticeship - Level 2", "Employer Provider - Apprenticeship - Level 3",
	"Employer Provider - Apprenticeship - Level 4", "Employer Provider - Apprenticeship - Level 5",
	"Employer Provider - Apprenticeship - Level 6", "Employer Provider - Apprenticeship - Level 7",
}

var firstNames = []string{"Amelia", "Oliver", "Isla", "George", "Ava", "Harry", "Mia", "Noah", "Freya", "Leo", "Grace", "Arthur", "Evie", "Oscar", "Lily", "Jack", "Rosie", "Theo", "Poppy", "Alfie"}
var lastNames = []string{"Smith", "Jones", "Taylor", "Brown", "Williams", "Wilson", "Johnson", "Davies", "Patel", "Evans", "Thomas", "Roberts", "Khan", "Walker", "Wright", "Hughes", "Green", "Hall", "Wood", "Clarke"}

// batch holds the ids created for one education combination. users[0] is
// "User 1" of the brief, users[4] is "User 5".
type batch struct {
	users       [5]int64
	user1Groups [3]int64
	user3Group  int64
}

type seeder struct {
	tx    *sql.Tx
	rng   *rand.Rand
	clock time.Time // advances with every row, so created_at follows insert order
	count map[string]int
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("pkg", "db", "sqlite", "social-network.db")
	}
	if _, err := os.Stat(dbPath); err != nil {
		log.Fatalf("database %s not found - start the server once so it creates it and applies the migrations", dbPath)
	}
	if len(combinations) != 79 {
		log.Fatalf("expected 79 education combinations, got %d", len(combinations))
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email LIKE ?", "%"+seedDomain).Scan(&existing); err != nil {
		log.Fatalf("can't read users (are the migrations applied?): %v", err)
	}
	if existing > 0 {
		fmt.Printf("%d seed users already exist in %s - nothing to do.\n", existing, dbPath)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	s := &seeder{
		tx:    tx,
		rng:   rand.New(rand.NewSource(2026)), // fixed seed: the same data every run
		clock: time.Now().UTC().Add(-30 * 24 * time.Hour),
		count: map[string]int{},
	}

	batches := make([]*batch, len(combinations))
	for i, combo := range combinations {
		batches[i] = s.seedBatch(i, combo, string(hash))
	}
	s.seedCrossBatch(batches)

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Seeded %s:\n", dbPath)
	for _, k := range []string{"users", "follows", "groups", "group members", "posts", "private post viewers", "likes", "comments", "events", "direct messages", "group messages"} {
		fmt.Printf("  %-21s %d\n", k, s.count[k])
	}
	fmt.Printf("Log in with any <name>%s address above and the password %s.\n", seedDomain, seedPassword)
}

// seedBatch creates one combination's 5 users and everything that happens
// inside the batch.
func (s *seeder) seedBatch(i int, combo, hash string) *batch {
	parts := strings.Split(combo, " - ")
	provider, qualification, level := parts[0], parts[1], parts[2]
	b := &batch{}

	// Users 1 and 3 public, 2 and 5 private, 4 public (the connector).
	public := [5]bool{true, false, true, true, false}
	for r := 0; r < 5; r++ {
		first := firstNames[(i*5+r)%len(firstNames)]
		last := lastNames[(i*3+r*7)%len(lastNames)]
		email := fmt.Sprintf("%s.%s.%03d.%d%s", strings.ToLower(first), strings.ToLower(last), i+1, r+1, seedDomain)
		dob := time.Date(1985+s.rng.Intn(20), time.Month(1+s.rng.Intn(12)), 1+s.rng.Intn(28), 0, 0, 0, 0, time.UTC)
		about := fmt.Sprintf("%s %s at a %s.", qualification, level, provider)
		b.users[r] = s.insert("users",
			`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, about_me, is_public,
			 education_provider, qualification_name, rqf_level, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			email, hash, first, last, dob, about, public[r], provider, qualification, level, s.tick(), s.now())
	}
	u1, u2, u3, u4, u5 := b.users[0], b.users[1], b.users[2], b.users[3], b.users[4]

	// Follows (all accepted).
	s.follow(u2, u1)
	s.follow(u1, u2) // User 1 and User 2 follow each other
	s.follow(u1, u3)
	s.follow(u2, u3)
	s.follow(u4, u3) // User 3 is followed by the other public users and User 2
	s.follow(u3, u1) // lets User 1 pick User 3 as a viewer of a private post
	s.follow(u5, u1)
	s.follow(u5, u2)
	s.follow(u1, u5)
	s.follow(u2, u5)
	s.follow(u4, u5) // User 5 follows 1 and 2, and is followed back by 1, 2 and 4

	// User 1: 3 groups, User 2 joins the first; an event in each (the event
	// in the external group they join is added in seedCrossBatch).
	topics := []string{"Study Circle", "Revision Club", "Careers Corner"}
	for g := range b.user1Groups {
		b.user1Groups[g] = s.group(u1, fmt.Sprintf("%s %s: %s", qualification, level, topics[g]),
			fmt.Sprintf("For %s %s learners at a %s.", qualification, level, provider))
	}
	s.join(b.user1Groups[0], u2, u1)
	for _, g := range b.user1Groups {
		s.event(g, u1, "Meet-up")
	}

	// User 1: 5 posts - 3 public, 1 followers-only, 1 private for Users 2 and 3.
	var user1Posts []int64
	for p := 1; p <= 3; p++ {
		user1Posts = append(user1Posts, s.post(u1, fmt.Sprintf("%s %s tip #%d: little and often beats cramming.", qualification, level, p), "public", 0))
	}
	user1Posts = append(user1Posts, s.post(u1, "Followers only: sharing my revision timetable.", "almost_private", 0))
	private := s.post(u1, "Private: notes for my study partners.", "private", 0, u2, u3)
	user1Posts = append(user1Posts, private)

	// User 2: likes all 5 of User 1's posts (they can see every one of them).
	for _, p := range user1Posts {
		s.like(p, u2)
	}

	// User 3: 1 group with Users 1 and 4 as members, 50 posts and 5 events
	// in it, and a few group chat messages.
	b.user3Group = s.group(u3, fmt.Sprintf("%s %s Hub", provider, level), "Everything about "+qualification+".")
	s.join(b.user3Group, u1, u3)
	s.join(b.user3Group, u4, u3)
	for p := 1; p <= 50; p++ {
		s.post(u3, fmt.Sprintf("Group update %d for %s %s.", p, qualification, level), "public", b.user3Group)
	}
	for e := 1; e <= 5; e++ {
		s.event(b.user3Group, u3, fmt.Sprintf("Session %d", e))
	}
	for _, text := range []string{"Welcome to the group!", "Next session details are in the events.", "Shout if you need help with anything."} {
		s.groupMessage(b.user3Group, u3, text)
	}
	s.groupMessage(b.user3Group, u1, "Thanks, see you there!")
	s.groupMessage(b.user3Group, u4, "Count me in.")

	// User 5: 10 private posts, visible to their followers 1, 2 and 4, and
	// direct messages with each of them.
	for p := 1; p <= 10; p++ {
		s.post(u5, fmt.Sprintf("Private journal %d: %s %s progress.", p, qualification, level), "private", 0, u1, u2, u4)
	}
	for _, other := range []int64{u1, u2, u4} {
		s.dm(u5, other, "Hi! How is the course going?")
		s.dm(other, u5, "Going well, thanks - you?")
		s.dm(u5, other, "Good! Let's catch up this week.")
	}

	// User 3 likes User 1's private post (User 3 is one of its chosen viewers).
	s.like(private, u3)
	return b
}

// seedCrossBatch adds the activity that reaches outside a batch: User 1
// joins another batch's group and creates an event there, and Users 1, 3
// and 5 like and comment on public posts from other batches.
func (s *seeder) seedCrossBatch(batches []*batch) {
	type publicPost struct {
		id    int64
		batch int
	}
	var publicPosts []publicPost
	rows, err := s.tx.Query("SELECT id, user_id FROM posts WHERE privacy_level = 'public' AND group_id IS NULL")
	if err != nil {
		log.Fatal(err)
	}
	batchOf := map[int64]int{}
	for i, b := range batches {
		for _, u := range b.users {
			batchOf[u] = i
		}
	}
	for rows.Next() {
		var id, author int64
		if err := rows.Scan(&id, &author); err != nil {
			log.Fatal(err)
		}
		if bi, ok := batchOf[author]; ok {
			publicPosts = append(publicPosts, publicPost{id, bi})
		}
	}
	rows.Close()

	// pick returns n different public posts that aren't from batch own.
	pick := func(own, n int) []int64 {
		var picked []int64
		for _, i := range s.rng.Perm(len(publicPosts)) {
			if publicPosts[i].batch != own {
				picked = append(picked, publicPosts[i].id)
				if len(picked) == n {
					break
				}
			}
		}
		return picked
	}

	for i, b := range batches {
		u1, u3, u5 := b.users[0], b.users[2], b.users[4]

		// User 1 joins the next batch's first group and creates an event there.
		external := batches[(i+1)%len(batches)].user1Groups[0]
		s.join(external, u1, batches[(i+1)%len(batches)].users[0])
		s.event(external, u1, "Guest meet-up")

		for _, p := range pick(i, 10) {
			s.like(p, u1)
		}
		for _, p := range pick(i, 10) {
			s.comment(p, u1, "Really useful, thanks for sharing.")
		}
		for _, p := range pick(i, 20) {
			s.like(p, u3)
		}
		for _, p := range pick(i, 15) {
			s.comment(p, u3, "Great point - adding this to my notes.")
		}
		for _, p := range pick(i, 5) {
			s.like(p, u5)
		}
	}
}

func (s *seeder) now() time.Time { return s.clock }

func (s *seeder) tick() time.Time {
	s.clock = s.clock.Add(time.Duration(1+s.rng.Intn(5)) * time.Minute)
	return s.clock
}

// insert runs an INSERT, counts it under label, and returns the new row's id.
func (s *seeder) insert(label, query string, args ...interface{}) int64 {
	res, err := s.tx.Exec(query, args...)
	if err != nil {
		log.Fatalf("insert %s: %v", label, err)
	}
	s.count[label]++
	id, _ := res.LastInsertId()
	return id
}

func (s *seeder) follow(follower, following int64) {
	s.insert("follows", `INSERT INTO follows (follower_id, following_id, status, created_at, updated_at) VALUES (?, ?, 'accepted', ?, ?)`,
		follower, following, s.tick(), s.now())
}

// group creates a group with its creator as an active member.
func (s *seeder) group(creator int64, title, description string) int64 {
	id := s.insert("groups", `INSERT INTO groups (creator_id, title, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		creator, title, description, s.tick(), s.now())
	s.insert("group members", `INSERT INTO group_members (group_id, user_id, status, created_at, updated_at) VALUES (?, ?, 'active', ?, ?)`,
		id, creator, s.now(), s.now())
	return id
}

func (s *seeder) join(groupID, user, invitedBy int64) {
	s.insert("group members", `INSERT INTO group_members (group_id, user_id, status, invited_by, created_at, updated_at) VALUES (?, ?, 'active', ?, ?, ?)`,
		groupID, user, invitedBy, s.tick(), s.now())
}

// post creates a post; groupID 0 means not in a group, and viewers are the
// users allowed to see a private post.
func (s *seeder) post(author int64, content, privacy string, groupID int64, viewers ...int64) int64 {
	var group interface{}
	if groupID != 0 {
		group = groupID
	}
	id := s.insert("posts", `INSERT INTO posts (user_id, content, privacy_level, group_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		author, content, privacy, group, s.tick(), s.now())
	for _, v := range viewers {
		s.insert("private post viewers", `INSERT INTO post_allowed_users (post_id, user_id, created_at) VALUES (?, ?, ?)`, id, v, s.now())
	}
	return id
}

func (s *seeder) like(postID, user int64) {
	s.insert("likes", `INSERT OR IGNORE INTO post_likes (post_id, user_id, created_at) VALUES (?, ?, ?)`, postID, user, s.tick())
}

func (s *seeder) comment(postID, user int64, content string) {
	s.insert("comments", `INSERT INTO comments (post_id, user_id, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		postID, user, content, s.tick(), s.now())
}

// event creates an upcoming event, 1 to 60 days from now.
func (s *seeder) event(groupID, creator int64, title string) {
	when := time.Now().UTC().Add(time.Duration(24+s.rng.Intn(24*60)) * time.Hour).Truncate(time.Hour)
	s.insert("events", `INSERT INTO events (group_id, creator_id, title, description, event_time, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		groupID, creator, title, "Open to all group members.", when, s.tick(), s.now())
}

func (s *seeder) dm(from, to int64, content string) {
	s.insert("direct messages", `INSERT INTO messages (sender_id, recipient_id, content, is_group_message, created_at, read_at) VALUES (?, ?, ?, 0, ?, ?)`,
		from, to, content, s.tick(), s.now())
}

func (s *seeder) groupMessage(groupID, from int64, content string) {
	s.insert("group messages", `INSERT INTO messages (sender_id, group_id, content, is_group_message, created_at) VALUES (?, ?, ?, 1, ?)`,
		from, groupID, content, s.tick())
}
