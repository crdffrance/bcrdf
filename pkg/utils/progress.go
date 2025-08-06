package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ProgressBar représente une barre de progression
type ProgressBar struct {
	total     int64
	current   int64
	width     int
	startTime time.Time
	writer    io.Writer
}

// NewProgressBar crée une nouvelle barre de progression
func NewProgressBar(total int64) *ProgressBar {
	return &ProgressBar{
		total:     total,
		current:   0,
		width:     50,
		startTime: time.Now(),
		writer:    os.Stderr,
	}
}

// Update met à jour la progression
func (p *ProgressBar) Update(current int64) {
	p.current = current
	p.render()
}

// Add ajoute à la progression actuelle
func (p *ProgressBar) Add(n int64) {
	p.current += n
	p.render()
}

// render affiche la barre de progression
func (p *ProgressBar) render() {
	if p.total == 0 {
		return
	}

	percentage := float64(p.current) / float64(p.total)
	filled := int(float64(p.width) * percentage)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	// Calculer la vitesse
	elapsed := time.Since(p.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(p.current) / elapsed
	}

	// Formater la taille
	currentStr := formatBytes(p.current)
	totalStr := formatBytes(p.total)
	speedStr := formatBytes(int64(speed)) + "/s"

	// Afficher la barre
	fmt.Fprintf(p.writer, "\r[%s] %s/%s (%d%%) %s",
		bar, currentStr, totalStr, int(percentage*100), speedStr)
}

// Finish termine la barre de progression
func (p *ProgressBar) Finish() {
	p.current = p.total
	p.render()
	fmt.Fprintln(p.writer)
}

// formatBytes formate les bytes en unités lisibles
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Status affiche un statut avec un spinner
type Status struct {
	message string
	spinner []string
	current int
	writer  io.Writer
	done    chan bool
}

// NewStatus crée un nouveau statut avec spinner
func NewStatus(message string) *Status {
	return &Status{
		message: message,
		spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		writer:  os.Stderr,
		done:    make(chan bool),
	}
}

// Start démarre l'affichage du statut
func (s *Status) Start() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.render()
			}
		}
	}()
}

// render affiche le statut avec le spinner
func (s *Status) render() {
	spinner := s.spinner[s.current]
	s.current = (s.current + 1) % len(s.spinner)

	fmt.Fprintf(s.writer, "\r%s %s", spinner, s.message)
}

// Update met à jour le message
func (s *Status) Update(message string) {
	s.message = message
}

// Stop arrête l'affichage du statut
func (s *Status) Stop() {
	close(s.done)
	fmt.Fprintln(s.writer)
}

// ProgressSuccess affiche un message de succès
func ProgressSuccess(message string) {
	fmt.Fprintf(os.Stderr, "✅ %s\n", message)
}

// ProgressError affiche un message d'erreur
func ProgressError(message string) {
	fmt.Fprintf(os.Stderr, "❌ %s\n", message)
}

// ProgressWarning affiche un message d'avertissement
func ProgressWarning(message string) {
	fmt.Fprintf(os.Stderr, "⚠️  %s\n", message)
}

// ProgressInfo affiche un message d'information
func ProgressInfo(message string) {
	fmt.Fprintf(os.Stderr, "ℹ️  %s\n", message)
}

// ProgressStep affiche une étape en cours
func ProgressStep(message string) {
	fmt.Fprintf(os.Stderr, "🔄 %s\n", message)
}

// ProgressDone affiche une étape terminée
func ProgressDone(message string) {
	fmt.Fprintf(os.Stderr, "✅ %s\n", message)
}
