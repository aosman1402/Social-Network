package main

import (
    "log"
    "net/http"
    "os"
    "path/filepath"

    "social-network3/pkg/db/sqlite"
    "social-network3/pkg/handlers"
    "social-network3/pkg/middleware"
    "social-network3/pkg/websocket"
)

func main() {
    // Get current working directory
    cwd, err := os.Getwd()
    if err != nil {
        log.Fatalf("Failed to get current directory: %v", err)
    }

    // Database configuration
    dbPath := os.Getenv("DB_PATH")
    if dbPath == "" {
        dbPath = filepath.Join(cwd, "pkg", "db", "sqlite", "social-network.db")
    }

    migrationsPath := os.Getenv("MIGRATIONS_PATH")
    if migrationsPath == "" {
        migrationsPath = filepath.Join(cwd, "pkg", "db", "migrations", "sqlite")
    }

    log.Printf("Current directory: %s", cwd)
    log.Printf("Database path: %s", dbPath)
    log.Printf("Migrations path: %s", migrationsPath)

    // Initialize database
    db, err := sqlite.NewDB(sqlite.Config{
        DBPath:          dbPath,
        MigrationsPath:  migrationsPath,
        EnableForeignKeys: true,
    })
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    // Apply migrations
    if err := db.ApplyMigrations(migrationsPath); err != nil {
        log.Fatalf("Failed to apply migrations: %v", err)
    }

    // Initialize WebSocket hub
    hub := websocket.NewHub()
    go hub.Run()

    // Initialize handlers
    authHandler := &handlers.AuthHandler{DB: db.DB}
    userHandler := &handlers.UserHandler{DB: db.DB}
    postHandler := &handlers.PostHandler{DB: db.DB}
    groupHandler := &handlers.GroupHandler{DB: db.DB}
    wsHandler := &handlers.WebSocketHandler{
        DB:  db.DB,
        Hub: hub,
    }

    // Initialize middleware
    authMiddleware := &middleware.AuthMiddleware{DB: db.DB}

    // Serve static files (uploads)
    http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

    // CORS middleware
    cors := func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // ✅ Allow multiple origins
            origin := r.Header.Get("Origin")
            allowedOrigins := map[string]bool{
                "http://localhost:3000": true,
                "http://localhost:3001": true,
                "http://127.0.0.1:3000": true,
                "http://127.0.0.1:3001": true,
            }
            
            if allowedOrigins[origin] {
                w.Header().Set("Access-Control-Allow-Origin", origin)
            } else {
                w.Header().Set("Access-Control-Allow-Origin", "*")
            }
            
            w.Header().Set("Access-Control-Allow-Credentials", "true")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Cookie, Set-Cookie")
            w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")

            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }

            next(w, r)
        }
    }

    // Health check
    http.HandleFunc("/api/health", cors(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"ok"}`))
    }))

    // Auth routes
    http.HandleFunc("/api/register", cors(authHandler.Register))
    http.HandleFunc("/api/login", cors(authHandler.Login))
    http.HandleFunc("/api/logout", cors(authMiddleware.Authenticate(authHandler.Logout)))
    http.HandleFunc("/api/check-auth", cors(authMiddleware.Authenticate(authHandler.CheckAuth)))

    // User routes
    http.HandleFunc("/api/user/profile", cors(authMiddleware.Authenticate(userHandler.GetProfile)))
    http.HandleFunc("/api/user/update", cors(authMiddleware.Authenticate(userHandler.UpdateProfile)))
    http.HandleFunc("/api/user/follow", cors(authMiddleware.Authenticate(userHandler.FollowUser)))
    http.HandleFunc("/api/user/unfollow", cors(authMiddleware.Authenticate(userHandler.UnfollowUser)))
    http.HandleFunc("/api/user/follow-request", cors(authMiddleware.Authenticate(userHandler.HandleFollowRequest)))
    http.HandleFunc("/api/user/follow-requests", cors(authMiddleware.Authenticate(userHandler.GetFollowRequests)))
    http.HandleFunc("/api/user/followers", cors(authMiddleware.Authenticate(userHandler.GetFollowers)))
    http.HandleFunc("/api/user/following", cors(authMiddleware.Authenticate(userHandler.GetFollowing)))
    http.HandleFunc("/api/user/posts", cors(authMiddleware.Authenticate(userHandler.GetUserPosts)))

    // ✅ POST ROUTES - ALL OF THEM
    http.HandleFunc("/api/posts", cors(authMiddleware.Authenticate(postHandler.GetPosts)))
    http.HandleFunc("/api/post/create", cors(authMiddleware.Authenticate(postHandler.CreatePost)))
    http.HandleFunc("/api/post/update", cors(authMiddleware.Authenticate(postHandler.UpdatePost)))
    http.HandleFunc("/api/post/delete", cors(authMiddleware.Authenticate(postHandler.DeletePost)))
    http.HandleFunc("/api/post/like", cors(authMiddleware.Authenticate(postHandler.LikePost)))        // ✅ THIS IS THE ONE
    http.HandleFunc("/api/post/save", cors(authMiddleware.Authenticate(postHandler.SavePost)))
    http.HandleFunc("/api/post/comments", cors(authMiddleware.Authenticate(postHandler.GetComments)))
    http.HandleFunc("/api/post/comment", cors(authMiddleware.Authenticate(postHandler.CreateComment)))

    // Group routes
    http.HandleFunc("/api/groups", cors(authMiddleware.Authenticate(groupHandler.GetAllGroups)))
    http.HandleFunc("/api/groups/my", cors(authMiddleware.Authenticate(groupHandler.GetMyGroups)))
    http.HandleFunc("/api/group/create", cors(authMiddleware.Authenticate(groupHandler.CreateGroup)))
    http.HandleFunc("/api/group/detail", cors(authMiddleware.Authenticate(groupHandler.GetGroup)))
    http.HandleFunc("/api/group/invite", cors(authMiddleware.Authenticate(groupHandler.InviteUserToGroup)))
    http.HandleFunc("/api/group/join", cors(authMiddleware.Authenticate(groupHandler.RequestToJoinGroup)))
    http.HandleFunc("/api/group/leave", cors(authMiddleware.Authenticate(groupHandler.LeaveGroup)))
    http.HandleFunc("/api/group/request", cors(authMiddleware.Authenticate(groupHandler.HandleGroupRequest)))
    http.HandleFunc("/api/group/invitation", cors(authMiddleware.Authenticate(groupHandler.HandleGroupInvitation)))
    http.HandleFunc("/api/group/events", cors(authMiddleware.Authenticate(groupHandler.GetEvents)))
    http.HandleFunc("/api/group/event/create", cors(authMiddleware.Authenticate(groupHandler.CreateEvent)))
    http.HandleFunc("/api/group/event/respond", cors(authMiddleware.Authenticate(groupHandler.RespondToEvent)))
    http.HandleFunc("/api/group/posts", cors(authMiddleware.Authenticate(groupHandler.GetGroupPosts)))
    http.HandleFunc("/api/group/messages/send", cors(authMiddleware.Authenticate(wsHandler.SendGroupMessage)))
    http.HandleFunc("/api/group/messages", cors(authMiddleware.Authenticate(wsHandler.GetGroupMessages)))

    // Chat routes
    http.HandleFunc("/api/chats", cors(authMiddleware.Authenticate(wsHandler.GetChats)))
    http.HandleFunc("/api/messages/history", cors(authMiddleware.Authenticate(wsHandler.GetChatHistory)))
    http.HandleFunc("/api/messages/send", cors(authMiddleware.Authenticate(wsHandler.SendMessage)))

    // Notification routes
    http.HandleFunc("/api/notifications", cors(authMiddleware.Authenticate(wsHandler.GetNotifications)))
    http.HandleFunc("/api/notifications/read", cors(authMiddleware.Authenticate(wsHandler.MarkNotificationAsRead)))
    http.HandleFunc("/api/notifications/read-all", cors(authMiddleware.Authenticate(wsHandler.MarkAllNotificationsAsRead)))
    http.HandleFunc("/api/notifications/unread", cors(authMiddleware.Authenticate(wsHandler.GetUnreadCount)))

    // WebSocket route
    http.HandleFunc("/ws", cors(wsHandler.HandleWebSocket))

    // Start server
    port := os.Getenv("PORT")
    if port == "" {
        port = "8081"
    }

    log.Printf("Server starting on :%s", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}