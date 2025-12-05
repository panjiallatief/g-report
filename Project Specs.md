IT Broadcast Ops & Helpdesk - Final Specification (v2.0)

Stack: Go (GIN/MUX), PostgreSQL, TailwindCSS, HTMX, AlpineJS (minimal).
Architecture: Monolith Modular (Go Templates + HTMX for dynamic interaction).

1. Final Database Schema (ERD)

Penambahan tabel untuk Knowledge Base (Big Book) dan update tabel Tickets untuk konteks siaran.

-- ENUMS & TYPES
CREATE TYPE user_role AS ENUM ('CONSUMER', 'STAFF', 'ADMIN');
CREATE TYPE task_status AS ENUM ('PENDING', 'IN_PROGRESS', 'HANDOVER', 'RESOLVED', 'CLOSED');
CREATE TYPE urgency_context AS ENUM ('NORMAL', 'PRE_PRODUCTION', 'ON_AIR_EMERGENCY');
CREATE TYPE ticket_category AS ENUM ('AUDIO', 'VIDEO', 'IT_NETWORK', 'SOFTWARE', 'ELECTRICAL');

-- 1. USERS & SHIFTS
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    role user_role DEFAULT 'CONSUMER',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    label VARCHAR(50) -- e.g., 'Pagi', 'Siang'
);

-- 2. BIG BOOK (KNOWLEDGE BASE) - NEW!
CREATE TABLE knowledge_articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL, -- Markdown / HTML support
    category ticket_category,
    
    author_id UUID REFERENCES users(id),
    is_verified BOOLEAN DEFAULT FALSE, -- Harus diapprove Admin
    views_count INT DEFAULT 0,
    helpful_count INT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 3. ROUTINE TASKS (Checklists)
CREATE TABLE routine_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(200) NOT NULL,
    cron_expression VARCHAR(50) NOT NULL, 
    deadline_minutes INT DEFAULT 30,
    created_by UUID REFERENCES users(id)
);

CREATE TABLE routine_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID REFERENCES routine_templates(id),
    assigned_user_id UUID REFERENCES users(id),
    
    generated_at TIMESTAMP DEFAULT NOW(),
    due_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    
    status VARCHAR(20) DEFAULT 'PENDING' -- PENDING, COMPLETED, OVERDUE, MISSED
);

-- 4. TICKETS & HANDOVERS
CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number SERIAL UNIQUE,
    
    -- New Context Fields from Prototype
    location VARCHAR(100) NOT NULL, -- e.g., 'Studio 1', 'Office'
    urgency urgency_context DEFAULT 'NORMAL',
    category ticket_category,
    
    subject VARCHAR(255),
    description TEXT,
    proof_image_url TEXT, -- Optional Photo
    
    requester_id UUID REFERENCES users(id), -- Consumer
    current_assignee_id UUID REFERENCES users(id), -- IT Staff
    
    status task_status DEFAULT 'PENDING',
    created_at TIMESTAMP DEFAULT NOW(),
    first_response_at TIMESTAMP,
    resolved_at TIMESTAMP,
    
    -- Handover Flag
    is_handover BOOLEAN DEFAULT FALSE
);

CREATE TABLE ticket_handovers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID REFERENCES tickets(id),
    from_user_id UUID REFERENCES users(id),
    to_user_id UUID REFERENCES users(id), -- NULL means "To Pool"
    handover_note TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 5. PWA NOTIFICATIONS
CREATE TABLE push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL
);


2. Business Logic Modules

A. Consumer Logic (Deflection & Reporting)

Search First: Sebelum tombol "Lapor" muncul/aktif, user disarankan mencari di Big Book.

Auto-Priority:

Jika User memilih Status Siaran: Darurat (ON AIR), tiket otomatis diset menjadi PRIORITY: URGENT dan mentrigger notifikasi suara keras di HP Teknisi.

Jika Pra-Siaran, set PRIORITY: HIGH.

Smart Suggestion: Saat user mengetik judul masalah (misal: "Printer"), sistem mencari artikel Big Book terkait ("Fix Printer Error") dan menampilkannya di modal.

B. IT Staff Logic (Scheduler & Handover)

Routine Scheduler:

Job berjalan per menit di Go.

Generate routine_instances berdasarkan cron.

Push Notification ke HP Staff yang sedang shift.

Handover Mechanism:

Saat tiket di-handover, status menjadi HANDOVER.

Notifikasi dikirim ke Staff shift berikutnya atau ke "General Pool".

MTTA (Response Time) dihitung untuk penerima tiket pertama, Resolution Time dihitung total.

C. Big Book Logic (Wiki)

Creation: Staff bisa membuat draft artikel dari tiket yang sudah RESOLVED.

Verification: Artikel baru statusnya Unverified. Manager harus klik "Approve" agar artikel muncul di pencarian Consumer.

Gamification: Menghitung jumlah artikel yang ditulis oleh Staff sebagai KPI "Knowledge Contribution".

3. UI/UX Plan (Prototype Mapping)

View 1: Consumer (Web Mobile)

Header: Profil User & Search Bar Besar (Fokus cari solusi).

Action Grid:

Tombol Merah: Lapor Masalah (Membuka Modal Form).

Tombol Biru: Buka Big Book.

Modal Lapor: Form wizard (Lokasi -> Urgensi -> Kategori -> Detail -> Foto).

View 2: IT Staff (PWA Mobile Native-feel)

Top Nav: Status Shift & Toggle Big Book.

Tab Home:

Urgent Checklist: Kartu Routine Task yang akan expired (Merah).

Active Tickets: List tiket masuk. Badge "Big Book Suggestion" jika ada solusi mirip.

Tab Alerts: Notifikasi tiket masuk / handover.

Tab Profile: Statistik kinerja pribadi (MTTA & Task Completion).

Ticket Detail: Chat room, tombol "Handover" (dengan form catatan), tombol "Selesai".

View 3: Manager (Desktop Admin Dashboard)

Dashboard: 4 Kartu KPI (MTTA, MTTR, FCR, Big Book Count).

Shift Management: Drag & Drop CSV untuk import jadwal massal. Tabel status staff Online/Offline.

Big Book Verification: List artikel draft yang butuh approval manager.

Reporting: Export data bulanan.

4. Development Phase Strategy

Fase 1 (Core): Setup DB, Auth (3 Roles), CRUD Tiket Dasar.

Fase 2 (Consumer & Knowledge): UI Consumer, Form Lapor (Context logic), Big Book System.

Fase 3 (Ops Automation): Scheduler Routine Task, Logic Shift, Handover.

Fase 4 (Manager & PWA): Dashboard Admin, Service Worker (Notifikasi Push), CSV Import.