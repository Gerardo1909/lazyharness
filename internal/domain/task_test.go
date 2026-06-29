package domain

import "testing"

func TestTaskShouldBeCreatedWithPendingStatusWhenNewTaskCalled(t *testing.T) {
	// Arrange / Act
	task := NewTask("T-01", "arquitecto", "diseñar auth")
	// Assert
	if task.Status != TaskPending {
		t.Errorf("estado inicial esperado %q, obtuve %q", TaskPending, task.Status)
	}
	if task.ID != "T-01" {
		t.Errorf("id esperado %q, obtuve %q", "T-01", task.ID)
	}
	if task.Role != "arquitecto" {
		t.Errorf("rol esperado %q, obtuve %q", "arquitecto", task.Role)
	}
	if task.CreatedAt.IsZero() {
		t.Error("created_at no debería ser cero")
	}
}

func TestTaskShouldReportStatusCorrectly(t *testing.T) {
	// Arrange
	tests := []struct {
		name          string
		status        TaskStatus
		isInProgress  bool
		isDone        bool
	}{
		{"pendiente", TaskPending, false, false},
		{"en curso", TaskInProgress, true, false},
		{"hecha", TaskDone, false, true},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{Status: tt.status}
			// Assert
			if task.IsInProgress() != tt.isInProgress {
				t.Errorf("IsInProgress() esperado %v para estado %q", tt.isInProgress, tt.status)
			}
			if task.IsDone() != tt.isDone {
				t.Errorf("IsDone() esperado %v para estado %q", tt.isDone, tt.status)
			}
		})
	}
}

func TestHarnessSummaryShouldCountTasksCorrectly(t *testing.T) {
	// Arrange
	h, _ := NewHarness("test", "/tmp/p", "xml")
	tasks := []Task{
		{Status: TaskInProgress},
		{Status: TaskInProgress},
		{Status: TaskDone},
		{Status: TaskPending},
	}
	// Act
	summary := h.Summary(tasks)
	// Assert
	if summary.TasksInProgress != 2 {
		t.Errorf("esperaba 2 en curso, obtuve %d", summary.TasksInProgress)
	}
	if summary.TasksDone != 1 {
		t.Errorf("esperaba 1 hecha, obtuve %d", summary.TasksDone)
	}
}
