# API Endpoint Creation

## Overview

The Go backend uses Fiber framework to create RESTful API endpoints. Routes are defined in `setupRoutes` and handler functions implement the business logic.

## High-Level Flow

```
Server Initialization
    │
    ▼
┌─────────────────────┐
│  setupRoutes       │
│  (server.go)      │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Register Routes   │ ◄── api.Get(), api.Post(), etc.
│  with Fiber       │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Server.Listen     │
│  Ready for Requests│
└─────────────────────┘

Request Received
    │
    ▼
┌─────────────────────┐
│  Fiber Router      │
│  Matches Route    │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Handler Function  │ ◄── HandleXxx(c *fiber.Ctx)
│  Executes          │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Response JSON     │
│  to Client        │
└─────────────────────┘
```

## Code Flow

### 1. Setup Routes
**File: `internal/api/server.go`**
```go
func (s *Server) setupRoutes(app *fiber.App) {
    // Create API group with /api prefix
    api := app.Group("/api")

    // Auth routes
    api.Post("/auth/login", s.HandleLogin)
    api.Post("/auth/logout", s.HandleLogout)
    api.Get("/auth/check", s.HandleCheckAuth)
    api.Post("/auth/change-credentials", s.HandleChangeCredentials)

    // Agent routes
    api.Get("/agents", s.HandleListAgents)
    api.Post("/agents", s.HandleCreateAgent)
    api.Get("/agents/:id", s.HandleGetAgent)
    api.Put("/agents/:id", s.HandleUpdateAgent)
    api.Delete("/agents/:id", s.HandleDeleteAgent)

    // Calendar routes
    api.Get("/calendar", s.HandleListCalendar)
    api.Post("/calendar", s.HandleCreateCalendarEvent)
    api.Put("/calendar/:id", s.HandleUpdateCalendarEvent)
    api.Delete("/calendar/:id", s.HandleDeleteCalendarEvent)

    // ... more routes ...

    // Setup static file serving
    s.setupStaticRoutes(app)
}
```

### 2. Create Handler Function
**File: `internal/api/server.go`**
```go
// Handler function signature: func(c *fiber.Ctx) error
func (s *Server) HandleListAgents(c *fiber.Ctx) error {
    // 1. Get data from database
    agents, err := s.db.ListAgents()
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    // 2. Transform data if needed
    type AgentResponse struct {
        ID           int64  `json:"id"`
        Name         string `json:"name"`
        Status       string `json:"status"`
        ProfilePic   string `json:"profilePic"`
    }
    var result []AgentResponse
    for _, a := range agents {
        result = append(result, AgentResponse{
            ID:         a.ID,
            Name:       a.Name,
            Status:     a.Status,
            ProfilePic: a.ProfilePic,
        })
    }

    // 3. Return JSON response
    return c.JSON(result)
}
```

### 3. Handler with Request Body
**File: `internal/api/server.go`**
```go
func (s *Server) HandleCreateCalendarEvent(c *fiber.Ctx) error {
    // 1. Parse request body
    var req struct {
        AgentID int64  `json:"agentId"`
        Date    string `json:"date"`
        Time    string `json:"time"`
        Prompt  string `json:"prompt"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
    }

    // 2. Validate input
    if req.AgentID == 0 || req.Date == "" {
        return c.Status(400).JSON(fiber.Map{"error": "Missing required fields"})
    }

    // 3. Execute business logic
    event := &db.CalendarEvent{
        AgentID: req.AgentID,
        Date:    req.Date,
        Time:    req.Time,
        Prompt:  req.Prompt,
    }
    id, err := s.db.CreateCalendarEvent(event)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    // 4. Return success
    return c.JSON(fiber.Map{"id": id, "success": true})
}
```

### 4. Handler with URL Parameters
**File: `internal/api/server.go`**
```go
func (s *Server) HandleGetAgent(c *fiber.Ctx) error {
    // Parse ID from URL parameter
    id, err := strconv.ParseInt(c.Params("id"), 10, 64)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid ID"})
    }

    // Get from database
    agent, err := s.db.GetAgent(id)
    if err != nil || agent == nil {
        return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
    }

    return c.JSON(agent)
}
```

### 5. Handler with Query Parameters
**File: `internal/api/server.go`**
```go
func (s *Server) HandleGetLogs(c *fiber.Ctx) error {
    // Parse query parameters
    category := c.Query("category", "all")  // Default: "all"
    limit, _ := strconv.Atoi(c.Query("limit", "100"))  // Default: 100

    logs, err := s.db.GetAllAuditLogs(category, limit)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(logs)
}
```

## Cheatsheet

| HTTP Method | Fiber Function | Route Pattern |
|-------------|----------------|---------------|
| GET | `api.Get("/path", handler)` | List/Get |
| POST | `api.Post("/path", handler)` | Create |
| PUT | `api.Put("/path/:id", handler)` | Update |
| DELETE | `api.Delete("/path/:id", handler)` | Delete |

| Operation | Code |
|-----------|------|
| Parse body | `c.BodyParser(&struct)` |
| Get URL param | `c.Params("id")` |
| Get query param | `c.Query("name", "default")` |
| Get cookie | `c.Cookies("session")` |
| Set cookie | `c.Cookie(&fiber.Cookie{...})` |
| JSON response | `c.JSON(data)` |
| Error response | `c.Status(500).JSON(...)` |

## Adding a New Endpoint

1. **Add route** in `setupRoutes`:
```go
api.Get("/my-resource", s.HandleListMyResource)
api.Post("/my-resource", s.HandleCreateMyResource)
api.Delete("/my-resource/:id", s.HandleDeleteMyResource)
```

2. **Create handler**:
```go
func (s *Server) HandleListMyResource(c *fiber.Ctx) error {
    // Implementation
    return c.JSON(result)
}
```

3. **Add to database** if needed (see `db.go`):
```go
func (d *DB) ListMyResource() ([]*MyResource, error) {
    // SQL query
}
```

## Related Files

- Route Setup: `internal/api/server.go` (lines 222-286)
- Handler Examples: `internal/api/server.go` (various)
- Database Functions: `internal/db/db.go`
- Frontend API Calls: `dashboard/src/App.tsx`
