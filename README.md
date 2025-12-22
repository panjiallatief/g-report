# IT Broadcast Operations & Helpdesk System (G-Report)

G-Report is a comprehensive IT Operations and Helpdesk management system designed to streamline communication between consumers (employees) and IT staff/managers. It features ticket reporting, shift management, knowledge base (Big Book Wiki), and real-time performance tracking.

## 🚀 Features

### for Consumers (Employees)
- **Ticket Reporting**: specific issue categories (Network, Hardware, Software).
- **Rich Forms**: Support for photo uploads, urgency description, and location tagging.
- **Real-time Status**: Track ticket progress (Pending, Handled, Resolved).
- **Knowledge Base**: Access "Big Book" articles for self-service troubleshooting.

### for IT Staff
- **PWA Interface**: Mobile-first design for technicians on the move.
- **Shift Handover**: Structured workflow to pass active issues to the next shift.
- **Routine Checks**: Digital checklist for daily system health checks.
- **Chat & Timeline**: Communicate directly on tickets and view history.

### for Managers
- **Operational Dashboard**: Real-time KPIs (MTTA, MTTR, FCR) and active shift visibility.
- **Shift Management**: Schedule shifts, drag-and-drop interface, CSV import/export.
- **Staff Performance**: Track individual technician performance metrics.
- **Big Book Approval**: Review and approve knowledge base articles submitted by staff.

## 🛠 Tech Stack

- **Backend**: Go (Golang) with Gin Framework.
- **Database**: PostgreSQL with GORM (ORM).
- **Frontend**: Go Templates (HTML/CSS), Tailwind CSS, Alpine.js, HTMX.
- **Authentication**: Cookie-based session capability.
- **Deployment**: Docker/Docker Compose ready.

## 📦 Installation & Setup

### Prerequisites
- Go 1.21+
- PostgreSQL
- Docker (optional)

### Method 1: Local Development

1. **Clone the repository**
   ```bash
   git clone https://github.com/panjiallatief/g-report.git
   cd g-report
   ```

2. **Setup Environment**
   Duplicate `.env.example` (if available) or create `.env`:
   ```env
   DB_HOST=localhost
   DB_USER=postgres
   DB_PASSWORD=yourpassword
   DB_NAME=it_broadcast_db
   DB_PORT=5432
   PORT=8080
   SESSION_SECRET=your_secret_key
   ```

3. **Install Dependencies**
   ```bash
   go mod download
   ```

4. **Run the Application**
   ```bash
   go run main.go
   ```
   The application will start at `http://localhost:8080`.

### Method 2: Docker (Recommended for Production)

#### Option A: Pull from Docker Hub
```bash
# Pull the latest image
docker pull panjiallatief/goreport:latest

# Run with docker-compose
docker-compose up -d
```

#### Option B: Build Locally
```bash
# Build and run all services
docker-compose up --build -d
```

#### Docker Environment Variables
Create a `.env` file in the project root:
```env
# Database
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=it_broadcast_db
DB_PORT=5432

# Application
PORT=8080
SESSION_SECRET=your_secret_key

# LDAP (Optional)
LDAP_ENABLED=false
LDAP_SERVER=ldap.example.com
LDAP_PORT=389
LDAP_BASE_DN=dc=example,dc=com
LDAP_BIND_DN=cn=admin,dc=example,dc=com
LDAP_BIND_PASSWORD=admin_password

# Push Notifications (Optional)
VAPID_PUBLIC_KEY=your_vapid_public_key
VAPID_PRIVATE_KEY=your_vapid_private_key
```

#### Docker Commands
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f app

# Stop all services
docker-compose down

# Rebuild and restart
docker-compose up --build -d
```

#### Docker Hub Image
- **Repository**: `panjiallatief/goreport`
- **Tag**: `latest`
- **Pull**: `docker pull panjiallatief/goreport:latest`


## 📂 Project Structure

```
├── internal/
│   ├── auth/           # Authentication logic
│   ├── database/       # DB connection and migration
│   ├── models/         # GORM structs and SQL schemas
│   ├── modules/        # Feature modules (Consumer, Manager, Staff)
│   ├── server/         # Router and Template setup
│   └── utils/          # Helper functions (Seeding, etc.)
├── web/
│   ├── static/         # Assets (CSS, JS, Images)
│   ├── templates/      # HTML Templates
│   └── uploads/        # User uploaded files (Avatars, Evidence)
├── main.go             # Entry point
└── docker-compose.yml  # Docker config
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License.

## 📚 API Documentation

This project uses **Swagger** for interactive API documentation.

### Access Swagger UI
After running the application, access the API documentation at:
```
http://localhost:8080/swagger/index.html
```

### Available Endpoints
The API is organized into the following groups:
- **Auth** - Login, logout endpoints
- **Public** - Anonymous ticket submission (no auth required)
- **Consumer** - Employee dashboard, ticket creation, knowledge base
- **Staff** - IT staff dashboard, ticket management, routines
- **Manager** - Management dashboard, KPIs, shift scheduling, article approval
- **Notifications** - Push notification subscription management

### Regenerate Docs
If you modify API annotations, regenerate the docs:
```bash
swag init
```

## 🧪 Testing Strategy & Report

We use a modified **Testing Pyramid** strategy to ensure high quality and reliability.

### Strategy
1.  **Unit Tests (Base)**: Test pure logic (e.g., Utils, Crypto).
2.  **Integration Tests (Core)**: Test Handlers, Database, and Middleware interactions using a dedicated test database environment.
3.  **E2E Tests**: Manual or automated browser tests (Playwright/Cypress).

### Current Test Status

| Component | Type | Status | Notes |
| :--- | :--- | :--- | :--- |
| **Crypto Utils** | Unit | ✅ PASS | Password hashing/validation secure. |
| **Auth Module** | Integration | ⚠️ CHECK | Login flow verification. |
| **Consumer Module** | Integration | ⚠️ CHECK | Dashboard & Ticket flow verification. |

### How to Run Tests
```bash
# Run all tests
go test ./internal/... -v
```

> [!NOTE]
> Tests require a local PostgreSQL instance. The test suite automatically manages the `it_broadcast_test` database.

