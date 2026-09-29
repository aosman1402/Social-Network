# Helpers for the proof commands in AUDIT.md.
# Load once per terminal, from the repo root:   source audit.zsh
# Needs the backend running on :8081 (cd backend && go run server.go).

AUDIT_API=${AUDIT_API:-http://localhost:8081/api}
AUDIT_DB=${AUDIT_DB:-backend/pkg/db/sqlite/social-network.db}
AUDIT_PW='Sup3rSec9!zx'
AUDIT_JARS=${TMPDIR:-/tmp}/sn-audit-jars
mkdir -p $AUDIT_JARS

# mk NAME [private] - registers a brand-new user (NAME<random>@audit.test),
# logs it in, and sets $NAME (in capitals, e.g. $ALICE) to its user id and
# $NAME_EMAIL to its email.
# Fresh users every time, so results never depend on earlier tests.
mk() {
  local name=$1 email="$1$(date +%s)$RANDOM@audit.test" id reg
  reg=$(curl -s -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' $AUDIT_API/register \
    -d "{\"email\":\"$email\",\"password\":\"$AUDIT_PW\",\"first_name\":\"${(C)name}\",\"last_name\":\"Test\",\"date_of_birth\":\"1990-01-01\"}")
  if [[ $reg != *user_id* ]]; then
    echo "mk $name: registration failed: $reg"
    [[ $reg == *"Too many"* ]] && echo "  (more than 20 registrations in the last minute from this computer - wait a minute and run the block again)"
    return 1
  fi
  id=$(curl -s -c $AUDIT_JARS/$name.jar -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' $AUDIT_API/login \
    -d "{\"email\":\"$email\",\"password\":\"$AUDIT_PW\"}" | grep -o '"user_id":[0-9]*' | cut -d: -f2)
  typeset -g ${(U)name}=$id ${(U)name}_EMAIL=$email
  if [[ $2 == private ]]; then
    as $name POST /user/update "{\"first_name\":\"${(C)name}\",\"last_name\":\"Test\",\"about_me\":\"\",\"is_public\":false}" >/dev/null
  fi
  echo "${(U)name}=$id  ($email${2:+, $2})"
}

# as NAME METHOD PATH [JSON] - sends a request as NAME (use "anon" for no
# login) and prints "STATUS body". Long bodies are cut at 300 characters on
# screen, but passed whole when piped (e.g. into grep).
as() {
  local name=$1 method=$2 route=$3 body=$4 jar=() out
  [[ $name != anon ]] && jar=(-b $AUDIT_JARS/$name.jar)
  out=$(curl -s -X $method $jar -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
    ${body:+-d} ${body:+$body} -w '\n%{http_code}' "$AUDIT_API$route")
  _audit_print "$out"
}

# asf NAME PATH -F field=value ... - same as "as", for form uploads (posts,
# comments, images). Example: asf alice /post/create -F content=hi -F image=@pic.png
asf() {
  local name=$1 route=$2 out; shift 2
  out=$(curl -s -b $AUDIT_JARS/$name.jar -H 'Origin: http://localhost:3000' "$@" -w '\n%{http_code}' "$AUDIT_API$route")
  _audit_print "$out"
}

# Small real test images (and a text file pretending to be one) for the
# upload checks: $AUDIT_JARS/pic.png, pic.jpg, pic.gif, fake.png
print -n 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=' | base64 --decode > $AUDIT_JARS/pic.png
print -n 'R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7' | base64 --decode > $AUDIT_JARS/pic.gif
print -n '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=' | base64 --decode > $AUDIT_JARS/pic.jpg
print 'just text, not an image' > $AUDIT_JARS/fake.png
IMG=$AUDIT_JARS

_audit_print() {
  local code=${1##*$'\n'} body=${1%$'\n'*}
  [[ -t 1 ]] && body=${body[1,300]}
  echo "$code $body"
}

# sql "QUERY" - runs a query on the database, with column headers.
sql() { sqlite3 -header -column $AUDIT_DB "$1"; }

# audit_cleanup - removes every user the audit created (all *@audit.test
# users, plus the fixed alice@t.test from AUDIT.md's early blocks) and,
# through ON DELETE CASCADE, everything they made: posts, comments, follows,
# groups (and the posts/events inside them), messages, notifications and
# sessions. Also deletes their uploaded images from backend/uploads and the
# helpers' cookie jars. Demo and seed users are never touched.
audit_cleanup() {
  local match="email LIKE '%@audit.test' OR email = 'alice@t.test'" files f n=0
  local users=$(sqlite3 $AUDIT_DB "SELECT count(*) FROM users WHERE $match")
  if (( users == 0 )); then
    echo "audit_cleanup: no audit users in the database - nothing to do"
    return 0
  fi
  # Collect image paths before the rows that point to them are gone.
  files=(${(f)"$(sqlite3 $AUDIT_DB "
    WITH u AS (SELECT id FROM users WHERE $match),
         g AS (SELECT id FROM groups WHERE creator_id IN u),
         p AS (SELECT id FROM posts WHERE user_id IN u OR group_id IN g)
    SELECT avatar_path FROM users WHERE id IN u
    UNION SELECT image_path FROM posts WHERE id IN p
    UNION SELECT image_path FROM comments WHERE user_id IN u OR post_id IN p")"})
  # foreign_keys=ON is needed: the sqlite3 CLI ignores ON DELETE CASCADE
  # without it, and would leave orphaned posts/messages behind.
  sqlite3 $AUDIT_DB "PRAGMA foreign_keys=ON; DELETE FROM users WHERE $match; VACUUM;" || return 1
  for f in $files; do
    # Only our own uploads ("/uploads/<uuid>.<ext>"); ${f:t} keeps just the
    # file name, so a stored path can never point outside backend/uploads.
    [[ $f == /uploads/* && -f backend/uploads/${f:t} ]] && rm -f backend/uploads/${f:t} && (( n++ ))
  done
  rm -f $AUDIT_JARS/*.jar alice.jar
  echo "audit_cleanup: removed $users audit users (and everything they created) and $n uploaded images"
}

curl -s -m 2 $AUDIT_API/health >/dev/null && echo "audit.zsh loaded - backend is up" || echo "audit.zsh loaded - BACKEND IS NOT RUNNING (cd backend && go run server.go)"
