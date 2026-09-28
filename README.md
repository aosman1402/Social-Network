# Social Network

A Facebook-like social network, built with a Go backend and a JavaScript-framework frontend, backed by SQLite. It covers followers, profiles, posts, groups (with events), real-time chat, and notifications.

## Objectives

The project's mandatory features are:

- Followers
- Profile
- Posts
- Groups
- Notification
- Chats

## Architecture

### Frontend

The client-side of the app: everything the user sees and interacts with directly (text, images, buttons, navigation), plus the requests it makes to the backend to fetch or store data. Built with HTML, CSS, and JavaScript.

A JS **framework** is required (as opposed to a JS library) to lay out the project's structure. Options include:

- **Next.js** ← chosen for this project
- Vue.js
- Svelte
- Mithril

### Backend

Split into three parts:

- **Server** — the entry point that receives incoming requests.
- **App** — the application logic that listens for requests, queries the database, and sends responses. Contains middleware such as:
  - **Authentication** — since HTTP is stateless, sessions and cookies are used to keep a client authenticated.
  - **Image handling** — supports at least JPEG, PNG, and GIF; uploaded files are stored on the filesystem, with their path saved in the database.
  - **WebSocket** — handles real-time connections between clients, used for private/group chat.
- **Database** — SQLite, queried by the app to read/write data submitted by clients.

### Database & Migrations

Data is stored in **SQLite**. The schema is version-controlled through migrations, applied automatically every time the app runs, using a folder structure like:

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

`sqlite.go` handles the database connection and applies the migrations (e.g. via `golang-migrate`) from a path such as `file://backend/pkg/db/migrations/sqlite`.

### Docker

The project runs as two separate Docker images, each with its own `Dockerfile`:

- **Backend container** (`backend/Dockerfile`) — a multi-stage Go build; runs the server-side logic and exposes port `8081` for the frontend and external clients to reach it. The SQLite database (`/app/pkg/db/sqlite`) and uploaded files (`/app/uploads`) are mounted as volumes so they survive container recreation.
- **Frontend container** (`frontend/Dockerfile`) — a multi-stage Next.js build (`output: "standalone"`); serves the client-side HTML/CSS/JS and exposes port `3000` to users' browsers, communicating with the backend over HTTP.

`docker-compose.yml` at the repo root builds and wires both together. See [Run it with Docker](#run-it-with-docker) below.

## Feature Details

### Authentication

Registration requires: **Email**, **Password**, **First Name**, **Last Name**, and **Date of Birth**. **Avatar/Image**, **Nickname**, and **About Me** are shown on the form but optional. Once logged in, a user stays logged in (via session + cookie) until they explicitly log out, an option that stays available at all times.

The optional **Avatar/Image** offers two ways to set a picture at registration:
- **Choose an avatar** — pick from a set of preset avatars generated via the [DiceBear](https://www.dicebear.com/styles/clay/) "clay" style.
- **Upload a photo** — JPG, PNG, or GIF only. The image is resized and recompressed in the browser before upload, so it doesn't take up much space once stored; the server independently verifies the real file type by sniffing its bytes rather than trusting the extension.

Either way, an avatar can also be skipped entirely and added or changed later from Settings.

**Password policy**: 12 to 15 characters, containing at least one of each of the following, entered via a live checklist that ticks each requirement off as the user types (submission is blocked until every requirement is met):
- A lowercase letter (`a`-`z`)
- An uppercase letter (`A`-`Z`)
- A digit (`0`-`9`)
- A special character — the space character or one of `! " # $ % & ' ( ) * + , - . / : ; < = > ? @ [ \ ] ^ _ \` { | } ~`

No other characters are permitted. The same rule applies to setting a new password from Settings, and is enforced by the backend independently of the frontend check.

**Nickname policy**: if provided, must be 3 to 14 characters made up only of:
- Lowercase letters (`a`-`z`)
- Uppercase letters (`A`-`Z`)
- Digits (`0`-`9`)

Nicknames must also be unique. As the user types, a live check (debounced, hitting the server) marks the field with a ✓ once the nickname is valid and available, or a ✕ if it's badly formatted or already taken - submission is blocked until it resolves to available. Enforced by the backend as well, so a nickname can't be registered out from under an in-flight check.

**Session & cookie security**:
- A session stays valid for up to **30 days**. The cookie carrying it (`session_token`) is `HttpOnly` (unreachable from JS, so it can't be stolen via XSS), and `SameSite=Lax` (blocks CSRF). It's also marked `Secure` whenever the connection is actually HTTPS - conditional rather than hardcoded, so it still works over plain HTTP in local development, and engages automatically once deployed behind HTTPS.
- **IP address changes** are logged for visibility but don't invalidate the session - mobile networks and Wi-Fi switching change IPs often and legitimately.
- **Device fingerprint mismatch** (via [FingerprintJS](https://github.com/fingerprintjs/fingerprintjs)) invalidates the session immediately - a stronger signal the cookie is being used from a different device than the one that logged in.
- **Brute-force protection**: repeated failed login attempts against one account (even spread across different IPs) invalidate any session that account currently has, forcing re-login everywhere.
- **Changing your password** invalidates every session on the account, including on other devices - not just the one making the change.

**Logging out**:
- The session record is deleted from the database - the session ID becomes useless immediately, not just expired.
- The server clears the cookie (same name, blank value, expiration set in the past).
- The frontend clears `localStorage` and `sessionStorage` entirely.

### Followers

Users can follow/unfollow each other. Following a user with a **private profile** requires sending a follow request, which they can accept or decline. Following a user with a **public profile** happens instantly — no request/accept step needed. Unfollowing requires already following that user.

### Profile

Every profile shows:
- The user's registration info (excluding password)
- Their activity — every post they've made
- Their followers and who they're following

A profile is either **public** (visible to everyone) or **private** (visible only to followers). The owner can toggle this on their own profile.

### Posts

Logged-in users can create posts and comment on existing posts, optionally attaching an image or GIF. Each post's privacy is one of:
- **Public** — visible to all users
- **Almost private** — visible only to the creator's followers
- **Private** — visible only to a specific list of followers chosen by the creator

### Groups

Any user can create a group with a title and description, then invite others (who must accept) to join. Members can invite further users, or a user can request to join a group they found by browsing all groups — only the group's creator can accept/refuse those requests.

- **Invitations** can only be sent to users you have a follow relationship with (either direction). The invited user gets a notification and answers with the **Accept / Decline** buttons on the group page; the inviter is notified of the answer.
- **Join requests** show up on the group page for the creator to accept or decline; the requester is notified either way.
- A declined invitation or request doesn't lock anyone out — the user can be invited again or ask to join again later.

Within a group, members can create posts and comments visible only to the group, and create **events** with:
- Title
- Description
- Day/Time
- At least 2 response options: **Going** / **Not going**

Every member can pick a response option after an event is created.

### Chat

Private messaging is allowed only between users where at least one follows the other (an accepted follow — a pending request doesn't count, and a public profile alone isn't enough). This is enforced by the backend on every path: sending over HTTP or WebSocket, loading chat history, and typing indicators. After an unfollow, the conversation disappears from the chat list.

- Messages are delivered instantly via **WebSockets**, with an HTTP fallback if the socket isn't connected.
- New messages show a pop-up and an unread count on the navbar's **Chat** link, which clears as the conversation is read (in every open tab).
- Emojis can be sent like any other text (typed with the OS emoji keyboard or pasted) and are stored and delivered unchanged.
- Group members can also exchange group chat messages over the same WebSocket connection; they're delivered only to the group's active members.

### Notifications

Shown through the bell in the navbar, which is on every page, and kept separate from new private messages (those use the **Chat** unread count instead). Notifications arrive live over the WebSocket, and the bell's unread count stays in sync with the Notifications page and across tabs; anything missed while offline is reloaded on reconnect. A user is notified when:
- They have a private profile and someone sends a follow request
- They receive a group invitation (to accept/refuse)
- They created a group and someone requests to join it (to accept/refuse)
- They're a member of a group in which a new event is created

Additional notifications: a follow request accepted or declined, a new follower or an unfollow, a group invitation accepted or declined (sent to the inviter), a join request accepted or declined (sent to the requester), likes and comments on your posts, and a confirmation when you switch your own profile between public and private.

## Allowed Packages

- Standard Go packages
- Gorilla WebSocket
- golang-migrate
- sql-migration
- migration
- sqlite3
- bcrypt
- gofrs/uuid or google/uuid

## What This Project Covers

- Authentication (sessions and cookies)
- Docker (setup, containerizing an application, image creation, compatibility/dependencies)
- SQL (database manipulation, migrations)
- The basics of encryption
- WebSockets

## Tech Stack

- **Backend**: Go
- **Database**: SQLite, with versioned migrations
- **Frontend framework options**: Next.js ← chosen, Vue.js, Svelte, Mithril
- **Real-time**: WebSockets
- **Languages**: Go, JavaScript, HTML, CSS, SQL

## How to Run it

### Prerequisites
- Go (backend)
- Node.js + npm (frontend)

No `.env` files or extra configuration are required — the backend uses an embedded SQLite database and the frontend is pre-configured to talk to it.

### 1. Start the backend

```bash
cd backend
go run server.go
```

- Runs on `http://localhost:8081` by default (override with the `PORT` env var).
- On startup it opens `backend/pkg/db/sqlite/social-network.db` and applies any pending migrations from `backend/pkg/db/migrations/sqlite` automatically.
- Override the DB/migrations locations with the `DB_PATH` / `MIGRATIONS_PATH` env vars if needed.

### 2. Start the frontend

In a separate terminal:

```bash
cd frontend
npm install
npm run dev
```

- Runs on `http://localhost:3000`.
- `next.config.js` already proxies `/api/*` requests to `http://localhost:8081`, so the backend must be running first. The live WebSocket connection goes straight to `ws://localhost:8081/ws`.

### 3. Use the app

Open `http://localhost:3000` in your browser. You'll be redirected to `/login` — use "Register" to create an account.

To try the real-time features (chat, notifications) with two users at once, use two different browsers, or one normal and one private window, since a browser shares its login cookie across tabs.

### Run the tests

From the repo root:

```bash
(cd backend && go test ./...)
(cd frontend && npm test)
```

## Demo data

A new database is filled with demo data by migration `000022_seed_demo_data` (removed again by its `.down.sql`). Log in as any of these, all with the password **`Demo-Pass-2026`**:

| Email | Profile | Try |
|---|---|---|
| `alice@demo.test` | public | creator of "Weekend Hikers" - accept/decline Eve's join request |
| `bob@demo.test` | public | sees Alice's private post; chat history with Alice |
| `carol@demo.test` | **private** | a pending follow request (Eve) and a group invitation |
| `dave@demo.test` | public | member of "Weekend Hikers", voted on the upcoming hike |
| `eve@demo.test` | public | waiting for her join request / follow request |

## Login with GitHub (optional)

"Continue with GitHub" on the Login and Register pages uses GitHub OAuth 2.0 (standard library only, see `backend/pkg/handlers/oauth_github.go`). It needs a GitHub OAuth App - a one-time, 2-minute setup:

1. Go to https://github.com/settings/developers → **OAuth Apps** → **New OAuth App**:
   - Homepage URL: `http://localhost:3000`
   - Authorization callback URL: `http://localhost:8081/api/auth/github/callback`
2. Copy the **Client ID** and generate a **Client secret**.
3. Put them in `backend/.env` (running without Docker) or `.env` in the repo root (Docker) - both are gitignored; see the `.env.example` files:
   ```
   GITHUB_CLIENT_ID=...
   GITHUB_CLIENT_SECRET=...
   ```
4. Restart the backend (or `docker-compose up -d --build`). The log says `GitHub login enabled`.

How it works: a returning GitHub user is logged straight in; if a local account already has the same **verified** GitHub email, it's linked to it; a new user fills in the one required field GitHub doesn't provide (date of birth), plus optional nickname / about me, then lands logged in. Without the keys the button shows as unavailable, with a short explanation and these setup steps underneath it, and everything else works as before.

## Run it with Docker

An alternative to the manual steps above — no local Go or Node install needed, just Docker.

### Prerequisites
- Docker (with Compose — `docker compose`/`docker-compose`)

### Option A: build script (recommended)

From the repo root:

```bash
./build.sh
```

It checks Docker is running, builds both images, starts the containers in the background, lists them with their sizes and prints the URLs. It runs the same thing as:

```bash
docker-compose up --build
```

This builds and starts both containers in the right order — Compose starts `backend` first (`depends_on`), then `frontend` — and:
- Backend → `http://localhost:8081`, with its SQLite DB and `uploads/` persisted in named volumes (`backend-db`, `backend-uploads`) so they survive `docker-compose down` / rebuilds.
- Frontend → `http://localhost:3000`, built with `BACKEND_INTERNAL_URL=http://backend:8081` so its server-side `/api/*` proxy (`next.config.js`) can reach the backend container by service name.

Open `http://localhost:3000` as in step 3 above. Stop everything with `docker-compose down` (add `-v` to also delete the DB/uploads volumes).

### Option B: plain `docker build` / `docker run`

If you're not using Compose, the two images must still be built and started **backend first, frontend second** — the frontend's build step needs to know where the backend will be reachable from. A shared user-defined network lets the frontend address the backend by container name, the same way Compose's default network does:

```bash
docker network create social-network

# 1. Backend — build and run first
docker build -t social-network-backend ./backend
docker run -d --name social-network-backend \
  --network social-network \
  -p 8081:8081 \
  -v social-network-db:/app/pkg/db/sqlite \
  -v social-network-uploads:/app/uploads \
  social-network-backend

# 2. Frontend — build second, pointing its server-side proxy at the
#    backend container (BACKEND_INTERNAL_URL is baked in at build time
#    via next.config.js's rewrites(), not read at container startup)
docker build -t social-network-frontend ./frontend \
  --build-arg BACKEND_INTERNAL_URL=http://social-network-backend:8081
docker run -d --name social-network-frontend \
  --network social-network \
  -p 3000:3000 \
  social-network-frontend
```

This is more steps for the same result — Option A (docker-compose) is the recommended path.
