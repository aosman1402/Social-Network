
# Functional

## Has the requirement for the allowed packages been respected? 
[note: bash didn't like it needs to be amended]
Yes - check by running bash command:
```zsh
allowed='^(github\.com/(gorilla/websocket|golang-migrate/migrate|rubenv/sql-migrate|mattn/go-sqlite3|google/uuid|gofrs/uuid)|golang\.org/x/crypto/bcrypt)'
imports=$(cd backend && go list -f '{{join .Imports "\n"}}{{"\n"}}{{join .TestImports "\n"}}{{"\n"}}{{join .XTestImports "\n"}}' ./... | sort -u | grep -v -e '^$' -e '^social-network')
std=$(cd backend && go list std)
echo "1) Standard Go packages used:"
echo $imports | grep -xFf <(echo $std) | sed 's/^/   OK  /'
echo "\n2) Project-specific (allowed) packages used:"
echo $imports | grep -vxFf <(echo $std) | grep -E $allowed | sed 's/^/   OK  /'
echo "\n3) Other packages:"
others=$( { echo $imports | grep -vxFf <(echo $std) | grep -vE $allowed; node -e 'Object.keys(require("./frontend/package.json").dependencies).filter(p=>!["next","react","react-dom"].includes(p)).forEach(p=>console.log(p+"  (frontend)"))'; } )
[[ -n $others ]] && echo $others | sed 's/^/   --  /' || echo "   none"
```

**Standard Go packages allowed:**
archive/tar, archive/zip, bufio, builtin, bytes, cmp, compress/bzip2, compress/flate, compress/gzip, compress/lzw, compress/zlib, container/heap, container/list, container/ring, context, crypto, crypto/aes, crypto/cipher, crypto/des, crypto/dsa, crypto/ecdh, crypto/ecdsa, crypto/ed25519, crypto/elliptic, crypto/fips140, crypto/hkdf, crypto/hmac, crypto/hpke, crypto/md5, crypto/mldsa, crypto/mlkem, crypto/mlkem/mlkemtest, crypto/pbkdf2, crypto/rand, crypto/rc4, crypto/rsa, crypto/sha1, crypto/sha256, crypto/sha3, crypto/sha512, crypto/subtle, crypto/tls, crypto/x509, crypto/x509/pkix, database/sql, database/sql/driver, debug/buildinfo, debug/dwarf, debug/elf, debug/gosym, debug/macho, debug/pe, debug/plan9obj, embed, encoding, encoding/ascii85, encoding/asn1, encoding/base32, encoding/base64, encoding/binary, encoding/csv, encoding/gob, encoding/hex, encoding/json, encoding/json/jsontext, encoding/json/v2, encoding/pem, encoding/xml, errors, expvar, flag, fmt, go/ast, go/build, go/build/constraint, go/constant, go/doc, go/doc/comment, go/format, go/importer, go/parser, go/printer, go/scanner, go/token, go/types, go/version, hash, hash/adler32, hash/crc32, hash/crc64, hash/fnv, hash/maphash, html, html/template, image, image/color, image/color/palette, image/draw, image/gif, image/jpeg, image/png, index/suffixarray, io, io/fs, io/ioutil, iter, log, log/slog, log/syslog, maps, math, math/big, math/bits, math/cmplx, math/rand, math/rand/v2, mime, mime/multipart, mime/quotedprintable, net, net/http, net/http/cgi, net/http/cookiejar, net/http/fcgi, net/http/httptest, net/http/httptrace, net/http/httputil, net/http/pprof, net/mail, net/netip, net/rpc, net/rpc/jsonrpc, net/smtp, net/textproto, net/url, os, os/exec, os/signal, os/user, path, path/filepath, plugin, reflect, regexp, regexp/syntax, runtime, runtime/cgo, runtime/coverage, runtime/debug, runtime/metrics, runtime/pprof, runtime/race, runtime/secret, runtime/trace, simd, simd/archsimd, slices, sort, strconv, strings, structs, sync, sync/atomic, syscall, syscall/js, testing, testing/cryptotest, testing/fstest, testing/iotest, testing/quick, testing/slogtest, testing/synctest, text/scanner, text/tabwriter, text/template, text/template/parse, time, time/tzdata, unicode, unicode/utf16, unicode/utf8, unique, unsafe, uuid, weak

**Additional packages allowed for Social-Network:**
* Gorilla websocket
* golang-migrate
* sql-migration
* migration
* sqlite3
* bcrypt
* gofrs/uuid or google/uuid

## Open the project

## When examining the file system of the backend, did you find a well-organized structure, similar to the example provided in the subject, with a clear separation of packages and migrations folders?
Yes, the expected outcome of our project is:
```zsh
backend
backend/pkg
backend/pkg/db
backend/pkg/db/migrations
backend/pkg/db/migrations/sqlite
backend/pkg/db/sqlite
backend/pkg/handlers
backend/pkg/middleware
backend/pkg/models
backend/pkg/websocket
```

Check the above by running the following bash command:
```zsh
find backend -type d -not -path '*/uploads*' | sort
```

## Is the file system for the frontend well organized?
Yes, the expected outcome of our project is:
```zsh
frontend/src
frontend/src/app
frontend/src/app/chat
frontend/src/app/chat/[id]
frontend/src/app/groups
frontend/src/app/groups/[id]
frontend/src/app/login
frontend/src/app/notifications
frontend/src/app/profile
frontend/src/app/profile/[id]
frontend/src/app/register
frontend/src/app/register/github
frontend/src/app/settings
frontend/src/components
frontend/src/context
frontend/src/hooks
frontend/src/lib
frontend/src/utils
```

Check the above by running the following bash command:
```zsh
find frontend/src -type d | sort
```


# Backend

## Does the backend include a clear separation of responsibilities among its three major parts - Server, App, and Database?
Yes

**What separation of concerns means:** each part of the program has one job and only knows what it needs for that job. You can change one part (e.g. add a route, change a query, add a migration) without rewriting the others, and each part can be tested on its own.

**How it's done here:**
- **Server - `backend/server.go`:** starts up and wires things together. It reads the config, asks the Database layer for a connection, hands that connection to the App (`DB: db.DB`, lines 62-76), registers the routes with CORS, auth and rate-limit wrappers, and starts HTTP (`ListenAndServe`). It has no SQL and no business logic.
- **App - `pkg/handlers`, `pkg/middleware`, `pkg/websocket`:** what happens on each request. Handlers validate input, apply the rules (privacy, followers, groups) and write the JSON response. Middleware checks sessions, CORS and the login rate limit. The websocket package runs the real-time hub (chat, notifications, presence). The App receives the DB connection from the Server and never opens the database or loads the SQLite driver itself.
- **Database - `pkg/db`:** `pkg/db/sqlite/sqlite.go` is the only file that opens SQLite, loads the driver and runs golang-migrate (`NewDB`, `ApplyMigrations`, `Close`, `Ping`, `HealthCheck`). The schema lives only in the numbered up/down migration files in `pkg/db/migrations/sqlite`.

Note: the handlers write their own SQL queries using the connection they are given (plain `database/sql`, the usual Go way). The Database layer owns the connection, the driver and the schema.

Check those out:
```zsh
cd backend
echo "1) SERVER - backend/server.go"
echo "   routes registered:        $(grep -c 'HandleFunc' server.go)"
echo "   DB handed to the App on lines: $(grep -n 'DB: *db.DB\|EnablePresence' server.go | cut -d: -f1 | tr '\n' ' ')"
echo "   starts HTTP:              $(grep -n 'ListenAndServe' server.go | head -1 | tr -s '\t ' ' ')"
echo "   SQL queries in server.go: $(grep -cE 'SELECT|INSERT|UPDATE |DELETE FROM' server.go)"
echo "\n2) APP - request logic (gets the DB handed in, never opens it)"
for d in handlers middleware websocket; do
  echo "   pkg/$d: $(ls pkg/$d/*.go | grep -vc _test.go) files - $(ls pkg/$d/*.go | grep -v _test.go | xargs -n1 basename | tr '\n' ' ')"
done
echo "   files in the App that open the DB / load the driver: $(grep -rlE 'sql\.Open|go-sqlite3|golang-migrate' pkg/handlers pkg/middleware pkg/websocket --include='*.go' | grep -vc _test.go)"
echo "\n3) DATABASE - pkg/db"
echo "   connection + migrations: pkg/db/sqlite/sqlite.go ($(grep -oE '^func (\([^)]*\) )?[A-Za-z]+' pkg/db/sqlite/sqlite.go | awk '{print $NF}' | tr '\n' ' '))"
echo "   migrations: $(ls pkg/db/migrations/sqlite/*.up.sql | wc -l | tr -d ' ') up + $(ls pkg/db/migrations/sqlite/*.down.sql | wc -l | tr -d ' ') down files in pkg/db/migrations/sqlite"
echo "   only file that opens the DB / uses the driver: $(grep -rlE 'sql\.Open|go-sqlite3|golang-migrate' --include='*.go' . | grep -v _test.go | tr '\n' ' ')"
cd ..
```


## Is there a server that effectively receives incoming requests and serves as the entry point for all requests to the application?
Yes - every route is registered in `backend/server.go` and served on :8081.

Check this out by running first:
```zsh
cd backend
go run server.go
```

Then on different terminal run (zsh, from the repo root, with the backend running):
```zsh
API=http://localhost:8081/api
echo "1) Server answers:            $(curl -s -m 3 $API/health || echo "NOT RUNNING - start the backend first")"
echo "2) Routes in server.go:       $(grep -cE 'http\.Handle(Func)?\(' backend/server.go)"
echo "3) Routes/servers elsewhere:  $(grep -rlE 'http\.Handle(Func)?\(|ListenAndServe' backend --include='*.go' | grep -v _test.go | grep -vc '^backend/server.go$')"
echo "4) Server starts on:          $(grep -n 'ListenAndServe' backend/server.go | tr -s '\t ' ' ')"
echo "5) Unknown route:             $(curl -s -m 3 -o /dev/null -w '%{http_code}' $API/does-not-exist)"
```

Explanation:
```t
1. The backend is running and answered the health check.
2. All 60 routes are registered in server.go.
3. No other file registers routes or starts a server.
4. The server starts at line 208, on port 8081.
5. An unknown path gets a 404 from the same server.
```

## Does the application (App) running on the server effectively listen for requests, retrieve information from the database, and send responses?
Yes

Check this out by running (zsh, from the repo root, with the backend running - works on its own, no setup needed):
```zsh
setopt nobanghist
API=http://localhost:8081/api
PW='Sup3rSec9!zx'
H=(-H 'Content-Type: application/json' -H 'Origin: http://localhost:3000')
curl -s "${H[@]}" $API/register -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\",\"first_name\":\"Alice\",\"last_name\":\"Test\",\"date_of_birth\":\"1995-04-12\"}"; echo
curl -s -c alice.jar "${H[@]}" $API/login -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\"}"; echo
ALICE=$(curl -s -b alice.jar "${H[@]}" $API/check-auth | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2); echo "alice=$ALICE"
curl -s -b alice.jar "${H[@]}" "$API/user/profile?id=$ALICE"; echo
```
The last line returns Alice's row from the database as JSON (`"email":"alice@t.test","first_name":"Alice",...`). The first line may say the email is already taken on a second run - that's fine.

## Is the core logic of the social network implemented within the App component, including the logic for handling various types of requests based on HTTP or other protocols?
Yes. The question has three parts:

**1) Is the core logic in the App?** Yes. Every rule of the social network lives in the App (`pkg/handlers` + `pkg/websocket`, 24 files). `server.go` only wires things together and has no SQL or rules. 
Examples of the core logic:
- Who can see a post - one shared rule used by the feed, single post, profile posts and comments (`pkg/handlers/post_response.go:78`). Line by line: the author always; a group post only active group members; otherwise public = everyone, almost private = accepted followers, private = the followers the author picked:
```go
const postVisibilityClause = `(
    p.user_id = ?
    OR (p.group_id IS NOT NULL AND p.group_id IN (
        SELECT group_id FROM group_members WHERE user_id = ? AND status = 'active'
    ))
    OR (p.group_id IS NULL AND (
        p.privacy_level = 'public'
        OR (p.privacy_level = 'almost_private' AND p.user_id IN (
            SELECT following_id FROM follows WHERE follower_id = ? AND status = 'accepted'
        ))
        OR (p.privacy_level = 'private' AND p.id IN (
            SELECT post_id FROM post_allowed_users WHERE user_id = ?
        ))
    ))
)`
```
- Who can see a profile - public, or private and followed (`pkg/handlers/user.go:101`):
```go
func (uh *UserHandler) canViewAccount(viewerID, targetID int64) (bool, error) {
	if viewerID == targetID {
		return true, nil
	}
	var isPublic bool
	if err := uh.DB.QueryRow("SELECT is_public FROM users WHERE id = ?", targetID).Scan(&isPublic); err != nil {
		return false, err
	}
	if isPublic {
		return true, nil
	}
	return uh.hasAcceptedFollow(viewerID, targetID)
}
```
- The same pattern covers followers and follow requests, groups (invites, join requests, events), notifications and chat.

**2) Does it handle HTTP requests?** Yes. Each HTTP handler checks the method, reads the input, applies the rule and answers with a JSON body and the right status code (`pkg/handlers/auth.go:225`):
```go
func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if r.Method != "POST" {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }
```

**3) Does it handle other protocols?** Yes - WebSocket. `/ws` checks the session cookie, then upgrades the HTTP connection to a WebSocket (`pkg/handlers/websocket.go:105`):
```go
conn, err := upgrader.Upgrade(w, r, nil)
```
After that, the client reads each WebSocket message and handles it by type (`pkg/websocket/client.go:109`):
```go
switch wsMsg.Type {
case "chat":         // private or group message: save it, send it to the recipient(s)
case "typing":       // typing indicator
case "read_receipt": // mark a message as read
case "ping":         // keep-alive, answers "pong"
}
```
The server also pushes notifications, new posts and online/offline status over the same connection.

Check this out (zsh, from the repo root, with the backend running - works on its own, no setup needed):
```zsh
setopt nobanghist
API=http://localhost:8081/api
PW='Sup3rSec9!zx'
H=(-H 'Content-Type: application/json' -H 'Origin: http://localhost:3000')
curl -s "${H[@]}" $API/register -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\",\"first_name\":\"Alice\",\"last_name\":\"Test\",\"date_of_birth\":\"1995-04-12\"}" >/dev/null
curl -s -c alice.jar "${H[@]}" $API/login -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\"}" >/dev/null
src=(${(f)"$(ls backend/pkg/handlers/*.go backend/pkg/websocket/*.go | grep -v _test.go)"})

echo "1) CORE LOGIC IS IN THE APP (pkg/handlers + pkg/websocket), NOT IN server.go"
echo "   App: $(cat $src | wc -l | tr -d ' ') lines in ${#src} files, $(cat $src | grep -c '^func ') functions"
echo "   server.go: $(grep -cE 'SELECT|INSERT|UPDATE |DELETE FROM' backend/server.go) SQL queries, only wiring"
echo "   rules, where they live:"
grep -noE 'const postVisibilityClause|func (\([^)]*\) )?(canViewPost|canViewAccount|hasAcceptedFollow|validateChatContent)' $src | sed -E 's#^backend/##; s/func \([^)]*\) /func /; s/^/     /'

echo "\n2) HTTP REQUESTS - each handler checks the method and answers with JSON"
echo "   GET  /api/register (POST only)  -> $(curl -s -o /dev/null -w '%{http_code}' $API/register)  (405 = method not allowed)"
echo "   POST /api/register (bad data)   -> $(curl -s -w ' %{http_code}' "${H[@]}" $API/register -d '{}' | tr -d '\n')"
echo "   GET  /api/posts (no session)    -> $(curl -s -w ' %{http_code}' "${H[@]}" $API/posts | tr -d '\n')"
echo "   GET  /api/posts (logged in)     -> $(curl -s -o /dev/null -w '%{http_code}' -b alice.jar "${H[@]}" $API/posts)  (200 = feed returned)"

echo "\n3) OTHER PROTOCOL - WebSocket on /ws (chat, typing, read receipts, ping)"
echo "   handshake without session       -> $(curl -s -o /dev/null -w '%{http_code}' -m 2 -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' http://localhost:8081/ws)  (401 = rejected)"
echo "   handshake with session          -> $(curl -s -o /dev/null -w '%{http_code}' -m 2 -b alice.jar -H 'Origin: http://localhost:3000' -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' http://localhost:8081/ws)  (101 = switched to WebSocket)"
echo "   same session, from another site -> $(curl -s -o /dev/null -w '%{http_code}' -m 2 -b alice.jar -H 'Origin: http://evil.example.com' -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' http://localhost:8081/ws)  (403 = only our frontend may connect)"
echo "   message types handled: $(grep -oE 'case "[a-z_]+"' backend/pkg/websocket/client.go | cut -d'"' -f2 | tr '\n' ' ')"

echo "\n4) TESTS for the App"
(cd backend && go test ./pkg/handlers ./pkg/websocket ./pkg/middleware 2>&1 | sed 's/^/   /')
rm -f alice.jar
```


# Database

## Is SQLite being used in the project as the database?
Yes - SQLite is the only database in the project.

Run the following (zsh, from the repo root):
```zsh
echo "Files using SQLite:"
grep -rlE 'sql\.Open\("sqlite3"|mattn/go-sqlite3|migrate/v4/database/sqlite3' backend --include='*.go' --include='go.mod' | sed 's/^/   OK  /'
echo "Files using any other database:"
grep -rliE 'postgres|mysql|mariadb|mongo|redis|lib/pq|pgx|gorm|sqlserver|mssql|firestore|dynamodb|supabase|prisma' backend frontend/src frontend/package.json docker-compose.yml --include='*.go' --include='go.mod' --include='*.js' --include='*.json' --include='*.yml' 2>/dev/null | sed 's/^/   --  /' | grep . || echo "   none"
```

## Are clients able to request information stored in the database, and can they submit data to be added to it without encountering errors or issues?
Yes - clients submit data (a post, a comment) that is saved in the database, and request it back, with no errors.

Check this out by running the following (zsh, from the repo root, with the backend running):
```zsh
API=http://localhost:8081/api
PW='Sup3rSec9!zx'
H=(-H 'Origin: http://localhost:3000')
TXT="audit post $(date +%s)"
curl -s "${H[@]}" -H 'Content-Type: application/json' $API/register -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\",\"first_name\":\"Alice\",\"last_name\":\"Test\",\"date_of_birth\":\"1995-04-12\"}" >/dev/null
curl -s -c alice.jar "${H[@]}" -H 'Content-Type: application/json' $API/login -d "{\"email\":\"alice@t.test\",\"password\":\"$PW\"}" >/dev/null
R=$(curl -s -w ' HTTP %{http_code}' -b alice.jar "${H[@]}" -F "content=$TXT" -F privacy_level=public $API/post/create)
POST=$(echo $R | grep -oE '"post_id":[0-9]+' | cut -d: -f2)
echo "1) SUBMIT a post:          $(echo $R | grep -oE '"message":"[^"]*"|HTTP [0-9]+' | tr '\n' ' ')-> post id $POST"
echo "2) SUBMIT a comment:       $(curl -s -w ' HTTP %{http_code}' -b alice.jar "${H[@]}" -F 'content=audit comment' -F post_id=$POST $API/post/comment | grep -oE '"message":"[^"]*"|HTTP [0-9]+' | tr '\n' ' ')"
echo "3) REQUEST the post back:  $(curl -s -w ' HTTP %{http_code}' -b alice.jar "${H[@]}" "$API/post/get?id=$POST" | grep -oE "\"content\":\"$TXT\"|HTTP [0-9]+" | tr '\n' ' ')"
echo "4) REQUEST comments back:  $(curl -s -w ' HTTP %{http_code}' -b alice.jar "${H[@]}" "$API/post/comments?post_id=$POST" | grep -oE '"content":"audit comment"|HTTP [0-9]+' | tr '\n' ' ')"
rm -f alice.jar
```

Expected output (the post id and number change on each run):
```
1) SUBMIT a post:          "message":"Post created successfully" HTTP 201 -> post id number
2) SUBMIT a comment:       "message":"Comment created successfully" HTTP 200
3) REQUEST the post back:  "content":"audit post number" HTTP 200
4) REQUEST comments back:  "content":"audit comment" HTTP 200
```

## Does the app implement a migration system?
Yes - the app uses golang-migrate. It runs every time the server starts and brings the database up to the latest schema.

**Where it sits:**
- Migration files: `backend/pkg/db/migrations/sqlite/` - 22 numbered pairs `NNNNNN_name.up.sql` (apply) / `NNNNNN_name.down.sql` (undo).
- Runner: `ApplyMigrations` in `backend/pkg/db/sqlite/sqlite.go` (the Database layer).
- Started from: `backend/server.go:56`, right after the database is opened and before any route is served.

**How it is implemented** (`backend/pkg/db/sqlite/sqlite.go`):
```go
driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{
    DatabaseName: "main",
    NoTxWrap:     false,
})
m, err := migrate.NewWithDatabaseInstance(
    "file://"+absPath,
    "sqlite3",
    driver,
)
if err := m.Up(); err != nil && err != migrate.ErrNoChange {
    return fmt.Errorf("failed to apply migrations: %v", err)
}
```
golang-migrate reads the files in number order, runs each `.up.sql` that hasn't been applied yet, and records the new version. If the database is already up to date, `m.Up()` returns `ErrNoChange` and nothing happens.

**Reference schema:**
- Tracking table, created by golang-migrate: `schema_migrations (version uint64, dirty bool)`. `version` is the last migration applied; `dirty=1` would mean a migration failed halfway.
- Each schema change is its own migration. Example pair, `000001_create_users_table`: the up file creates the `users` table (`CREATE TABLE IF NOT EXISTS users (...)`), the down file removes it (`DROP TABLE IF EXISTS users;`).

Check this out (zsh, from the repo root):
```zsh
M=backend/pkg/db/migrations/sqlite
DB=backend/pkg/db/sqlite/social-network.db
echo "1) WHERE IT SITS"
echo "   migration files:  $M ($(ls $M/*.up.sql | wc -l | tr -d ' ') up + $(ls $M/*.down.sql | wc -l | tr -d ' ') down)"
echo "   runner:           backend/pkg/db/sqlite/sqlite.go -> func ApplyMigrations"
echo "   started from:     backend/server.go:$(grep -n 'ApplyMigrations(' backend/server.go | cut -d: -f1) (every server start)"
echo "\n2) HOW IT IS IMPLEMENTED (golang-migrate, backend/pkg/db/sqlite/sqlite.go)"
grep -nE 'sqlite3\.WithInstance|migrate\.NewWithDatabaseInstance|"file://"|m\.Up\(\)' backend/pkg/db/sqlite/sqlite.go | tr -s '\t ' ' ' | sed 's/^/   /'
echo "\n3) REFERENCE SCHEMA"
echo "   tracking table:   $(sqlite3 $DB '.schema schema_migrations' | head -1)"
echo "   current state:    version=$(sqlite3 $DB 'SELECT version FROM schema_migrations') dirty=$(sqlite3 $DB 'SELECT dirty FROM schema_migrations')"
echo "   example pair:     000001_create_users_table"
echo "     up:   $(grep -m1 'CREATE TABLE' $M/000001_create_users_table.up.sql)"
echo "     down: $(cat $M/000001_create_users_table.down.sql)"
echo "   all migrations, in order:"
ls $M/*.up.sql | xargs -n1 basename | sed 's/\.up\.sql//; s/^/     /'
```

## Is that migration file system well organized? (like the example from the subject)
Yes - it follows the subject's recommended structure exactly.

**What "well organized" means for migrations:**
1. **Predictable location** - one folder for migrations, separate from the code that runs them.
2. **Consistent naming** - `NNNNNN_description.up.sql` / `.down.sql`: a 6-digit number, then what the migration does.
3. **Reversible** - every `up` (apply) has a matching `down` (undo).
4. **Ordered** - numbered 1, 2, 3... with no gaps or duplicates, so they always run in the same order.
5. **Clean** - nothing in the folder except migration files.
6. **Small steps** - one change per migration, so the history of the schema is readable.

**The subject's recommendation** (also in `README.md`, "Database & Migrations") - this project uses the same paths:
```
backend
├── pkg
│   ├── db
│   │   ├── migrations
│   │   │   └── sqlite
│   │   │       ├── 000001_create_users_table.down.sql
│   │   │       ├── 000001_create_users_table.up.sql
│   │   │       ├── 000002_create_posts_table.down.sql
│   │   │       └── 000002_create_posts_table.up.sql
│   │   └── sqlite
│   │       └── sqlite.go
│   └── ...other_pkgs.go
└── server.go
```
`sqlite.go` connects to the database and applies the migrations with golang-migrate from `file://.../pkg/db/migrations/sqlite` - exactly as recommended.

Check the above (zsh, from the repo root) - checks each rule and prints OK or FAIL:
```zsh
M=backend/pkg/db/migrations/sqlite
ok() { [[ $1 == 0 ]] && echo "OK  " || echo "FAIL" }
echo "1) Same layout as the subject's example:"
for p in backend/server.go backend/pkg/db/sqlite/sqlite.go $M; do echo "   $(ok $([[ -e $p ]]; echo $?))  $p"; done
bad=$(ls $M | grep -vcE '^[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql$')
echo "2) $(ok $bad)  Naming NNNNNN_description.up|down.sql: $(ls $M | wc -l | tr -d ' ') files, $bad wrong"
nopair=$(ls $M/*.up.sql | sed 's/\.up\.sql$//' | while read f; do [[ -f $f.down.sql ]] || echo $f; done | wc -l | tr -d ' ')
echo "3) $(ok $nopair)  Every .up.sql has its .down.sql: $(ls $M/*.up.sql | wc -l | tr -d ' ') pairs, $nopair missing"
n=$(ls $M/*.up.sql | wc -l | tr -d ' ')
seq_ok=$([[ "$(ls $M/*.up.sql | xargs -n1 basename | cut -c1-6 | sed 's/^0*//' | tr '\n' ' ')" == "$(seq 1 $n | tr '\n' ' ')" ]]; echo $?)
echo "4) $(ok $seq_ok)  Numbered 1 to $n in order, no gaps or duplicates"
other=$(ls -A $M | grep -vc '\.sql$')
echo "5) $(ok $other)  Only migration files in the folder ($other other files)"
echo "6) The migrations (one change each, the name says what it does):"
ls $M/*.up.sql | xargs -n1 basename | sed 's/\.up\.sql$//; s/^/         /'
```


## Start the social network application, then enter the database using the command "sqlite3 <database_name.db>".

Terminal 1 - start the backend (it applies the migrations on start):
```zsh
cd backend && go run server.go
```
Terminal 2 - start the frontend (open http://localhost:3000):
```zsh
cd frontend && npm run dev
```
Terminal 3 - enter the database (from the repo root) for testing purposes:
```zsh
sqlite3 backend/pkg/db/sqlite/social-network.db
```
Inside `sqlite3`, type these one by one:
```
.tables
.headers on
.mode column
SELECT * FROM schema_migrations;
SELECT id, email, first_name FROM users LIMIT 3;
.quit
```

## Are the migrations being applied by the migration system?
Yes - every migration file has been applied to the database.

**How to tell:** each time the backend starts, golang-migrate runs any migration files the database hasn't had yet, in number order. It keeps a small table, `schema_migrations`, holding two values:
- `version` - the number of the last migration it applied
- `dirty` - `0` if that migration finished cleanly, `1` if it failed halfway

So the migrations have been applied when `version` equals the number of the newest file in `backend/pkg/db/migrations/sqlite` (22) and `dirty` is `0`. You can also check that the tables and data those files create are really in the database.

Prove (zsh, from the repo root, after the backend has been started once):
```zsh
DB=backend/pkg/db/sqlite/social-network.db
M=backend/pkg/db/migrations/sqlite
newest=$(ls $M/*.up.sql | tail -1 | xargs basename)
latest=$(echo $newest | cut -c1-6 | sed 's/^0*//')
applied=$(sqlite3 $DB 'SELECT version FROM schema_migrations')
dirty=$(sqlite3 $DB 'SELECT dirty FROM schema_migrations')
echo "Newest migration file:        $newest (number $latest)"
echo "Last migration applied to DB: $applied"
echo "Finished cleanly:             dirty=$dirty (0 = yes)"
echo "Tables from migration 1:      $(sqlite3 $DB "SELECT name FROM sqlite_master WHERE name='users'")"
echo "Tables from migration 21:     $(sqlite3 $DB "SELECT group_concat(name,' ') FROM sqlite_master WHERE type='table' AND name LIKE 'oauth%'")"
echo "Demo users from migration 22: $(sqlite3 $DB "SELECT COUNT(*) FROM users WHERE email LIKE '%@demo.test'")"
[[ $latest == $applied && $dirty == 0 ]] && echo "RESULT: OK - all $latest migrations have been applied" || echo "RESULT: FAIL - database is at $applied, files go up to $latest"
```


# How to check the answers below

Every answer is followed by the fastest accurate check. Most are terminal blocks - run them from the repo root in zsh, with the backend running (`cd backend && go run server.go`):

- **`source audit.zsh`** (first line of every block) loads five helpers: `mk alice` registers and logs in a **brand-new** user and sets `$ALICE` to its id (`mk carol private` makes it private); `as alice GET /path` / `as alice POST /path '{json}'` calls the API as that user and prints `STATUS body` (`as anon ...` = not logged in); `asf` does the same for uploads; `sql "..."` queries the database. Fresh users every time, so the results never depend on earlier tests. When you're done, `audit_cleanup` deletes every user the audit created (`*@audit.test` and `alice@t.test`), everything they made (via `ON DELETE CASCADE`) and their uploaded images - demo and seed users are left alone.
- Registration is limited to 20 per minute per computer - if `mk` prints "Too many attempts", wait a minute.
- Real-time (WebSocket) behaviour is proved by the project's Go tests (`go test ... -run <name> -v`), which open real WebSocket connections; the browser check is given alongside.

# Authentication

## Does the app implement sessions for the authentication of the users?
Yes - login stores a random token in the `sessions` table (user + 30-day expiry) and sends it as an HttpOnly `session_token` cookie; every private request is checked against that table; logout deletes the row.
```zsh
source audit.zsh; mk alice
curl -s -D - -o /dev/null -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' $AUDIT_API/login -d "{\"email\":\"$ALICE_EMAIL\",\"password\":\"$AUDIT_PW\"}" | grep -i '^set-cookie'
sql "SELECT user_id, substr(session_token,1,8)||'...' AS token, expires_at FROM sessions WHERE user_id=$ALICE"
as alice GET /check-auth
as anon GET /check-auth
as alice POST /logout
as alice GET /check-auth
```
Expect: `Set-Cookie: session_token=...; Expires=<+30 days>; HttpOnly; SameSite=Lax`; one `sessions` row per login (two here: `mk` + this login); `"authenticated":true` with the cookie, `false` without it; after logout the same cookie gives `false`.
Browser: DevTools → **Application → Cookies → http://localhost:3000** → `session_token`, **HttpOnly ✓**; `document.cookie` in the Console doesn't show it.

## Are the correct form elements being used in the registration? (Email, Password, First Name, Last Name, Date of Birth, Avatar/Image (Optional), Nickname (Optional), About Me (Optional))
Yes - open http://localhost:3000/register: Email, First Name, Last Name, Date of Birth (date picker), Password + Confirm Password, Nickname (optional, live availability check), About Me (optional), Avatar (optional: upload JPG/PNG/GIF or pick a preset).
```zsh
grep -nE "type='(email|password|date)'|name='(first_name|last_name|about_me)'|<NicknameField|<AvatarPicker" frontend/src/app/register/page.js
```

# Try to register a user.

## During registration, when attempting to register a user, did the application correctly save the registered user to the database without any errors?
Yes.
```zsh
source audit.zsh; E="dave$RANDOM@audit.test"
as anon POST /register "{\"email\":\"$E\",\"password\":\"$AUDIT_PW\",\"first_name\":\"Dave\",\"last_name\":\"Test\",\"date_of_birth\":\"1990-01-01\",\"nickname\":\"dave$RANDOM\",\"about_me\":\"hi\"}"
sql "SELECT id, email, first_name, last_name, date_of_birth, nickname, about_me, substr(password_hash,1,7) AS hash FROM users WHERE email='$E'"
```
Expect: `201 {"message":"User registered successfully","user_id":N}` and one row with every field; `hash` = `$2a$10$` (bcrypt - the password itself is never stored).
Browser: DevTools → Network → `register` → **201 Created**; then open `backend/pkg/db/sqlite/social-network.db` in VS Code (SQLite Viewer) → `users`.


# Try to log in with the user you just registered.

## When attempting to log in with the user you just registered, did the login process work without any problems?
Yes.
```zsh
source audit.zsh; mk alice
as anon POST /login "{\"email\":\"$ALICE_EMAIL\",\"password\":\"$AUDIT_PW\"}"
sql "SELECT COUNT(*) AS sessions_for_alice FROM sessions WHERE user_id=$ALICE"
```
Expect: `200 {"message":"Login successful","user_id":N}` and a new session row (2: one from `mk`, one from this login). Correct logins are never rate-limited.


# Try to log in with the user you created, but with a wrong password or email.

## Did the application correctly detect and respond to the incorrect login details?
Yes - same `401 "Invalid credentials"` for a wrong password and an unknown email (doesn't reveal which emails exist), and no session is created.
```zsh
source audit.zsh; mk alice
as anon POST /login "{\"email\":\"$ALICE_EMAIL\",\"password\":\"WrongPass123\"}"
as anon POST /login "{\"email\":\"nobody$RANDOM@audit.test\",\"password\":\"$AUDIT_PW\"}"
sql "SELECT COUNT(*) AS sessions_for_alice FROM sessions WHERE user_id=$ALICE"
```
Expect: two `401 {"error":"Invalid credentials"}`, and still only 1 session (the one from `mk`).
Browser: red "Invalid credentials" under the form; Network → `login` → 401, no `Set-Cookie`.
Also protected: 5 failures on one account in 15 min lock that account (429) whatever the IP; 10 failures per minute from one IP get 429 (`go test ./pkg/handlers -run TestLoginLocksAccountAfterFiveFailures -v` from `backend`).


# Try to register the same user you already registered.

## Did the app detect if the email/user is already present in the database?
Yes - also with different capitals (emails are stored lowercase and compared case-insensitively).
```zsh
source audit.zsh; mk alice
as anon POST /register "{\"email\":\"$ALICE_EMAIL\",\"password\":\"$AUDIT_PW\",\"first_name\":\"A\",\"last_name\":\"B\",\"date_of_birth\":\"1990-01-01\"}"
as anon POST /register "{\"email\":\"${(U)ALICE_EMAIL}\",\"password\":\"$AUDIT_PW\",\"first_name\":\"A\",\"last_name\":\"B\",\"date_of_birth\":\"1990-01-01\"}"
sql "SELECT COUNT(*) AS accounts FROM users WHERE email='$ALICE_EMAIL'"
```
Expect: two `409 {"error":"Email already in use"}` and `accounts` = 1.
Browser: red "Email already in use" on the register page; Network → `register` → 409, and no `login` request follows.


# Open two browsers (ex: Chrome and Firefox), log in into one and refresh the other browsers.

## Can you confirm that the browser non logged remains unregistered?
Yes - the session is a cookie held only by the browser that logged in; without it the server answers "not logged in" and the page redirects to /login.
```zsh
source audit.zsh; mk alice
as alice GET /check-auth
as anon GET /check-auth
as anon GET /posts
```
Expect: `"authenticated":true` with the cookie, `{"authenticated":false}` without, and `401 {"error":"Unauthorized"}` for any private route.
Browser: the second browser stays on /login after refreshing; its Application → Cookies has no `session_token`.


# Using the two browsers, log in with different users in each one. Then refresh both the browsers.

## Can you confirm that both browsers continue with the right users?
Yes - each login gets its own token, and the server looks the user up by token.
```zsh
source audit.zsh; mk alice; mk bob
as alice GET /check-auth
as bob GET /check-auth
```
Expect: each returns its own `id` and `email`.
Browser: each navbar keeps showing its own name after refreshing.


# Followers


# Try to follow a private user.

## Are you able to send a following request to the private user?
Yes.
```zsh
source audit.zsh; mk alice; mk carol private
as alice POST /user/follow "{\"user_id\":$CAROL}"
sql "SELECT follower_id, following_id, status FROM follows WHERE follower_id=$ALICE AND following_id=$CAROL"
as carol GET /user/follow-requests | cut -c1-80
```
Expect: `"message":"Follow request sent","status":"pending"`, a `pending` row, and Alice in Carol's follow requests.
Browser: Alice clicks **Follow Request** on Carol's profile → button becomes **Cancel Request**; Carol's bell shows "Alice ... requested to follow you" live and her profile has a **Requests (1)** tab.


# Try to follow a public user.

## Are you able to follow the public user without the need of sending a following request?
Yes.
```zsh
source audit.zsh; mk alice; mk eve
as alice POST /user/follow "{\"user_id\":$EVE}"
sql "SELECT follower_id, following_id, status FROM follows WHERE follower_id=$ALICE AND following_id=$EVE"
```
Expect: `"message":"You are now following this user","status":"accepted"` and an `accepted` row straight away.
Browser: the button goes straight from **Follow** to **Following**; Eve gets "Alice ... started following you" (information only, nothing to accept).


# Open two browsers(ex: Chrome and Firefox), log in as two different private users and with one of them try to follow the other.

## Is the user who received the request able to accept or decline the following request?
Yes.
```zsh
source audit.zsh; mk dave private; mk carol private; mk eve private
as dave POST /user/follow "{\"user_id\":$CAROL}" >/dev/null; as eve POST /user/follow "{\"user_id\":$CAROL}" >/dev/null
as carol POST /user/follow-request "{\"follower_id\":$DAVE,\"action\":\"accept\"}"
as carol POST /user/follow-request "{\"follower_id\":$EVE,\"action\":\"decline\"}"
sql "SELECT follower_id, following_id, status FROM follows WHERE following_id=$CAROL"
```
Expect: `Follow request accepted` / `Follow request declined`, and the rows become `accepted` / `declined`; Dave and Eve each get a `follow_accepted` / `follow_declined` notification.
Browser: Carol → own profile → **Requests** tab → ✓ Accept / ✕ Decline.


# After following another user successfully try to unfollow him.

## Were you able to do so?
Yes.
```zsh
source audit.zsh; mk alice; mk bob
as alice POST /user/follow "{\"user_id\":$BOB}" >/dev/null
as alice POST /user/unfollow "{\"user_id\":$BOB}"
sql "SELECT COUNT(*) AS follow_rows FROM follows WHERE follower_id=$ALICE AND following_id=$BOB"
```
Expect: `200 {"message":"Unfollowed successfully"}` and `follow_rows` = 0 (row deleted); Bob gets an "Alice ... unfollowed you" notification.
Browser: click **Following** on the profile → confirm → button back to **Follow**, Followers count −1.


# Profile


# Try opening your own profile.

## Does the profile displays every information requested in the register form, apart from the password?
Yes - avatar, first + last name, @nickname, about me, email, date of birth (plus join date and a Public/Private badge); the password hash is never read or sent.
```zsh
source audit.zsh; mk alice
as alice GET "/user/profile?id=$ALICE"
as alice GET "/user/profile?id=$ALICE" | grep -c -i password
```
Expect: every register field in the JSON, and `0` - no password field at all.
Browser: navbar → **Profile**.


# Try opening your own profile.

## Does the profile displays every post created by the user?
Yes - Profile → **Posts** tab (open by default), newest first; on your own profile it includes your public, almost-private, private and group posts.
```zsh
source audit.zsh; mk alice
for level in public almost_private private; do asf alice /post/create -F "content=my $level post" -F privacy_level=$level | cut -c1-40; done
as alice GET "/user/posts?id=$ALICE" | grep -o '"content":"[^"]*"'
```
Expect: three `201`s, then all three posts listed.
Note: the profile shows the newest 50 posts (no "load more").


# Try opening your own profile.

## Does the profile displays the users that you follow and the ones who are following you?
Yes - Profile → **Followers (N)** and **Following (N)** tabs (accepted follows only; pending ones are in **Requests**).
```zsh
source audit.zsh; mk alice; mk bob; mk eve
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null; as alice POST /user/follow "{\"user_id\":$EVE}" >/dev/null
as alice GET "/user/followers?id=$ALICE" | grep -o '"email":"[^"]*"'
as alice GET "/user/following?id=$ALICE" | grep -o '"email":"[^"]*"'
```
Expect: Bob as follower, Eve as following.


# Try opening your own profile.

## Are you able to change between private profile and public profile?
Yes - own profile → **🌍 Public / 🔒 Private** button (with a confirmation pop-up), or Settings → Profile → "Make my profile public". Stored in `users.is_public` (1 = public, 0 = private).
```zsh
source audit.zsh; mk alice
as alice POST /user/update '{"first_name":"Alice","last_name":"Test","about_me":"","is_public":false}'
sql "SELECT id, is_public FROM users WHERE id=$ALICE"
as alice POST /user/update '{"first_name":"Alice","last_name":"Test","about_me":"","is_public":true}'
sql "SELECT id, is_public FROM users WHERE id=$ALICE"
```
Expect: `is_public` 0, then 1. Switching to public also accepts any pending follow requests.


# Open two browsers and log in with different users on them, with one of the users having a private profile and successfully follow that user.

## Are you able to see a followed user private profile?
Yes - private profiles, their posts and follower lists open once `follows.status = 'accepted'` (`hasAcceptedFollow` / `canViewAccount` in `backend/pkg/handlers/user.go`).
```zsh
source audit.zsh; mk alice; mk carol private
asf carol /post/create -F "content=carol followers-only post" -F privacy_level=almost_private >/dev/null
as alice GET "/user/profile?id=$CAROL" | cut -c1-50
as alice POST /user/follow "{\"user_id\":$CAROL}" >/dev/null; as carol POST /user/follow-request "{\"follower_id\":$ALICE,\"action\":\"accept\"}" >/dev/null
as alice GET "/user/profile?id=$CAROL" | cut -c1-50
as alice GET "/user/posts?id=$CAROL" | grep -o '"content":"[^"]*"'
```
Expect: `403 ... "This profile is private"` before, `200` with the full profile after accepting, and Carol's followers-only post visible.


# Using the two browsers with the same users, with one of the users having a private profile and be sure not to follow him.

## Are you prevented from seeing a non-followed user private profile?
Yes - only name, avatar and a **Follow Request** button are shown; details, posts and follower lists are refused by the server, not just hidden by the page.
```zsh
source audit.zsh; mk alice; mk carol private
as alice GET "/user/profile?id=$CAROL"
as alice GET "/user/posts?id=$CAROL"
as alice GET "/user/followers?id=$CAROL"
as alice GET "/user/following?id=$CAROL"
```
Expect: four `403`s; the profile one carries only name/avatar/nickname (no email, date of birth or about me).


# Using the two browsers with the users, with one of the users having a public profile and be sure not to follow him.

## Are you able to see a non-followed user public profile?
Yes - the full profile and the public posts; followers-only (almost private) posts stay hidden until you follow.
```zsh
source audit.zsh; mk alice; mk eve
asf eve /post/create -F "content=eve public post" -F privacy_level=public >/dev/null
asf eve /post/create -F "content=eve followers-only post" -F privacy_level=almost_private >/dev/null
as alice GET "/user/profile?id=$EVE" | cut -c1-50
as alice GET "/user/posts?id=$EVE" | grep -o '"content":"[^"]*"'
```
Expect: `200` and only `"eve public post"`.


# Using the two browsers with the users, with one of the users having a public profile and successfully follow that user.

## Are you able to see a followed user public profile?
Yes - and after following, the followers-only posts appear too.
```zsh
source audit.zsh; mk alice; mk eve
asf eve /post/create -F "content=eve public post" -F privacy_level=public >/dev/null
asf eve /post/create -F "content=eve followers-only post" -F privacy_level=almost_private >/dev/null
as alice POST /user/follow "{\"user_id\":$EVE}" >/dev/null
as alice GET "/user/posts?id=$EVE" | grep -o '"content":"[^"]*"'
```
Expect: both posts.


# Posts

## Are you able to create a post and comment on already existing posts after logging in?
Yes - and not without logging in.
```zsh
source audit.zsh; mk alice; mk bob
P=$(asf bob /post/create -F "content=bob's post" -F privacy_level=public | grep -o '"post_id":[0-9]*' | cut -d: -f2)
asf alice /post/create -F "content=hello from alice" -F privacy_level=public | cut -c1-50
asf alice /post/comment -F post_id=$P -F "content=nice post, bob" | cut -c1-50
as anon POST /post/create
```
Expect: `201 Post created`, `200 {"comment":...}`, and `401 Unauthorized` when logged out.
Browser: home feed → the "What's on your mind?" box; open any post's comments to add one.


# Try creating a post.

## Are you able to include an image (JPG or PNG) or a GIF on it?
Yes - JPG, PNG and GIF are accepted; anything else is refused, checked by the file's real content, not its name.
```zsh
source audit.zsh; mk alice
for f in pic.png pic.jpg pic.gif fake.png; do echo "$f -> $(asf alice /post/create -F "content=image test" -F privacy_level=public -F image=@$IMG/$f | grep -oE '^[0-9]+|"image_path":"[^"]*"|"error":"[^"]*"' | tr '\n' ' ')"; done
```
Expect: `201` with an `/uploads/....png|.jpg|.gif` path for the three images, and `400 "Invalid image type. Only JPEG, PNG, and GIF are allowed"` for `fake.png` (a text file named .png).


# Try creating a comment.

## Are you able to include an image (JPG or PNG) or a GIF on it?
Yes - same rules as posts.
```zsh
source audit.zsh; mk alice
P=$(asf alice /post/create -F content=post -F privacy_level=public | grep -o '"post_id":[0-9]*' | cut -d: -f2)
for f in pic.png pic.jpg pic.gif fake.png; do echo "$f -> $(asf alice /post/comment -F post_id=$P -F "content=image comment" -F image=@$IMG/$f | grep -oE '^[0-9]+|"image_path":"[^"]*"|"error":"[^"]*"' | tr '\n' ' ')"; done
```
Expect: `200` with an image path for the three images, `400` for `fake.png`.


# Try creating a post.

## Can you specify the type of privacy of the post (public, almost private, private)?
Yes - a selector in the post box; stored in `posts.privacy_level`; any other value is refused.
```zsh
source audit.zsh; mk alice
for level in public almost_private private; do asf alice /post/create -F "content=$level" -F privacy_level=$level | grep -oE '^[0-9]+|"privacy_level":"[^"]*"' | tr '\n' ' '; echo; done
asf alice /post/create -F content=x -F privacy_level=everyone
```
Expect: three `201`s with the matching `privacy_level`, then `400 "Invalid privacy level..."`.

## If you choose the private privacy option, can you specify the users that are allowed to see the post?
Yes - choosing **Private** shows a checklist of your followers; only the ones ticked (stored in `post_allowed_users`) can see it.
```zsh
source audit.zsh; mk alice; mk bob; mk carol
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null; as carol POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
P=$(asf alice /post/create -F "content=only for bob" -F privacy_level=private -F allowed_user_ids=$BOB | grep -o '"post_id":[0-9]*' | cut -d: -f2)
sql "SELECT post_id, user_id AS allowed_user FROM post_allowed_users WHERE post_id=$P"
echo "bob:   $(as bob GET "/post/get?id=$P" | cut -c1-50)"
echo "carol: $(as carol GET "/post/get?id=$P" | cut -c1-50)"
```
Expect: one allowed row (Bob); Bob gets `200` with the post, Carol - also a follower, but not picked - gets `404 "Post not found"`.


# Groups

# Try creating a group.

## Were you able to invite one of your followers to join the group?
Yes - and only followers / people you follow can be invited.
```zsh
source audit.zsh; mk alice; mk bob; mk carol
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
G=$(as alice POST /group/create '{"title":"Hikers","description":"weekend walks"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
echo "invite bob (follower):         $(as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}")"
echo "invite carol (not a follower): $(as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$CAROL}")"
sql "SELECT group_id, user_id, status FROM group_members WHERE group_id=$G"
```
Expect: `200 User invited successfully` for Bob, `403` for Carol; rows: Alice `active`, Bob `invited`.
Browser: Groups → create → open the group → **+ Invite** → pick from your followers.


# Open two browsers, log in with different users on each browser, follow each other and with one of the users create a group and invite the other user.

## Did the other user received a group invitation that he/she can refuse/accept?
Yes.
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
G1=$(as alice POST /group/create '{"title":"Group A","description":"a"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
G2=$(as alice POST /group/create '{"title":"Group B","description":"b"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G1,\"user_id\":$BOB}" >/dev/null; as alice POST /group/invite "{\"group_id\":$G2,\"user_id\":$BOB}" >/dev/null
sql "SELECT type, content FROM notifications WHERE user_id=$BOB AND type LIKE 'group%'"
echo "accept A:  $(as bob POST /group/invitation "{\"group_id\":$G1,\"action\":\"accept\"}")"
echo "decline B: $(as bob POST /group/invitation "{\"group_id\":$G2,\"action\":\"decline\"}")"
sql "SELECT group_id, user_id, status FROM group_members WHERE user_id=$BOB"
```
Expect: two `group_invitation` notifications; `Invitation accepted` / `Invitation declined`; Bob is `active` in Group A and has no row in Group B (a decline removes the invitation).
Browser: Bob's bell → the invitation with **Accept / Decline**.


# Using the same browsers and the same users, with one of the users create a group and with the other try to make a group entering request.

## Did the owner of the group received a request that he/she can refuse/accept?
Yes.
```zsh
source audit.zsh; mk alice; mk bob; mk carol
G=$(as alice POST /group/create '{"title":"Readers","description":"books"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as bob POST /group/join "{\"group_id\":$G}"; as carol POST /group/join "{\"group_id\":$G}"
sql "SELECT type, content FROM notifications WHERE user_id=$ALICE AND type LIKE 'group%'"
RB=$(sqlite3 $AUDIT_DB "SELECT id FROM group_members WHERE group_id=$G AND user_id=$BOB"); RC=$(sqlite3 $AUDIT_DB "SELECT id FROM group_members WHERE group_id=$G AND user_id=$CAROL")
echo "accept bob:    $(as alice POST /group/request "{\"request_id\":$RB,\"action\":\"accept\"}")"
echo "decline carol: $(as alice POST /group/request "{\"request_id\":$RC,\"action\":\"decline\"}")"
sql "SELECT user_id, status FROM group_members WHERE group_id=$G"
```
Expect: two `group_request` notifications for the owner; `Request accepted` / `Request declined`; Bob `active`, Carol gone.
Browser: owner's bell, or the group page → **Pending Join Requests** → **Accept / Decline**.

## Can a user make group invitations, after being part of the group (being the user different from the creator of the group)?
Yes.
```zsh
source audit.zsh; mk alice; mk bob; mk carol
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null; as carol POST /user/follow "{\"user_id\":$BOB}" >/dev/null
G=$(as alice POST /group/create '{"title":"Cooks","description":"recipes"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}" >/dev/null; as bob POST /group/invitation "{\"group_id\":$G,\"action\":\"accept\"}" >/dev/null
as bob POST /group/invite "{\"group_id\":$G,\"user_id\":$CAROL}"
sql "SELECT user_id, status, invited_by FROM group_members WHERE group_id=$G"
```
Expect: `200 User invited successfully`; Carol `invited`, `invited_by` = Bob's id.

## Can a user make a group entering request (a request to enter a group)?
Yes - Groups → a group you're not in → **Join →** (or **Join Group** on its page); the tile turns into **⏳ Requested**, the request is stored as `group_members.status = 'pending'` and the owner is notified. Proved by the two `/group/join` calls in the check two questions above (`Request to join sent successfully`).

## After being part of a group, can the user create posts and comment already created posts?
Yes - and non-members can't post, comment on or read them.
```zsh
source audit.zsh; mk alice; mk bob; mk carol
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
G=$(as alice POST /group/create '{"title":"Runners","description":"5k"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}" >/dev/null; as bob POST /group/invitation "{\"group_id\":$G,\"action\":\"accept\"}" >/dev/null
P=$(asf alice /post/create -F "content=first run on sunday" -F group_id=$G | grep -o '"post_id":[0-9]*' | cut -d: -f2)
echo "member bob posts:     $(asf bob /post/create -F "content=count me in" -F group_id=$G | cut -c1-40)"
echo "member bob comments:  $(asf bob /post/comment -F post_id=$P -F "content=see you there" | cut -c1-40)"
echo "outsider carol posts: $(asf carol /post/create -F "content=let me in" -F group_id=$G)"
echo "outsider carol reads: $(as carol GET "/group/posts?group_id=$G")"
```
Expect: `201` and `200` for Bob; `403 "You are not a member of this group"` twice for Carol.


# Try to create an event in a group.

## Were you asked for a title, a description, a day/time and at least two options (going, not going)?
Yes - group page → Events section (members only) → the form has **Title**, **Description**, **Day and time** (date-time picker) and "Options members can choose": **Going** / **Not going**. Title and time are required and the time can't be in the past.
```zsh
source audit.zsh; mk alice
G=$(as alice POST /group/create '{"title":"Chess","description":"club"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
WHEN=$(date -u -v+2d +%Y-%m-%dT18:00:00Z)
echo "no title:    $(as alice POST /group/event/create "{\"group_id\":$G,\"title\":\"\",\"description\":\"x\",\"event_time\":\"$WHEN\"}")"
echo "in the past: $(as alice POST /group/event/create "{\"group_id\":$G,\"title\":\"Old\",\"description\":\"x\",\"event_time\":\"2020-01-01T10:00:00Z\"}")"
echo "complete:    $(as alice POST /group/event/create "{\"group_id\":$G,\"title\":\"Tournament\",\"description\":\"Bring a board\",\"event_time\":\"$WHEN\"}" | cut -c1-40)"
grep -n 'value: "going"\|value: "not_going"' frontend/src/components/GroupEvents.js
```
Expect: `400 "Title and time are required"`, `400 "Event time can't be in the past"`, `200 {"event":...}`, and the two options in the form code.


# Using the same browsers and the same users, after both of them becoming part of the same group, create an event with one of them.

## Is the other user able to see the event and vote in which option he wants?
Yes.
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
G=$(as alice POST /group/create '{"title":"Film club","description":"movies"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}" >/dev/null; as bob POST /group/invitation "{\"group_id\":$G,\"action\":\"accept\"}" >/dev/null
as alice POST /group/event/create "{\"group_id\":$G,\"title\":\"Movie night\",\"description\":\"Popcorn\",\"event_time\":\"$(date -u -v+2d +%Y-%m-%dT19:00:00Z)\"}" >/dev/null
E=$(sqlite3 $AUDIT_DB "SELECT id FROM events WHERE group_id=$G")
echo "bob sees:  $(as bob GET "/group/events?group_id=$G" | grep -o '"title":"[^"]*"')"
echo "bob votes: $(as bob POST /group/event/respond "{\"event_id\":$E,\"response\":\"going\"}")"
sql "SELECT r.event_id, u.first_name, r.response FROM event_responses r JOIN users u ON u.id=r.user_id WHERE r.event_id=$E"
```
Expect: `"title":"Movie night"`, `Response recorded successfully`, and Bob `going` in `event_responses`.
Browser: Bob's group page → Events section → **Going (N) / Not going (N)** buttons; the counts update live in Alice's browser.


# Chat

# Try and open two browsers (ex: Chrome and Firefox), log in with different users in each one. Then with one of the users try to send a private message to the other user.

## Did the other user received the message in realtime?
Yes - messages are pushed over the WebSocket the moment they're sent.
```zsh
(cd backend && go test ./pkg/handlers -count=1 -v -run 'TestRealtimeChatDeliveredToFollowingRecipient|TestDirectMessageDeliveredOnceWithAck' | grep -E '^--- |^ok')
```
Expect: both `--- PASS` - two real WebSocket clients; the recipient gets the message exactly once.
Browser: chat open in both browsers (the users must follow each other or one the other) → the message appears instantly, no refresh.


# Try and open two browsers (ex: Chrome and Firefox), log in with different users that are not following each other at all. Then with one of the users try to send a private message to the other user.

## Can you confirm that it was not possible to create a chat between these two users?
Yes - refused by the server over HTTP and WebSocket, and nothing is stored.
```zsh
source audit.zsh; mk alice; mk bob
echo "send:    $(as alice POST /messages/send "{\"recipient_id\":$BOB,\"content\":\"hi bob\"}")"
echo "history: $(as alice GET "/messages/history?user_id=$BOB")"
sql "SELECT COUNT(*) AS messages FROM messages WHERE sender_id=$ALICE"
```
Expect: `403 "Cannot message this user"`, `403 "Cannot chat with this user"`, `messages` = 0. WebSocket side: `go test ./pkg/handlers -run TestChatBlockedSendOverWebSocketNotifiesSender -v` (from `backend`).
Browser: the other user doesn't appear in the chat contact list.


# Using the two browsers with the users start a chat between the two of them.

## Did the chat between the users went well? (did not crash the server)
Yes.
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
for i in {1..50}; do as alice POST /messages/send "{\"recipient_id\":$BOB,\"content\":\"message $i\"}" >/dev/null; as bob POST /messages/send "{\"recipient_id\":$ALICE,\"content\":\"reply $i\"}" >/dev/null; done
echo "server still up: $(as anon GET /health)"
sql "SELECT COUNT(*) AS messages_saved FROM messages WHERE sender_id IN ($ALICE,$BOB)"
```
Expect: `200 {"status":"ok"}` and `messages_saved` = 100.


# Try and open three browsers (ex: Chrome and Firefox or a private browser), log in with different users in each one. Then with one of the users try to send a private message to one of the other users.

## Did only the targeted user received the message?
Yes - each message goes only to the recipient's own connections.
```zsh
(cd backend && go test ./pkg/handlers -count=1 -v -run 'TestJoinRoomCannotEavesdrop|TestDirectMessageDeliveredOnceWithAck' | grep -E '^--- |^ok')
```
Expect: both `--- PASS` - a third connected user never receives someone else's message, even if they try to join the other user's room.


# Using the three browsers with the users, enter with each user a common group. Then start sending messages to the common chat room using one of the users.

## Did all the users that are common to the group receive the message in realtime?
Yes - and only the group's members.
```zsh
(cd backend && go test ./pkg/handlers -count=1 -v -run 'TestGroupChatDeliveredOnceToEachMember|TestGroupMessageOnlyReachesMembers' | grep -E '^--- |^ok')
```
Expect: both `--- PASS` - every active member gets each message exactly once, live; a non-member gets nothing.
Browser: the group page's chat panel, open in three browsers.


# Using the three browsers with the users, continue chatting between the users in the group.

## Did the chat between the users went well? (did not crash the server)
Yes.
```zsh
(cd backend && go test ./pkg/handlers -count=1 -v -run 'TestGroupChatBurstArrivesWholeAndInOrder' | grep -E '^--- |^ok')
curl -s localhost:8081/api/health
```
Expect: `--- PASS` (a quick burst of messages arrives complete and in order) and `{"status":"ok"}`.

## Can you confirm that it is possible to send emojis via chat to other users?
Yes - stored and delivered unchanged; the group chat has an emoji picker, and in private chat they can be typed (OS emoji panel: `Ctrl+Cmd+Space` on Mac, `Win+.` on Windows).
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
as alice POST /messages/send "{\"recipient_id\":$BOB,\"content\":\"Hello 😀🎉👍❤️\"}" | cut -c1-60
as bob GET "/messages/history?user_id=$ALICE" | grep -o '"content":"[^"]*"'
```
Expect: Bob's history shows `"content":"Hello 😀🎉👍❤️"` exactly.


# Notifications

## Can you check the notifications on every page of the project?
Yes - the bell is in the navbar, which `frontend/src/app/layout.js` renders on every page except login/register.
```zsh
grep -n "Navbar" frontend/src/app/layout.js; grep -n "NotificationBell" frontend/src/components/Navbar.js
```


# Open two browsers, log in as two different private users and with one of them try to follow the other.

## Did the other user received a notification regarding the following request?
Yes - pushed live and stored.
```zsh
source audit.zsh; mk dave private; mk carol private
as dave POST /user/follow "{\"user_id\":$CAROL}" >/dev/null
as carol GET /notifications | grep -oE '"type":"follow_request"|"content":"[^"]*requested to follow you"'
```
Expect: `"type":"follow_request"` and `"Dave Test requested to follow you"`.


# Open two browsers, log in with different users on each browser, follow each other and with one of the users create a group and invite the other user.

## Did the invited user received a notification regarding the group invitation request?
Yes.
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null; as alice POST /user/follow "{\"user_id\":$BOB}" >/dev/null
G=$(as alice POST /group/create '{"title":"Climbers","description":"rocks"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}" >/dev/null
as bob GET /notifications | grep -oE '"type":"group_invitation"|"content":"[^"]*invited you[^"]*"'
```
Expect: `"type":"group_invitation"` and `"Alice Test invited you to join: Climbers"`.


# Open two browsers, log in with different users on each browser, create a group with one of them and with the other send a group entering request.

## Did the other user received a notification regarding the group entering request?
Yes.
```zsh
source audit.zsh; mk alice; mk carol
G=$(as alice POST /group/create '{"title":"Painters","description":"art"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as carol POST /group/join "{\"group_id\":$G}" >/dev/null
as alice GET /notifications | grep -oE '"type":"group_request"|"content":"[^"]*requested to join[^"]*"'
```
Expect: `"type":"group_request"` and `"Carol Test requested to join: Painters"`. Live push: `go test ./pkg/handlers -run TestEveryJoinRequestNotifiesCreator -v` (from `backend`).


# Open two browsers, log in with different users on each browser, become part of the same group with both users and with one of the users create an event.

## Did the other user received a notification regarding the creation of the event?
Yes - every other active member, not the creator.
```zsh
source audit.zsh; mk alice; mk bob
as bob POST /user/follow "{\"user_id\":$ALICE}" >/dev/null
G=$(as alice POST /group/create '{"title":"Gamers","description":"games"}' | grep -o '"group_id":[0-9]*' | cut -d: -f2)
as alice POST /group/invite "{\"group_id\":$G,\"user_id\":$BOB}" >/dev/null; as bob POST /group/invitation "{\"group_id\":$G,\"action\":\"accept\"}" >/dev/null
as alice POST /group/event/create "{\"group_id\":$G,\"title\":\"LAN party\",\"description\":\"bring snacks\",\"event_time\":\"$(date -u -v+1d +%Y-%m-%dT20:00:00Z)\"}" >/dev/null
as bob GET /notifications | grep -oE '"type":"event_created"|"content":"New event[^"]*"'
echo "creator's own event notifications: $(as alice GET /notifications | grep -c event_created)"
```
Expect: Bob has `"type":"event_created"`, `"New event in Gamers: LAN party"`; the creator has 0. Live push: `go test ./pkg/handlers -run TestEventCreatedNotifiesOnlyOtherActiveMembers -v` (from `backend`).


# Docker


# Try to run the application and use the docker command "docker ps -a"

## Can you confirm that there are two containers (backend and frontend), and both containers have non-zero sizes indicating that they are not empty?
Yes.
```zsh
docker-compose up -d --build
docker ps -a --size --format 'table {{.Names}}\t{{.Status}}\t{{.Size}}'
```
Expect: `social-network-backend-1` and `social-network-frontend-1`, both `Up`, each with a non-zero size (the second number in brackets is the image size).
If the backend keeps restarting with "Dirty database version", its Docker volume holds a database from older migrations: `docker-compose down -v` (deletes the Docker copy of the data) and start again.


# Try to access the social network application through your web browser.

## Were you able to access the social network application through your web browser after running the docker containers, confirming that the containers are running and serving the application as expected?
Yes - http://localhost:3000 serves the app; the backend answers on :8081.
```zsh
curl -s -o /dev/null -w 'frontend: %{http_code}\n' http://localhost:3000/login; curl -s localhost:8081/api/health
```
Expect: `frontend: 200` and `{"status":"ok"}`. The browser talks to `localhost:8081` directly, so open the app on the machine running Docker.


# Bonus

## +Can you log in using Github or other type of external OAuthenticator (open standard for access delegation)?
Yes - "Continue with GitHub" on the Login page ("Sign up with GitHub" on Register). It uses one GitHub OAuth App, owned by a teammate's GitHub account. Any GitHub user can log in with it - the owner only decides the app's name and keys. The keys live in a `.env` file that git ignores, so they are never committed.

**Setup (once):**
1. The teammate who owns the app creates it on their GitHub: **Settings → Developer settings → OAuth Apps → New OAuth App** (https://github.com/settings/developers):
   - Homepage URL: `http://localhost:3000`
   - Authorization callback URL: `http://localhost:8081/api/auth/github/callback`
2. They copy the **Client ID**, click **Generate a new client secret**, copy it (GitHub shows it only once), and send both **privately** - never through the repo.
3. On each computer that runs the app, save them in a `.env`:
   - with `go run`: `cp backend/.env.example backend/.env`
   - with Docker: `cp .env.example .env` (repo root)

   then fill in:
   ```
   GITHUB_CLIENT_ID=<Client ID>
   GITHUB_CLIENT_SECRET=<Client Secret>
   ```
4. Restart the backend (`cd backend && go run server.go`, or `docker-compose up -d --build`). The log shows `GitHub login enabled`.

**Check it:**
```zsh
curl -s localhost:8081/api/auth/providers; echo
```
→ `{"github":true}` (`false` = the keys weren't picked up: check the file name/location and restart). Then http://localhost:3000/login → **Continue with GitHub** → authorize → you land logged in. GitHub's authorize screen shows the app name and "by <owner's username>" - that's expected. A first-time GitHub user is asked only for their date of birth (plus optional nickname / about me).

**Good to know:**
- Only the owner can manage the app - if the secret needs replacing, they generate a new one and send it again.
- `backend/.env` and `.env` are gitignored, so `git add .` never picks them up.
- After grading, the owner deletes the app, so the shared secret stops working.
- Without the keys, the button shows as unavailable with these setup steps, and the rest of the app works normally.

## +Did the student created a migration to fill the database?
Yes - migration `000022_seed_demo_data` inserts the demo data (5 users `alice`, `bob`, `carol`, `dave`, `eve` `@demo.test`, password `Demo-Pass-2026`, with follows, posts, groups, an event and messages). A migration runs only once, so to see it insert the data, undo just that migration and let the backend run it again. Only the demo rows are affected. From the repo root, with the backend stopped:

1. What's in the database now:
```zsh
sqlite3 backend/pkg/db/sqlite/social-network.db "SELECT email FROM users WHERE email LIKE '%@demo.test'; SELECT version FROM schema_migrations;"
```
2. Undo the seed migration (removes only the demo data):
```zsh
sqlite3 backend/pkg/db/sqlite/social-network.db < backend/pkg/db/migrations/sqlite/000022_seed_demo_data.down.sql
sqlite3 backend/pkg/db/sqlite/social-network.db "UPDATE schema_migrations SET version = 21; SELECT COUNT(*) AS demo_users FROM users WHERE email LIKE '%@demo.test';"
```
→ `demo_users` = 0
3. Start the backend - it runs the seed migration again (`Migrations applied successfully`):
```zsh
cd backend && go run server.go
```
4. In another terminal, check again:
```zsh
sqlite3 backend/pkg/db/sqlite/social-network.db "SELECT email FROM users WHERE email LIKE '%@demo.test'; SELECT version FROM schema_migrations;"
```
→ the 5 demo emails are back and the version is `22` again.

## +If you unfollow a user, do you get a confirmation pop-up?
Yes - "Unfollow this user? They'll be notified." (`frontend/src/components/FollowButton.js:56`, `profile/[id]/page.js:257`).

## +If you change your profile from public to private (or vice versa), do you get a confirmation pop-up?
Yes - explains what changes, both directions (`profile/[id]/page.js:326`, `SettingsProfileTab.js:161`).

## +Is there other notification apart from the ones explicit on the subject?
Yes - follow accepted / declined, new follower, unfollow, join request accepted / declined, invitation accepted / declined, comments, likes, profile went public / private.
Prove: `grep -rhoE 'notifType :?= "[a-z_]+"' backend/pkg/handlers/*.go | sort -u`

## +Does the project present a script to build the images and containers? (using a script to simplify the build)
Yes - `./build.sh` in the repo root checks Docker is running, builds both images and starts the containers (`docker-compose up -d --build`), then lists the two containers with their sizes and prints the app URLs.
```zsh
./build.sh
```

## +Do you think in general this project is well done?
Yes - every required feature works (58/58 checks against the Docker build, all tests passing), and the issues found earlier (login rate limit, debug output and password logging on the register page, email case, no per-account lockout) are fixed and tested. Do one run-through in real browsers before the audit.