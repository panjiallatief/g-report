package models

import (
	"database/sql/driver"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Enums
type UserRole string

const (
	RoleConsumer UserRole = "CONSUMER"
	RoleStaff    UserRole = "STAFF"
	RoleManager  UserRole = "MANAGER" // updated from ADMIN
)

type TicketStatus string

const (
	StatusOpen       TicketStatus = "OPEN"
	StatusInProgress TicketStatus = "IN_PROGRESS"
	StatusHandover   TicketStatus = "HANDOVER"
	StatusResolved   TicketStatus = "RESOLVED"
	StatusClosed     TicketStatus = "CLOSED"
)

type TicketPriority string

const (
	PriorityNormal       TicketPriority = "NORMAL"
	PriorityHigh         TicketPriority = "HIGH"
	PriorityUrgentOnAir  TicketPriority = "URGENT_ON_AIR"
)

type LocationEnum string

const (
	LocationStudio1     LocationEnum = "STUDIO_1"
	LocationStudio2     LocationEnum = "STUDIO_2"
	LocationMCR         LocationEnum = "MCR"
	LocationEditingRoom LocationEnum = "EDITING_ROOM"
	LocationOffice      LocationEnum = "OFFICE"
	LocationOBVan       LocationEnum = "OB_VAN"
)

// JSONB Helper
type JSONB []byte

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	*j = bytes
	return nil
}

// Models

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email        string    `gorm:"unique;not null"`
	PasswordHash string    `gorm:"not null"`
	FullName     string    `gorm:"not null"`
	Role         UserRole  `gorm:"type:user_role;default:'CONSUMER'"`
	AvatarURL    string
	IsActive     bool      `gorm:"default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Shift struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID
	StartTime time.Time `gorm:"not null"`
	EndTime   time.Time `gorm:"not null"`
	Label     string
	User      User `gorm:"foreignKey:UserID"`
}

type KnowledgeArticle struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title        string    `gorm:"not null"`
	Content      string    `gorm:"not null"`
	Category     string    // Simplified for now, mapped to enum in DB
	AuthorID     uuid.UUID
	IsVerified   bool `gorm:"default:false"`
	ViewsCount   int  `gorm:"default:0"`
	HelpfulCount int  `gorm:"default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Author       User `gorm:"foreignKey:AuthorID"`
}

type RoutineTemplate struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title           string    `gorm:"not null"`
	CronSchedule    string    `gorm:"column:cron_schedule;not null"`
	DeadlineMinutes int       `gorm:"default:30"`
	ChecklistItems  JSONB     `gorm:"type:jsonb"`
	CreatedBy       uuid.UUID
	IsActive        bool      `gorm:"default:true"`
}

type RoutineInstance struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TemplateID     uuid.UUID
	AssignedUserID uuid.UUID
	ChecklistState JSONB     `gorm:"type:jsonb"`
	GeneratedAt    time.Time `gorm:"default:now()"`
	DueAt          time.Time `gorm:"not null"`
	CompletedAt    *time.Time
	Status         string `gorm:"default:'PENDING'"`

	Template       RoutineTemplate `gorm:"foreignKey:TemplateID"`
}

type Ticket struct {
	ID                uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TicketNumber      int       `gorm:"autoIncrement;unique"`
	Location          LocationEnum `gorm:"type:location_enum"`
	Priority          TicketPriority `gorm:"type:ticket_priority;default:'NORMAL'"`
	Category          string       // Simplified
	Subject           string    `gorm:"not null"`
	Description       string
	Solution          string // Solusi penyelesaian (Big Book candidate)
	ProofImageURL     string
	RequesterID       uuid.UUID
	Status            TicketStatus `gorm:"type:ticket_status;default:'OPEN'"`
	
	CreatedAt         time.Time
	FirstResponseAt   *time.Time
	ResolvedAt        *time.Time
	ClosedAt          *time.Time
	IsHandover        bool `gorm:"default:false"`

	Requester         User `gorm:"foreignKey:RequesterID"`
}

type TicketActivity struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TicketID      uuid.UUID
	ActorID       uuid.UUID
	ActionType    string `gorm:"not null"`
	PreviousValue string
	NewValue      string
	Note          string
	CreatedAt     time.Time
	
	Actor         User `gorm:"foreignKey:ActorID"`
}

type PushSubscription struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID   uuid.UUID
	Endpoint string `gorm:"not null"`
	P256dh   string `gorm:"not null"`
	Auth     string `gorm:"not null"`
}
