# JWT Authentication API Usage

The Expenser application now includes a complete JWT authentication backend. The frontend remains unchanged, but the JWT API is available for testing and future integration.

## Available API Endpoints

### Public Endpoints (No Authentication Required)

#### Register User
```bash
POST /api/register
Content-Type: application/json

{
  "username": "testuser",
  "email": "test@example.com",
  "password": "password123",
  "confirm_password": "password123"
}
```

**Response:**
```json
{
  "message": "User registered successfully",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "created_at": "2025-07-16T17:44:29Z"
  }
}
```

#### Login User
```bash
POST /api/login
Content-Type: application/json

{
  "username": "testuser",
  "password": "password123"
}
```

**Response:**
```json
{
  "message": "Login successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "created_at": "2025-07-16T17:44:29Z"
  }
}
```

### Protected Endpoints (Authentication Required)

For protected endpoints, include the JWT token in the Authorization header:

```bash
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

#### Get User Profile
```bash
GET /api/profile
Authorization: Bearer <your-jwt-token>
```

**Response:**
```json
{
  "user": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com"
  }
}
```

#### Protected Example Endpoint
```bash
GET /api/protected
Authorization: Bearer <your-jwt-token>
```

**Response:**
```json
{
  "message": "This is a protected endpoint",
  "user_id": 1,
  "username": "testuser",
  "timestamp": 1642345678
}
```

## Testing with curl

### 1. Register a new user
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123",
    "confirm_password": "password123"
  }'
```

### 2. Login and get token
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "password123"
  }'
```

### 3. Use token to access protected endpoint
```bash
curl -X GET http://localhost:8080/api/profile \
  -H "Authorization: Bearer YOUR_JWT_TOKEN_HERE"
```

## JWT Configuration

The JWT authentication is configured via environment variables:

- `JWT_SECRET`: Secret key for signing tokens (default: development key)
- `JWT_EXPIRATION_HOURS`: Token expiration time in hours (default: 24)

## Database Schema

The authentication system adds a `users` table with the following structure:

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

## Security Features

- **Password Hashing**: Uses bcrypt with proper salt rounds
- **JWT Tokens**: Secure token generation with configurable expiration
- **Input Validation**: Comprehensive validation for all user inputs
- **Unique Constraints**: Prevents duplicate usernames and emails
- **HTTP-Only Cookies**: Support for secure cookie-based authentication (for future frontend integration)

## Frontend Integration

The existing frontend remains fully functional. When ready to add authentication to the frontend:

1. Create login/register forms
2. Use the existing API endpoints
3. Store JWT tokens securely
4. Add authentication middleware to protected routes
5. Update navigation to show user status

The JWT backend is production-ready and can be integrated with any frontend framework or used as a standalone API.
