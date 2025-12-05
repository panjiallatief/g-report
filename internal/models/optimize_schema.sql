-- =============================================
-- IT BROADCAST OPS - OPTIMIZED SCHEMA (v2.1)
-- Database: PostgreSQL 15+
-- =============================================

-- 1. ENUMS (Menjaga Konsistensi Data / Data Integrity)
CREATE TYPE user_role AS ENUM ('CONSUMER', 'STAFF', 'MANAGER');
CREATE TYPE ticket_status AS ENUM ('OPEN', 'IN_PROGRESS', 'HANDOVER', 'RESOLVED', 'CLOSED');
CREATE TYPE ticket_priority AS ENUM ('NORMAL', 'HIGH', 'URGENT_ON_AIR');
CREATE TYPE ticket_category AS ENUM ('AUDIO', 'VIDEO', 'IT_NETWORK', 'SOFTWARE', 'ELECTRICAL');
CREATE TYPE location_enum AS ENUM ('STUDIO_1', 'STUDIO_2', 'MCR', 'EDITING_ROOM', 'OFFICE', 'OB_VAN');

-- 2. USERS & AUTH
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    role user_role DEFAULT 'CONSUMER',
    avatar_url TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index untuk login cepat
CREATE INDEX idx_users_email ON users(email);

-- 3. SHIFT MANAGEMENT
CREATE TABLE shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    label VARCHAR(50), -- e.g. "Shift Pagi"
    
    -- Constraint: End time harus setelah Start time
    CONSTRAINT check_shift_time CHECK (end_time > start_time)
);

-- Index untuk query: "Siapa yang shift jam segini?"
CREATE INDEX idx_shifts_active ON shifts(start_time, end_time);

-- 4. KNOWLEDGE BASE (BIG BOOK) - Optimized for Search
CREATE TABLE knowledge_articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL, 
    category ticket_category,
    
    author_id UUID REFERENCES users(id),
    is_verified BOOLEAN DEFAULT FALSE,
    views_count INT DEFAULT 0,
    helpful_count INT DEFAULT 0,
    
    -- Full Text Search Vector (Judul + Konten)
    search_vector TSVECTOR, 
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Trigger otomatis update search_vector saat artikel disimpan
CREATE FUNCTION kb_search_update() RETURNS trigger AS $$
BEGIN
  new.search_vector :=
    setweight(to_tsvector('indonesian', new.title), 'A') ||
    setweight(to_tsvector('indonesian', new.content), 'B');
  RETURN new;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER tsvectorupdate BEFORE INSERT OR UPDATE ON knowledge_articles
FOR EACH ROW EXECUTE FUNCTION kb_search_update();

-- GIN Index: Membuat pencarian jutaan artikel tetap < 10ms
CREATE INDEX idx_kb_search ON knowledge_articles USING GIN(search_vector);


-- 5. ROUTINE TASKS (Checklists) - Using JSONB
CREATE TABLE routine_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(200) NOT NULL,
    cron_schedule VARCHAR(50) NOT NULL, -- "0 9 * * *"
    deadline_minutes INT DEFAULT 30,
    
    -- Checklist Items disimpan sebagai JSONB
    -- Contoh: [{"label": "Cek Mic 1", "checked": false}, {"label": "Cek Mic 2", "checked": false}]
    checklist_items JSONB DEFAULT '[]'::jsonb,
    
    created_by UUID REFERENCES users(id),
    is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE routine_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID REFERENCES routine_templates(id),
    assigned_user_id UUID REFERENCES users(id),
    
    checklist_state JSONB, -- Menyimpan status centang aktual
    
    generated_at TIMESTAMPTZ DEFAULT NOW(),
    due_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    
    status VARCHAR(20) DEFAULT 'PENDING'
);

-- Index untuk Dashboard Staff ("Tugas saya yang belum kelar")
CREATE INDEX idx_routine_user_status ON routine_instances(assigned_user_id, status);


-- 6. TICKETS (Inti Sistem)
CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number SERIAL UNIQUE, -- ID Manusia (#T-1001)
    
    location location_enum NOT NULL,
    priority ticket_priority DEFAULT 'NORMAL',
    category ticket_category,
    
    subject VARCHAR(255) NOT NULL,
    description TEXT,
    proof_image_url TEXT,
    
    requester_id UUID REFERENCES users(id),      -- Consumer
    current_assignee_id UUID REFERENCES users(id), -- Staff IT
    
    status ticket_status DEFAULT 'OPEN',
    
    -- Timestamps untuk KPI (MTTA & MTTR)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    first_response_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    
    is_handover BOOLEAN DEFAULT FALSE
);

-- Indexes untuk Dashboard Manager (Reporting)
CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_created_at ON tickets(created_at);
CREATE INDEX idx_tickets_assignee ON tickets(current_assignee_id); -- Untuk "My Tickets"


-- 7. TICKET ACTIVITY LOGS (Audit Trail Lengkap)
-- Menggantikan tabel 'ticket_handovers' agar lebih fleksibel
CREATE TABLE ticket_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
    actor_id UUID REFERENCES users(id), -- Siapa yang melakukan aksi
    
    action_type VARCHAR(50) NOT NULL, -- 'CREATED', 'REPLIED', 'STATUS_CHANGE', 'HANDOVER', 'RESOLVED'
    
    -- Data lama vs Baru (Untuk tracking perubahan status/assignee)
    previous_value TEXT,
    new_value TEXT,
    
    note TEXT, -- Pesan chat atau catatan handover
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index log agar history tiket loading cepat
CREATE INDEX idx_ticket_activities_ticket_id ON ticket_activities(ticket_id, created_at);


-- 8. PWA PUSH SUBSCRIPTIONS
CREATE TABLE push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);