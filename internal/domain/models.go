package domain

import "time"

type Workspace struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"size:120;not null" json:"name"`
	Description  string     `gorm:"size:500" json:"description"`
	CoverAssetID *uint64    `json:"coverAssetId,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `gorm:"index" json:"-"`
}

type Node struct {
	ID             uint64     `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint64     `gorm:"index;not null" json:"workspaceId"`
	NodeType       string     `gorm:"size:20;not null" json:"nodeType"`
	Title          string     `gorm:"size:120" json:"title"`
	Prompt         string     `gorm:"type:text" json:"prompt"`
	ModelKey       string     `gorm:"size:120" json:"modelKey"`
	Params         string     `gorm:"type:text" json:"params"`
	PosX           float64    `json:"posX"`
	PosY           float64    `json:"posY"`
	Width          float64    `json:"width"`
	Height         float64    `json:"height"`
	CurrentAssetID *uint64    `gorm:"index" json:"currentAssetId,omitempty"`
	Version        int        `gorm:"not null;default:1" json:"version"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `gorm:"index" json:"-"`
}

type Edge struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	WorkspaceID uint64     `gorm:"index;not null" json:"workspaceId"`
	FromNodeID  uint64     `gorm:"index;not null" json:"fromNodeId"`
	ToNodeID    uint64     `gorm:"index;not null" json:"toNodeId"`
	CreatedAt   time.Time  `json:"createdAt"`
	DeletedAt   *time.Time `gorm:"index" json:"-"`
}

type Asset struct {
	ID              uint64     `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint64     `gorm:"index;not null" json:"workspaceId"`
	AssetType       string     `gorm:"size:20;not null" json:"assetType"`
	StorageProvider string     `gorm:"size:20;not null" json:"storageProvider"`
	Bucket          string     `gorm:"size:255" json:"bucket"`
	ObjectKey       string     `gorm:"size:1024;not null" json:"objectKey"`
	ContentType     string     `gorm:"size:150" json:"contentType"`
	Size            int64      `json:"size"`
	ETag            string     `gorm:"size:255" json:"etag"`
	Meta            string     `gorm:"type:text" json:"meta"`
	Source          string     `gorm:"size:30" json:"source"`
	CreatedAt       time.Time  `json:"createdAt"`
	DeletedAt       *time.Time `gorm:"index" json:"-"`
}

type NodeVersion struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	NodeID    uint64    `gorm:"index;not null" json:"nodeId"`
	Version   int       `gorm:"not null" json:"version"`
	AssetID   *uint64   `gorm:"index" json:"assetId,omitempty"`
	Prompt    string    `gorm:"type:text" json:"prompt"`
	ModelKey  string    `gorm:"size:120" json:"modelKey"`
	Params    string    `gorm:"type:text" json:"params"`
	CreatedAt time.Time `json:"createdAt"`
}

type GenerationSession struct {
	ID            uint64     `gorm:"primaryKey" json:"id"`
	WorkspaceID   uint64     `gorm:"index;not null" json:"workspaceId"`
	NodeID        uint64     `gorm:"index;not null" json:"nodeId"`
	TaskType      string     `gorm:"size:20;not null" json:"taskType"`
	ModelKey      string     `gorm:"size:120;not null" json:"modelKey"`
	Prompt        string     `gorm:"type:text" json:"prompt"`
	Params        string     `gorm:"type:text" json:"params"`
	Status        string     `gorm:"size:20;index;not null" json:"status"`
	ErrorMessage  string     `gorm:"type:text" json:"errorMessage,omitempty"`
	ResultAssetID *uint64    `gorm:"index" json:"resultAssetId,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
}

type GenerationInput struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	SessionID uint64    `gorm:"index;not null" json:"sessionId"`
	AssetID   uint64    `gorm:"index;not null" json:"assetId"`
	InputType string    `gorm:"size:20;not null" json:"inputType"`
	InputRole string    `gorm:"size:30;not null" json:"inputRole"`
	SortOrder int       `gorm:"not null" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type GenerationTask struct {
	ID              uint64     `gorm:"primaryKey" json:"id"`
	SessionID       uint64     `gorm:"uniqueIndex;not null" json:"sessionId"`
	Provider        string     `gorm:"size:30;not null" json:"provider"`
	Status          string     `gorm:"size:20;index;not null" json:"status"`
	ExternalTaskID  string     `gorm:"size:255" json:"externalTaskId,omitempty"`
	Attempt         int        `gorm:"not null;default:0" json:"attempt"`
	NextPollAt      *time.Time `gorm:"index" json:"nextPollAt,omitempty"`
	LeaseUntil      *time.Time `gorm:"index" json:"-"`
	RequestPayload  string     `gorm:"type:text" json:"-"`
	RetryMode       string     `gorm:"size:20" json:"retryMode,omitempty"`
	ResultPayload   string     `gorm:"type:text" json:"-"`
	ResponsePayload string     `gorm:"type:text" json:"-"`
	ErrorMessage    string     `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// ModelDefinition is user-managed model metadata. Credentials deliberately do
// not live here: provider secrets stay in the process environment.
type ModelDefinition struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	ModelKey    string     `gorm:"size:160;uniqueIndex;not null" json:"key"`
	Name        string     `gorm:"size:160;not null" json:"name"`
	TaskType    string     `gorm:"size:20;index;not null" json:"taskType"`
	Provider    string     `gorm:"size:40;index;not null" json:"provider"`
	Protocol    string     `gorm:"size:60;not null" json:"protocol"`
	Description string     `gorm:"size:500" json:"description"`
	Inputs      string     `gorm:"type:text;not null" json:"-"`
	Defaults    string     `gorm:"type:text;not null" json:"-"`
	Options     string     `gorm:"type:text;not null" json:"-"`
	Enabled     bool       `gorm:"index;not null" json:"enabled"`
	Builtin     bool       `gorm:"not null;default:false" json:"builtin"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `gorm:"index" json:"-"`
}

const (
	StatusPending    = "pending"
	StatusSubmitting = "submitting"
	StatusProcessing = "processing"
	StatusArchiving  = "archiving"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
)

// SystemSetting contains server-only configuration, never returned directly by the API.
type SystemSetting struct {
	ID    uint   `gorm:"primaryKey"`
	Value string `gorm:"type:text;not null" json:"-"`
}

func ActiveStatuses() []string {
	return []string{StatusPending, StatusSubmitting, StatusProcessing, StatusArchiving}
}
