package domain

import "time"

type TaskStatus string

const (
	TaskPending    TaskStatus = "pendiente"
	TaskInProgress TaskStatus = "en curso"
	TaskDone       TaskStatus = "hecha"
)

type Task struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"`
	Title     string     `json:"title"`
	Report    string     `json:"report,omitempty"`
	Status    TaskStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

func NewTask(id, role, title string) Task {
	return Task{
		ID:        id,
		Role:      role,
		Title:     title,
		Status:    TaskPending,
		CreatedAt: time.Now(),
	}
}

func (t Task) IsInProgress() bool { return t.Status == TaskInProgress }
func (t Task) IsDone() bool       { return t.Status == TaskDone }
