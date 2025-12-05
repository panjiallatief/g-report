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

### Method 2: Docker

1. **Build and Run**
   ```bash
   docker-compose up --build
   ```

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
