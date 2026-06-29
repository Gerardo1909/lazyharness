package storage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommitEntry representa un commit del historial del harness.
type CommitEntry struct {
	Hash      string
	Message   string
	Timestamp time.Time
}

// FormatRelativeTime devuelve una descripción legible del tiempo transcurrido.
func FormatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "justo ahora"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %d hs", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("hace %d días", int(d.Hours()/24))
	default:
		return fmt.Sprintf("hace %d semanas", int(d.Hours()/(24*7)))
	}
}

// ponytail: os/exec + git CLI instead of go-git. No new dependency, git is already on the machine.
func newGitCmd(projectPath string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = HarnessPath(projectPath)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=lazyharness",
		"GIT_AUTHOR_EMAIL=lazyharness@local",
		"GIT_COMMITTER_NAME=lazyharness",
		"GIT_COMMITTER_EMAIL=lazyharness@local",
	)
	return cmd
}

// InitRepo inicializa un repo git en el directorio .lazyharness/
func InitRepo(projectPath string) error {
	dir := HarnessPath(projectPath)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	return newGitCmd(projectPath, "init").Run()
}

// Commit crea un commit con todos los archivos del harness.
func Commit(projectPath, message string) error {
	if err := InitRepo(projectPath); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := newGitCmd(projectPath, "add", "-A").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	out, err := newGitCmd(projectPath, "commit", "-m", message).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %s", string(out))
	}
	return nil
}

// LogForFile devuelve el historial de commits que tocaron un archivo de prompt.
func LogForFile(projectPath, promptFile string) ([]CommitEntry, error) {
	dir := HarnessPath(projectPath)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil, nil
	}
	relPath := filepath.Join(RolesDirName, promptFile)
	out, err := newGitCmd(projectPath, "log", "--format=%H%n%s%n%aI", "--", relPath).Output()
	if err != nil || len(out) == 0 {
		return nil, nil
	}
	return parseLogOutput(string(out)), nil
}

func parseLogOutput(raw string) []CommitEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	var entries []CommitEntry
	for i := 0; i+2 < len(lines); i += 3 {
		t, _ := time.Parse(time.RFC3339, strings.TrimSpace(lines[i+2]))
		entries = append(entries, CommitEntry{
			Hash:    strings.TrimSpace(lines[i]),
			Message: strings.TrimSpace(lines[i+1]),
			Timestamp: t,
		})
	}
	return entries
}

// DiffBetweenCommits devuelve el diff de un archivo entre dos commits.
func DiffBetweenCommits(projectPath, promptFile, fromHash, toHash string) (string, error) {
	relPath := filepath.Join(RolesDirName, promptFile)
	var out []byte
	var err error
	if fromHash == "" {
		out, err = newGitCmd(projectPath, "diff", toHash+"~1", toHash, "--", relPath).Output()
	} else {
		out, err = newGitCmd(projectPath, "diff", fromHash, toHash, "--", relPath).Output()
	}
	if err != nil || len(out) == 0 {
		return "(sin diferencias)", nil
	}
	return string(out), nil
}

// RollbackFile restaura un archivo a la versión de un commit y crea un commit nuevo.
func RollbackFile(projectPath, promptFile, hash, message string) error {
	relPath := filepath.Join(RolesDirName, promptFile)
	out, err := newGitCmd(projectPath, "show", hash+":"+relPath).Output()
	if err != nil {
		return fmt.Errorf("git show %s: %w", hash[:7], err)
	}
	path := filepath.Join(HarnessPath(projectPath), relPath)
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("escribiendo archivo: %w", err)
	}
	return Commit(projectPath, message)
}
