package utils

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProgressBarInterface définit l'interface commune pour toutes les barres de progression
type ProgressBarInterface interface {
	UpdateGlobal(current int64)
	UpdateChunk(chunkCurrent, chunkTotal int64)
	SetCurrentFile(fileName string, fileSize int64)
	Finish()
	Clear()
}

// ChunkUpdater définit une interface pour mettre à jour les chunks avec nom de fichier
type ChunkUpdater interface {
	UpdateChunkWithName(fileName string, chunkCurrent, chunkTotal int64)
}

// IntegratedProgressBarInterface étend l'interface pour les barres de progression intégrées
type IntegratedProgressBarInterface interface {
	ProgressBarInterface
	UpdateChunkWithName(fileName string, chunkCurrent, chunkTotal int64)
	RemoveFile(fileName string)
	ForceRender()
}

// ProgressBar représente une barre de progression simple
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

// Clear efface la ligne de progression
func (p *ProgressBar) Clear() {
	fmt.Fprintf(p.writer, "\r%s", strings.Repeat(" ", p.width+80))
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

// DualProgressBar représente une double barre de progression
// Barre 1: Progression globale des fichiers
// Barre 2: Progression des chunks du fichier actuel
type DualProgressBar struct {
	// Barre globale (fichiers)
	globalTotal     int64
	globalCurrent   int64
	globalWidth     int
	globalStartTime time.Time

	// Barre chunk (fichier actuel)
	chunkTotal     int64
	chunkCurrent   int64
	chunkWidth     int
	chunkStartTime time.Time

	// Informations contextuelles
	currentFileName string
	currentFileSize int64
	writer          io.Writer
}

// NewDualProgressBar crée une nouvelle double barre de progression
func NewDualProgressBar(globalTotal int64) *DualProgressBar {
	return &DualProgressBar{
		globalTotal:     globalTotal,
		globalCurrent:   0,
		globalWidth:     50,
		globalStartTime: time.Now(),
		chunkTotal:      0,
		chunkCurrent:    0,
		chunkWidth:      40,
		chunkStartTime:  time.Now(),
		currentFileName: "",
		currentFileSize: 0,
		writer:          os.Stderr,
	}
}

// UpdateGlobal met à jour la progression globale des fichiers
func (dp *DualProgressBar) UpdateGlobal(current int64) {
	if current < 0 {
		current = 0
	}
	if current > dp.globalTotal {
		current = dp.globalTotal
	}
	dp.globalCurrent = current
	dp.render()
}

// UpdateChunk met à jour la progression des chunks du fichier actuel
func (dp *DualProgressBar) UpdateChunk(chunkCurrent, chunkTotal int64) {
	if chunkCurrent < 0 {
		chunkCurrent = 0
	}
	if chunkTotal < 0 {
		chunkTotal = 0
	}
	if chunkCurrent > chunkTotal {
		chunkCurrent = chunkTotal
	}

	dp.chunkCurrent = chunkCurrent
	dp.chunkTotal = chunkTotal
	dp.chunkStartTime = time.Now()
	dp.render()
}

// SetCurrentFile définit le fichier actuellement traité
func (dp *DualProgressBar) SetCurrentFile(fileName string, fileSize int64) {
	dp.currentFileName = fileName
	dp.currentFileSize = fileSize
	dp.chunkCurrent = 0
	dp.chunkTotal = 0
	dp.chunkStartTime = time.Now()
	dp.render()
}

// render affiche la double barre de progression
func (dp *DualProgressBar) render() {
	// Calculer les pourcentages
	globalPercentage := float64(0)
	if dp.globalTotal > 0 {
		globalPercentage = float64(dp.globalCurrent) / float64(dp.globalTotal)
		globalPercentage = math.Max(0, math.Min(1, globalPercentage))
	}

	chunkPercentage := float64(0)
	if dp.chunkTotal > 0 {
		chunkPercentage = float64(dp.chunkCurrent) / float64(dp.chunkTotal)
		chunkPercentage = math.Max(0, math.Min(1, chunkPercentage))
	}

	// Rendu de la barre globale
	globalFilled := int(float64(dp.globalWidth) * globalPercentage)
	globalBar := strings.Repeat("█", globalFilled) + strings.Repeat("░", dp.globalWidth-globalFilled)

	// Rendu de la barre chunk
	chunkFilled := int(float64(dp.chunkWidth) * chunkPercentage)
	chunkBar := strings.Repeat("█", chunkFilled) + strings.Repeat("░", dp.chunkWidth-chunkFilled)

	// Calculer les vitesses
	globalElapsed := time.Since(dp.globalStartTime).Seconds()
	var globalSpeed float64
	if globalElapsed > 0 {
		globalSpeed = float64(dp.globalCurrent) / globalElapsed
	}

	chunkElapsed := time.Since(dp.chunkStartTime).Seconds()
	var chunkSpeed float64
	if chunkElapsed > 0 && dp.chunkTotal > 0 {
		chunkSpeed = float64(dp.chunkCurrent) / chunkElapsed
	}

	// Formater les tailles
	globalCurrentStr := formatBytes(dp.globalCurrent)
	globalTotalStr := formatBytes(dp.globalTotal)
	globalSpeedStr := formatBytes(int64(globalSpeed)) + "/s"

	chunkCurrentStr := formatBytes(dp.chunkCurrent)
	chunkTotalStr := formatBytes(dp.chunkTotal)
	chunkSpeedStr := formatBytes(int64(chunkSpeed)) + "/s"

	// Afficher la double barre sur une seule ligne
	if dp.currentFileName != "" && dp.chunkTotal > 0 {
		// Mode avec fichier en cours (chunking)
		fileName := dp.currentFileName
		if len(fileName) > 20 {
			fileName = "..." + fileName[len(fileName)-17:]
		}

		fileSizeStr := formatBytes(dp.currentFileSize)
		fmt.Fprintf(dp.writer, "\r📁 [%s] %s/%s (%d%%) | 📦 [%s] %s (%s) %s/%s (%d%%) %s",
			globalBar, globalCurrentStr, globalTotalStr, int(globalPercentage*100),
			chunkBar, fileName, fileSizeStr, chunkCurrentStr, chunkTotalStr, int(chunkPercentage*100), chunkSpeedStr)
	} else {
		// Mode normal (pas de fichier en cours)
		fmt.Fprintf(dp.writer, "\r📁 [%s] %s/%s (%d%%) %s",
			globalBar, globalCurrentStr, globalTotalStr, int(globalPercentage*100), globalSpeedStr)
	}
}

// Finish termine la double barre de progression
func (dp *DualProgressBar) Finish() {
	dp.globalCurrent = dp.globalTotal
	dp.chunkCurrent = dp.chunkTotal
	dp.render()
	fmt.Fprintln(dp.writer)
}

// Clear efface les lignes de progression
func (dp *DualProgressBar) Clear() {
	fmt.Fprintf(dp.writer, "\r%s\n\r%s",
		strings.Repeat(" ", dp.globalWidth+50),
		strings.Repeat(" ", dp.chunkWidth+80))
}

// IntegratedProgressBar est la nouvelle barre de progression intégrée pour bcrdf
// Elle gère automatiquement l'affichage des barres globales et des fichiers
type IntegratedProgressBar struct {
	// Barre globale (tous les fichiers)
	globalTotal     int64
	globalCurrent   int64
	globalWidth     int
	globalStartTime time.Time

	// Gestion des fichiers actifs
	activeFiles map[string]*FileProgress
	fileMutex   sync.RWMutex

	// Configuration d'affichage
	maxActiveFiles int
	writer         io.Writer

	// État d'affichage
	lastRenderTime time.Time
	renderInterval time.Duration
	// Seuil d'affichage des barres fichiers
	displayThreshold time.Duration
	// Nombre de lignes rendues la dernière fois (fichiers visibles + 1 ligne globale)
	lastRenderedLines int
}

// FileProgress représente la progression d'un fichier individuel
type FileProgress struct {
	FileName     string
	FileSize     int64
	ChunkCurrent int64
	ChunkTotal   int64
	StartTime    time.Time
	LastUpdate   time.Time
	IsActive     bool
}

// NewIntegratedProgressBar crée une nouvelle barre de progression intégrée
func NewIntegratedProgressBar(globalTotal int64) *IntegratedProgressBar {
	return &IntegratedProgressBar{
		globalTotal:       globalTotal,
		globalCurrent:     0,
		globalWidth:       50,
		globalStartTime:   time.Now(),
		activeFiles:       make(map[string]*FileProgress),
		maxActiveFiles:    3, // Afficher max 3 fichiers simultanément
		writer:            os.Stderr,
		lastRenderTime:    time.Now(),
		renderInterval:    1000 * time.Millisecond, // Rendu toutes les secondes pour plus de stabilité
		displayThreshold:  3 * time.Second,
		lastRenderedLines: 0,
	}
}

// SetDisplayThreshold permet de définir le délai avant d'afficher une barre fichier
func (ip *IntegratedProgressBar) SetDisplayThreshold(threshold time.Duration) {
	if threshold >= 0 {
		ip.displayThreshold = threshold
	}
}

// SetMaxActiveFiles définit le nombre maximum de fichiers affichés
func (ip *IntegratedProgressBar) SetMaxActiveFiles(max int) {
	if max > 0 && max <= 5 {
		ip.maxActiveFiles = max
	}
}

// UpdateGlobal met à jour la progression globale des fichiers
func (ip *IntegratedProgressBar) UpdateGlobal(current int64) {
	if current < 0 {
		current = 0
	}
	if current > ip.globalTotal {
		current = ip.globalTotal
	}
	ip.globalCurrent = current
	ip.renderIfNeeded()
}

// SetCurrentFile définit ou met à jour un fichier actif
func (ip *IntegratedProgressBar) SetCurrentFile(fileName string, fileSize int64) {
	ip.fileMutex.Lock()
	defer ip.fileMutex.Unlock()

	// Créer ou mettre à jour le fichier
	ip.activeFiles[fileName] = &FileProgress{
		FileName:     fileName,
		FileSize:     fileSize,
		ChunkCurrent: 0,
		ChunkTotal:   0,
		StartTime:    time.Now(),
		LastUpdate:   time.Now(),
		IsActive:     true,
	}

	// Limiter le nombre de fichiers actifs
	if len(ip.activeFiles) > ip.maxActiveFiles {
		ip.cleanupOldFiles()
	}

	ip.renderIfNeeded()
}

// UpdateChunk met à jour la progression d'un chunk pour un fichier spécifique
func (ip *IntegratedProgressBar) UpdateChunk(fileName string, chunkCurrent, chunkTotal int64) {
	ip.fileMutex.Lock()
	if file, exists := ip.activeFiles[fileName]; exists {
		file.ChunkCurrent = chunkCurrent
		file.ChunkTotal = chunkTotal
		file.LastUpdate = time.Now()
	}
	ip.fileMutex.Unlock()

	// Appeler renderIfNeeded après avoir libéré le verrou
	ip.renderIfNeeded()
}

// UpdateChunkWithName est un alias pour UpdateChunk pour l'interface
func (ip *IntegratedProgressBar) UpdateChunkWithName(fileName string, chunkCurrent, chunkTotal int64) {
	ip.UpdateChunk(fileName, chunkCurrent, chunkTotal)
}

// RemoveFile retire un fichier de la liste des actifs
func (ip *IntegratedProgressBar) RemoveFile(fileName string) {
	ip.fileMutex.Lock()
	if _, exists := ip.activeFiles[fileName]; exists {
		// Supprimer immédiatement le fichier pour qu'il disparaisse de l'affichage
		delete(ip.activeFiles, fileName)
	}
	ip.fileMutex.Unlock()

	// Appeler renderIfNeeded après avoir libéré le verrou
	ip.renderIfNeeded()
}

// cleanupOldFiles retire les fichiers les moins récemment mis à jour
func (ip *IntegratedProgressBar) cleanupOldFiles() {
	if len(ip.activeFiles) <= ip.maxActiveFiles {
		return
	}

	// Trier par dernière mise à jour (plus anciens en premier)
	type fileInfo struct {
		name       string
		lastUpdate time.Time
	}

	var files []fileInfo
	for name, file := range ip.activeFiles {
		files = append(files, fileInfo{name, file.LastUpdate})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].lastUpdate.Before(files[j].lastUpdate)
	})

	// Supprimer les plus anciens
	toRemove := len(files) - ip.maxActiveFiles
	for i := 0; i < toRemove; i++ {
		delete(ip.activeFiles, files[i].name)
	}
}

// renderIfNeeded rend la barre seulement si nécessaire (limite la fréquence)
func (ip *IntegratedProgressBar) renderIfNeeded() {
	now := time.Now()
	if now.Sub(ip.lastRenderTime) >= ip.renderInterval {
		ip.render()
		ip.lastRenderTime = now
	}
}

// clearPreviousOutput efface l'affichage précédent en remontant et en effaçant
func (ip *IntegratedProgressBar) clearPreviousOutput() {
	// Compter le nombre de lignes à effacer
	ip.fileMutex.RLock()
	fileCount := len(ip.activeFiles)
	ip.fileMutex.RUnlock()

	// Si c'est la première fois qu'on rend, pas besoin de nettoyer
	if fileCount == 0 {
		return
	}

	// Calculer le nombre total de lignes à effacer :
	// - fileCount lignes pour les fichiers
	// - 1 ligne pour la barre globale
	totalLines := 1 + fileCount

	// Remonter au début de l'affichage
	fmt.Fprintf(ip.writer, "\r")

	// Effacer toutes les lignes en remontant et en les remplaçant par des espaces
	for i := 0; i < totalLines; i++ {
		// Effacer la ligne actuelle
		fmt.Fprintf(ip.writer, "\033[K")

		// Remonter d'une ligne (sauf pour la première itération)
		if i < totalLines-1 {
			fmt.Fprintf(ip.writer, "\033[A")
		}
	}
}

// render affiche toutes les barres de progression
func (ip *IntegratedProgressBar) render() {
	// Sélectionner les fichiers à afficher (actifs ET au-delà du seuil de 3s)
	ip.fileMutex.RLock()
	activeFiles := make([]*FileProgress, 0, len(ip.activeFiles))
	for _, file := range ip.activeFiles {
		if file.IsActive && time.Since(file.StartTime) >= ip.displayThreshold {
			activeFiles = append(activeFiles, file)
		}
	}
	ip.fileMutex.RUnlock()

	// Trier par ordre de début (plus ancien en premier)
	sort.Slice(activeFiles, func(i, j int) bool {
		return activeFiles[i].StartTime.Before(activeFiles[j].StartTime)
	})

	// Limiter le nombre de fichiers affichés
	if len(activeFiles) > ip.maxActiveFiles {
		activeFiles = activeFiles[:ip.maxActiveFiles]
	}

	// Calculer le nombre de lignes à afficher (fichiers visibles + 1 ligne globale)
	currentLines := len(activeFiles) + 1

	// Préparer l'effacement du précédent rendu sans ajouter de nouvelles lignes
	if ip.lastRenderedLines > 0 {
		// Placer le curseur en haut du bloc précédent
		fmt.Fprint(ip.writer, "\r")
		if ip.lastRenderedLines > 1 {
			fmt.Fprint(ip.writer, strings.Repeat("\033[A", ip.lastRenderedLines-1))
		}
		// Effacer chaque ligne et se déplacer vers le bas
		for i := 0; i < ip.lastRenderedLines; i++ {
			fmt.Fprint(ip.writer, "\033[2K\r")
			if i < ip.lastRenderedLines-1 {
				fmt.Fprint(ip.writer, "\033[B")
			}
		}
		// Revenir en haut du bloc pour réécrire
		if ip.lastRenderedLines > 1 {
			fmt.Fprint(ip.writer, strings.Repeat("\033[A", ip.lastRenderedLines-1))
		}
	}

	// Afficher les fichiers visibles
	for _, file := range activeFiles {
		// Calcul de progression chunks
		chunkPercentage := float64(0)
		if file.ChunkTotal > 0 {
			chunkPercentage = float64(file.ChunkCurrent) / float64(file.ChunkTotal)
			chunkPercentage = math.Max(0, math.Min(1, chunkPercentage))
		}

		chunkWidth := 40
		chunkFilled := int(float64(chunkWidth) * chunkPercentage)
		chunkBar := strings.Repeat("█", chunkFilled) + strings.Repeat("░", chunkWidth-chunkFilled)

		// Vitesse
		fileElapsed := time.Since(file.StartTime).Seconds()
		var fileSpeed float64
		if fileElapsed > 0 && file.ChunkTotal > 0 {
			fileSpeed = float64(file.ChunkCurrent) / fileElapsed
		}

		fileName := file.FileName
		if len(fileName) > 25 {
			fileName = "..." + fileName[len(fileName)-22:]
		}
		fileSizeStr := formatBytes(file.FileSize)
		chunkCurrentStr := formatBytes(file.ChunkCurrent)
		chunkTotalStr := formatBytes(file.ChunkTotal)
		fileSpeedStr := formatBytes(int64(fileSpeed)) + "/s"

		// Dessiner la ligne fichier
		fmt.Fprintf(ip.writer, "\033[2K📦 %s (%s): [%s] %s/%s (%d%%) %s\n",
			fileName, fileSizeStr, chunkBar, chunkCurrentStr, chunkTotalStr, int(chunkPercentage*100), fileSpeedStr)
	}

	// Afficher la barre globale (ligne unique)
	globalPercentage := float64(0)
	if ip.globalTotal > 0 {
		globalPercentage = float64(ip.globalCurrent) / float64(ip.globalTotal)
		globalPercentage = math.Max(0, math.Min(1, globalPercentage))
	}

	// Rendu de la barre globale
	globalFilled := int(float64(ip.globalWidth) * globalPercentage)
	globalBar := strings.Repeat("█", globalFilled) + strings.Repeat("░", ip.globalWidth-globalFilled)

	// Calculer la vitesse globale
	globalElapsed := time.Since(ip.globalStartTime).Seconds()
	var globalSpeed float64
	if globalElapsed > 0 {
		globalSpeed = float64(ip.globalCurrent) / globalElapsed
	}

	// Formater les tailles globales
	globalCurrentStr := formatBytes(ip.globalCurrent)
	globalTotalStr := formatBytes(ip.globalTotal)
	globalSpeedStr := formatBytes(int64(globalSpeed)) + "/s"

	// Dessiner la ligne globale (sans saut de ligne)
	fmt.Fprintf(ip.writer, "\033[2K📁 Global: [%s] %s/%s (%d%%) %s",
		globalBar, globalCurrentStr, globalTotalStr, int(globalPercentage*100), globalSpeedStr)

	ip.lastRenderedLines = currentLines
}

// Finish termine la barre de progression
func (ip *IntegratedProgressBar) Finish() {
	ip.globalCurrent = ip.globalTotal
	// Rendre la dernière ligne et passer à la ligne suivante
	ip.render()
	fmt.Fprintln(ip.writer)
	ip.lastRenderedLines = 0
}

// ForceRender force le rendu de la barre de progression
func (ip *IntegratedProgressBar) ForceRender() {
	ip.render()
}

// clearScreen efface l'écran précédent et remonte au début
func (ip *IntegratedProgressBar) clearScreen() {
	// Compter le nombre de lignes à effacer
	ip.fileMutex.RLock()
	fileCount := len(ip.activeFiles)
	ip.fileMutex.RUnlock()

	// Si c'est la première fois qu'on rend, pas besoin de nettoyer
	if fileCount == 0 {
		return
	}

	// Calculer le nombre total de lignes à effacer :
	// - 1 ligne pour le séparateur
	// - fileCount lignes pour les fichiers
	// - 1 ligne pour la barre globale
	totalLines := 2 + fileCount

	// Remonter au début de l'affichage
	fmt.Fprintf(ip.writer, "\r")

	// Effacer toutes les lignes en remontant et en les remplaçant par des espaces
	for i := 0; i < totalLines; i++ {
		// Remonter d'une ligne
		if i > 0 {
			fmt.Fprintf(ip.writer, "\033[A")
		}

		// Effacer la ligne en la remplissant d'espaces
		fmt.Fprintf(ip.writer, "\r%s", strings.Repeat(" ", 120))
	}

	// Retourner au début
	fmt.Fprintf(ip.writer, "\r")
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
