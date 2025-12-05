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
