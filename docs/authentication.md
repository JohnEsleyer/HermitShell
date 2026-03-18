# Authentication Implementation

> See also: [Security Measures](./security-measures.md)

## Overview

HermitShell uses cookie-based session authentication for the dashboard. Users authenticate with username/password, and the server issues a secure HTTP-only cookie containing the user ID.

## High-Level Flow

```
User Login
    │
    ▼
┌─────────────────────┐
│  Login Form         │
│  (App.tsx)          │
└─────────┬───────────┘
          │ POST /api/auth/login
          ▼
┌─────────────────────┐
│  HandleLogin        │ ◄── Verify credentials against DB
│  (server.go)       │
└─────────┬───────────┘
          │
    ┌─────┴─────┐
    │ Success?  │
    └─────┬─────┘
      Yes │ No
          ▼    │
┌─────────────────────┐    │
│  Set HTTP-only     │    │
│  Cookie (session)  │    │
└─────────┬───────────┘    │
          │               ▼
          ▼        ┌─────────────────────┐
┌─────────────────────┐  │ Return error     │
│  Authenticated     │  └─────────────────────┘
│  State in frontend │
└─────────────────────┘
```

## Code Flow

### 1. Login Request (Frontend)
**File: `dashboard/src/App.tsx`**
```typescript
const handleLogin = async (username: string, password: string) => {
  const res = await fetch(`${API_BASE}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  const data = await res.json();
  if (data.success) {
    setIsAuthenticated(true);
    fetchAgents();
  }
};
```

### 2. Login Handler (Backend)
**File: `internal/api/server.go`**
```go
func (s *Server) HandleLogin(c *fiber.Ctx) error {
    // Parse request body
    var req struct{ Username, Password string }
    c.BodyParser(&req)

    // Verify credentials against database
    id, mustChange, err := s.db.VerifyUser(req.Username, req.Password)
    if err != nil || id == 0 {
        return c.JSON(fiber.Map{"success": false, "error": "Invalid credentials"})
    }

    // Set HTTP-only cookie with session
    c.Cookie(&fiber.Cookie{
        Name:     "session",
        Value:    fmt.Sprintf("%d", id),
        Path:     "/",
        HTTPOnly: true,  // Prevents JavaScript access
    })

    return c.JSON(fiber.Map{"success": true, "mustChangePassword": mustChange})
}
```

### 3. Credential Verification (Database)
**File: `internal/db/db.go`**
```go
func (d *DB) VerifyUser(username, password string) (int64, bool, error) {
    var id int64
    var hash string
    var mustChange int
    
    // Get stored hash for username
    err := d.db.QueryRow(
        "SELECT id, password_hash, must_change_password FROM users WHERE username = ?", 
        username,
    ).Scan(&id, &hash, &mustChange)
    
    // Compare SHA256 hash
    if hash != hashPassword(password) {
        return 0, false, nil  // Invalid credentials
    }
    return id, mustChange == 1, nil
}

func hashPassword(password string) string {
    hash := sha256.Sum256([]byte(password))
    return hex.EncodeToString(hash[:])
}
```

### 4. Auth Check (Middleware Pattern)
**File: `internal/api/server.go`**
```go
func (s *Server) HandleCheckAuth(c *fiber.Ctx) error {
    session := c.Cookies("session")
    if session == "" {
        return c.JSON(fiber.Map{"authenticated": false})
    }

    id, _ := strconv.ParseInt(session, 10, 64)
    username, mustChange, err := s.db.GetUserByID(id)
    if err != nil || username == "" {
        return c.JSON(fiber.Map{"authenticated": false})
    }
    return c.JSON(fiber.Map{
        "authenticated": true, 
        "username": username, 
        "mustChangePassword": mustChange,
    })
}
```

## Cheatsheet

| Operation | File | Function |
|-----------|------|----------|
| Login | `server.go:408` | `HandleLogin` |
| Logout | `server.go:429` | `HandleLogout` |
| Check Auth | `server.go:434` | `HandleCheckAuth` |
| Change Password | `server.go:448` | `HandleChangeCredentials` |
| Verify User | `db.go:307` | `VerifyUser` |
| Hash Password | `db.go:345` | `hashPassword` |
| Get User | `db.go:350` | `GetUserByID` |

## Key Implementation Details

1. **Password Storage**: SHA256 hash (not salted - consider adding salt for production)
2. **Session**: HTTP-only cookie with user ID
3. **Default User**: Created on first run with `admin` / `hermit123`
4. **Must Change Password**: Users forced to change password on first login
5. **Inline Login Errors**: The dashboard login form displays invalid credential and connectivity failures directly in the UI

## Database Schema

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'admin',
    must_change_password INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

## Related Files

- Frontend Login: `dashboard/src/App.tsx` (LoginScreen component)
- Auth API: `internal/api/server.go` (lines 408-460)
- User DB: `internal/db/db.go` (lines 289-368)
