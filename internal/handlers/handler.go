package handlers

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mr-kaspel/automatic-site-backup.git/internal/storages"
	"github.com/mr-kaspel/automatic-site-backup.git/internal/utils"
	"github.com/vmihailenco/msgpack/v5"
)

// Global cache for configurations
var (
	configCache     []storages.Configuration
	configCacheTime time.Time
	cacheMutex      sync.RWMutex
	cacheTimeout    = 5 * time.Minute
)

// File cache for frequently accessed files
var (
	fileCache        = make(map[string]utils.RemoteFile)
	fileCacheMutex   sync.RWMutex
	fileCacheTimeout = 10 * time.Minute
	fileCacheTime    = make(map[string]time.Time)
)

// getCachedConfigurations returns configurations with caching
func getCachedConfigurations() []storages.Configuration {
	cacheMutex.RLock()
	// Check if cache is valid
	if !configCacheTime.IsZero() && time.Since(configCacheTime) < cacheTimeout {
		cacheMutex.RUnlock()
		return configCache
	}
	cacheMutex.RUnlock()

	// Cache is invalid or empty, reload
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if !configCacheTime.IsZero() && time.Since(configCacheTime) < cacheTimeout {
		return configCache
	}

	// Load fresh data
	var data storages.Configuration
	configCache = data.GetFileConfiguration()
	configCacheTime = time.Now()

	return configCache
}

// invalidateConfigCache clears the configuration cache
func invalidateConfigCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	configCache = nil
	configCacheTime = time.Time{}
}

// getCachedFileInfo retrieves file info from cache if available
func getCachedFileInfo(filePath string) (utils.RemoteFile, bool) {
	fileCacheMutex.RLock()
	defer fileCacheMutex.RUnlock()

	file, exists := fileCache[filePath]
	if !exists {
		return utils.RemoteFile{}, false
	}

	// Check if cache entry is still valid
	if cacheTime, exists := fileCacheTime[filePath]; exists {
		if time.Since(cacheTime) > fileCacheTimeout {
			return utils.RemoteFile{}, false
		}
	}

	return file, true
}

// setCachedFileInfo stores file info in cache
func setCachedFileInfo(filePath string, file utils.RemoteFile) {
	fileCacheMutex.Lock()
	defer fileCacheMutex.Unlock()

	fileCache[filePath] = file
	fileCacheTime[filePath] = time.Now()
}

// clearFileCache clears the file cache
func clearFileCache() {
	fileCacheMutex.Lock()
	defer fileCacheMutex.Unlock()

	fileCache = make(map[string]utils.RemoteFile)
	fileCacheTime = make(map[string]time.Time)
}

var cmd = map[string]interface{}{
	"h": help,
	// project
	"help":     help, // 1
	"a":        add,  // 2 // -a site.ru
	"add":      add,
	"e":        edit, // 3 // -e *project ID* *data field name* *new value*
	"edit":     edit,
	"list":     configurationDataOutput, // 4 // data output from the configuration file
	"l":        configurationDataOutput,
	"d":        delet, // 5 // -d *project ID*
	"delet":    delet,
	"s":        settings, // 6 // s- *project ID*
	"settings": settings,
	// snapshot
	"c":      snapshot, // 7 // -sn *project ID*
	"create": snapshot,
	"cl":     snapshotAll,        // 8
	"ls":     listSnapshot,       // 9 // -ls *project ID*
	"gs":     getSnapshot,        // 10 // -gs *project ID* *snapshot ID*
	"gsf":    getSnapshotFiles,   // 11 // -gsf *project ID* *snapshot ID* *directory*
	"sc":     SnapshotComparison, // 12 // -sc *project ID* *snapshot ID* *snapshot ID*
}

type Question struct {
	Key   string
	Query string
}

// DownloadTask represents a file download task
type DownloadTask struct {
	RemotePath string
	LocalPath  string
	Index      int
	Total      int
}

// DownloadResult represents the result of a download task
type DownloadResult struct {
	Task    DownloadTask
	Error   error
	Success bool
}

// HashTask represents a file hash calculation task
type HashTask struct {
	File     utils.RemoteFile
	FilePath string
	Index    int
	Total    int
}

// HashResult represents the result of a hash calculation task
type HashResult struct {
	Task    HashTask
	Hash    string
	Error   error
	Success bool
}

// SnapshotMetadata represents metadata for a snapshot
type SnapshotMetadata struct {
	Timestamp      time.Time         `msgpack:"timestamp"`
	SnapshotType   string            `msgpack:"type"` // "full" or "incremental"
	ChangedFiles   []string          `msgpack:"changed_files"`
	RemovedFiles   []string          `msgpack:"removed_files"`   // files that were removed in this snapshot
	ParentSnapshot string            `msgpack:"parent_snapshot"` // filename of parent snapshot (empty for full snapshots)
	FileHashes     map[string]string `msgpack:"file_hashes"`
	TotalFiles     int               `msgpack:"total_files"`
	ArchiveSize    int64             `msgpack:"archive_size"`
}

func help(arguments []string) {
	fmt.Println(Logo)
	fmt.Println(CommandList)
}

func add(arguments []string) {
	// checking arguments
	if len(arguments) == 0 {
		fmt.Println("Not all parameters are listed, please refer to the help")
		return
	}

	var data storages.Configuration
	answersQuestions := make(map[string]string)
	var input string
	questions := []Question{
		{"protocol", "Enter the protocol for connecting to a remote server (FTP, SFTP):"},
		{"port", "Enter the port to connect to the remote server (21 FTP, 22 SFTP):"},
		{"host", "Enter the address of the remote server:"},
		{"login", "Enter the login from the account to connect to the remote server:"},
		{"password", "Enter the password for the account to connect to the remote server:"},
		{"dblogin", "Enter the login from the account to connect to the database:"},
		{"dbpassword", "Enter the password for the account to connect to the database:"},
		{"dbtype", "Enter the database type (mysql, postgresql, sqlite) or leave empty to skip database backup:"},
		{"dbhost", "Enter the database host (usually localhost) or leave empty:"},
		{"dbport", "Enter the database port (3306 for MySQL, 5432 for PostgreSQL) or leave empty:"},
		{"dbdatabase", "Enter the database name or SQLite file path or leave empty:"},
		{"rootdirectory", "Enter the full path to the project root directory on the remote server:"},
		{"savedirectory", "Enter the address of the local directory where you want to save the project snapshot:"},
		{"maxthreads", "Enter the maximum number of threads for parallel processing (default: 4, recommended: 4-8):"},
	}

	// collecting answers to questions
	answersQuestions["name"] = arguments[0]
	for _, question := range questions {
		fmt.Println(question.Query)

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		input = scanner.Text()
		answersQuestions[question.Key] = input
	}

	// create a Configuration object from responses
	newConfig := storages.Configuration{
		Name:          answersQuestions["name"],
		Protocol:      answersQuestions["protocol"],
		Port:          answersQuestions["port"],
		Host:          answersQuestions["host"],
		Login:         answersQuestions["login"],
		Password:      answersQuestions["password"],
		DBlogin:       answersQuestions["dblogin"],
		DBpassword:    answersQuestions["dbpassword"],
		DBType:        answersQuestions["dbtype"],
		DBHost:        answersQuestions["dbhost"],
		DBPort:        answersQuestions["dbport"],
		DBDatabase:    answersQuestions["dbdatabase"],
		RootDirectory: answersQuestions["rootdirectory"],
		SaveDirectory: answersQuestions["savedirectory"],
		MaxThreads:    answersQuestions["maxthreads"],
	}

	// reading current configurations from a file
	var existingConfigs = getCachedConfigurations()

	// adding a new configuration
	existingConfigs = append(existingConfigs, newConfig)

	// save all configurations
	data.SaveConfigurations(existingConfigs)

	// invalidate cache after modification
	invalidateConfigCache()
}

func edit(arguments []string) {
	// argument validation utils
	// ...
	if len(arguments) < 3 {
		fmt.Println("not all required parameters are listed, please provide id, field name, and new value")
		return
	}

	var data storages.Configuration
	data.EditReceivedData(arguments)

	// invalidate cache after modification
	invalidateConfigCache()
}

func configurationDataOutput(arguments []string) {
	dataBin := getCachedConfigurations()

	// Check if there are any configurations
	if len(dataBin) == 0 {
		fmt.Println("No configurations found.")
		return
	}

	// output each configuration on a new line
	for i, config := range dataBin {
		fmt.Printf("\033[38;2;31;111;235m Configuration %d:\033[0m\n", i+1)
		fmt.Printf("\tName: %s\n\tProtocol: %s\n\tPort: %s\n\tHost: %s\n\tLogin: %s\n\tPassword: %s\n\tDB Login: %s\n\tDB Password: %s\n\tDB Type: %s\n\tDB Host: %s\n\tDB Port: %s\n\tDB Database: %s\n\tRoot Directory: <%s>\n\tSave Directory: <%s>\n\tMax Threads: %s\n",
			config.Name, config.Protocol, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.DBType, config.DBHost, config.DBPort, config.DBDatabase, config.RootDirectory, config.SaveDirectory, config.MaxThreads)
		fmt.Println() // empty line to separate configurations
	}
}

func delet(arguments []string) {
	// checking arguments
	if len(arguments) == 0 {
		fmt.Println("Please provide the configuration id to delete.")
		return
	}

	// parsing configuration id
	idStr := arguments[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("Invalid id format:", err)
		return
	}

	dataBin := getCachedConfigurations()

	// check for the presence of a configuration with a given id
	if id < 0 || id >= len(dataBin) {
		fmt.Printf("Configuration with id %d not found\n", id)
		return
	}

	// request for confirmation of deletion
	fmt.Printf("Are you sure you want to delete configuration with id %d? Type 'yes' or 'y' to confirm: ", id)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	confirmation := strings.ToLower(scanner.Text())

	// checking the user's response
	if confirmation != "yes" && confirmation != "y" {
		fmt.Println("Deletion canceled.")
		return
	}

	// removing configuration
	dataBin = append(dataBin[:id], dataBin[id+1:]...)

	// saving data
	var data storages.Configuration
	data.SaveConfigurations(dataBin)

	// invalidate cache after modification
	invalidateConfigCache()

	fmt.Println("Configuration deleted successfully.")
}

func settings(arguments []string) {
	// parsing configuration id
	idStr := arguments[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("Invalid id format:", err)
		return
	}

	var dataBin = getCachedConfigurations()

	// search configuration by ID
	if id < 0 || id > len(dataBin) {
		fmt.Printf("Configuration with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]

	fmt.Printf("Configuration %d:\n", id)
	fmt.Printf("\tName: %s\r\n\tProtocol: %s\r\n\tPort: %s\r\n\tHost: %s\r\n\tLogin: %s\r\n\tPassword: %s\r\n\tDB Login: %s\r\n\tDB Password: %s\r\n\tDB Type: %s\r\n\tDB Host: %s\r\n\tDB Port: %s\r\n\tDB Database: %s\r\n\tRoot Directory: <%s>\r\n\tSave Directory: <%s>\r\n\tMax Threads: %s\n",
		config.Name, config.Protocol, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.DBType, config.DBHost, config.DBPort, config.DBDatabase, config.RootDirectory, config.SaveDirectory, config.MaxThreads)

}

func snapshot(arguments []string) {
	// parsing Configuration ID
	idStr := arguments[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("Invalid id format:", err)
		return
	}

	// reading configurations
	var dataBin = getCachedConfigurations()

	// search configuration by ID
	if id < 0 || id > len(dataBin) {
		fmt.Printf("Configuration with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]
	fmt.Printf("Project: %s\n", config.Name)
	fmt.Printf("Protocol: %s\n", config.Protocol)
	fmt.Printf("Host: %s:%s\n", config.Host, config.Port)
	fmt.Printf("Login: %s\n", config.Login)
	fmt.Printf("Root Directory: %s\n", config.RootDirectory)

	saveDir := config.SaveDirectory
	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	// Load existing hashes from the latest snapshot metadata
	fileHashes := loadLatestSnapshotHashes(saveDir)

	// preparing a client connection (FTP/SFTP)
	fmt.Printf("Connecting to %s server...\n", config.Protocol)
	var client utils.FTPClient // interface for working with FTP/SFTP
	if config.Protocol == "FTP" {
		client, err = utils.NewFTPClient(config.Host, config.Login, config.Password, config.Port)
		if err != nil {
			fmt.Printf("Failed to connect to FTP server: %v\n", err)
			return
		}
		fmt.Println("FTP connection established successfully")
	} else if config.Protocol == "SFTP" {
		client, err = utils.NewSFTPClient(config.Host, config.Login, config.Password, config.Port)
		if err != nil {
			fmt.Printf("Failed to connect to SFTP server: %v\n", err)
			return
		}
		fmt.Println("SFTP connection established successfully")
	} else {
		fmt.Println("Unsupported protocol for connection.")
		return
	}
	defer client.Close()

	// Trim whitespace and check if root directory is empty
	config.RootDirectory = strings.TrimSpace(config.RootDirectory)

	if config.RootDirectory == "" {
		fmt.Println("Root directory not specified, getting current directory...")
		currentDir, err := client.CurrentDir()
		if err != nil {
			fmt.Printf("Failed to get current directory: %v\n", err)
			return
		}
		config.RootDirectory = currentDir
		fmt.Printf("Using current directory: %s\n", config.RootDirectory)
	} else {
		fmt.Printf("Using specified root directory: %s\n", config.RootDirectory)
	}

	// getting a list of files from the server
	files, err := client.ListFiles(config.RootDirectory)
	if err != nil {
		fmt.Printf("Failed to list files on the server in directory '%s': %v\n", config.RootDirectory, err)
		return
	}

	// Get adaptive max threads for parallel processing
	maxThreads := getAdaptiveThreads(config.MaxThreads, len(files))
	fmt.Printf("Using %d threads for parallel processing (%d files)\n", maxThreads, len(files))

	// list for updated hashes and files to download
	updatedHashes := make(map[string]string)
	filesToDownload := []string{}
	removedFiles := []string{}

	// Calculate hashes in parallel with batching for large file sets
	fmt.Printf("Calculating hashes for %d files using %d threads...\n", len(files), maxThreads)
	hashResults := calculateHashesInBatches(client, files, config.RootDirectory, maxThreads)

	// Process hash results and determine files to download
	for _, result := range hashResults {
		if result.Error != nil {
			// Skip directories that can't be hashed (they don't need hashes)
			if result.Task.File.IsDir {
				continue
			}
			fmt.Printf("Failed to calculate hash for file %s: %v\n", result.Task.FilePath, result.Error)
			continue
		}

		// checking for changes
		if existingHash, exists := fileHashes[result.Task.FilePath]; !exists || existingHash != result.Hash {
			filesToDownload = append(filesToDownload, result.Task.FilePath)
		}

		// saving a new hash
		updatedHashes[result.Task.FilePath] = result.Hash
	}

	// removing missing files from hashes
	for path := range fileHashes {
		if _, exists := updatedHashes[path]; !exists {
			fmt.Printf("File %s has been removed from the server.\n", path)
			removedFiles = append(removedFiles, path)
		}
	}

	// Determine snapshot type and parent
	snapshotType := "full"
	parentSnapshot := ""

	// Check if this is the first snapshot
	// Full snapshots are created only for the first backup, all subsequent are incremental
	existingFiles, err := os.ReadDir(saveDir)
	if err == nil {
		var existingSnapshots []string
		for _, file := range existingFiles {
			if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
				existingSnapshots = append(existingSnapshots, file.Name())
			}
		}

		if len(existingSnapshots) > 0 {
			// Find the latest snapshot
			latestSnapshot := existingSnapshots[0]
			for _, snapshot := range existingSnapshots[1:] {
				if snapshot > latestSnapshot { // Simple string comparison works for our timestamp format
					latestSnapshot = snapshot
				}
			}

			// If there are existing snapshots, create incremental snapshot
			snapshotType = "incremental"
			parentSnapshot = latestSnapshot
		}
	}

	// downloading files with parallel processing
	if snapshotType == "full" {
		// For full snapshots, download ALL files
		fmt.Printf("Downloading ALL %d files for full snapshot using %d threads...\n", len(files), maxThreads)

		// Create download tasks for all files
		tasks := make([]DownloadTask, len(files))
		for i, file := range files {
			// Skip directories
			if file.IsDir {
				continue
			}

			// Preserve directory structure by using the full path
			// Remove root directory prefix from filePath to avoid nested directories in archive
			localPath := file.Name
			if strings.HasPrefix(file.Name, config.RootDirectory) {
				localPath = strings.TrimPrefix(file.Name, config.RootDirectory)
				localPath = strings.TrimPrefix(localPath, "/")
			}
			localFilePath := filepath.Join(saveDir, "temp", localPath)
			tasks[i] = DownloadTask{
				RemotePath: file.Name,
				LocalPath:  localFilePath,
				Index:      i + 1,
				Total:      len(files),
			}
		}

		// Execute parallel downloads
		downloadFilesParallel(client, tasks, maxThreads)
	} else if len(filesToDownload) > 0 {
		// For incremental snapshots, download only changed files
		fmt.Printf("Downloading %d modified files using %d threads...\n", len(filesToDownload), maxThreads)

		// Create download tasks
		tasks := make([]DownloadTask, len(filesToDownload))
		for i, filePath := range filesToDownload {
			// Preserve directory structure by using the full path
			// Remove root directory prefix from filePath to avoid nested directories in archive
			localPath := filePath
			if strings.HasPrefix(filePath, config.RootDirectory) {
				localPath = strings.TrimPrefix(filePath, config.RootDirectory)
				localPath = strings.TrimPrefix(localPath, "/")
			}
			localFilePath := filepath.Join(saveDir, "temp", localPath)
			tasks[i] = DownloadTask{
				RemotePath: filePath,
				LocalPath:  localFilePath,
				Index:      i + 1,
				Total:      len(filesToDownload),
			}
		}

		// Execute parallel downloads
		downloadFilesParallel(client, tasks, maxThreads)
	} else {
		fmt.Println("No files to download - all files are up to date")
	}

	// Hash file is now saved as part of snapshot metadata, no need to save separately

	// Database backup logic
	if config.DBType != "" {
		fmt.Println("Starting database backup...")

		// Validate database configuration
		err = utils.ValidateDatabaseConfig(config)
		if err != nil {
			fmt.Printf("Database configuration error: %v\n", err)
		} else {
			// Perform database backup (script upload/delete is handled internally)
			err = utils.BackupDatabase(config, client)
			if err != nil {
				fmt.Printf("Database backup failed: %v\n", err)
			}
		}
	}

	// Create archive with only changed files for incremental snapshots
	archiveName := fmt.Sprintf("backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	archivePath := filepath.Join(saveDir, archiveName)

	if snapshotType == "full" {
		// Full snapshot: archive all files in the downloaded directory
		// Create archive with proper path structure
		err = createFullSnapshotArchive(archivePath, saveDir)
	} else {
		// Incremental snapshot: archive only changed files
		err = createIncrementalArchive(archivePath, saveDir, filesToDownload)
	}

	if err != nil {
		fmt.Println("Failed to create archive:", err)
		return
	}

	// Clean up temporary files after successful archive creation
	if len(filesToDownload) > 0 {
		cleanupTempFiles(saveDir, filesToDownload)
	}

	// Get archive size
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		fmt.Println("Failed to get archive info:", err)
		return
	}

	// Create and save metadata
	metadata := SnapshotMetadata{
		Timestamp:      time.Now(),
		SnapshotType:   snapshotType,
		ChangedFiles:   filesToDownload,
		RemovedFiles:   removedFiles,
		ParentSnapshot: parentSnapshot,
		FileHashes:     updatedHashes,
		TotalFiles:     len(updatedHashes),
		ArchiveSize:    archiveInfo.Size(),
	}

	err = saveSnapshotMetadata(saveDir, archiveName, metadata)
	if err != nil {
		fmt.Println("Failed to save snapshot metadata:", err)
	} else {
		fmt.Printf("Backup archive created: %s (%s snapshot, %.2f MB)\n",
			archivePath, snapshotType, float64(archiveInfo.Size())/(1024*1024))
	}
}

func snapshotAll(arguments []string) {
	var dataBin = getCachedConfigurations()

	if len(dataBin) == 0 {
		fmt.Println("No projects found. Add a project first using 'snp -a <domain>'")
		return
	}

	fmt.Printf("Found %d projects. Starting backup process...\n", len(dataBin))

	for i, config := range dataBin {
		fmt.Printf("\n[%d/%d] Processing project: %s\n", i+1, len(dataBin), config.Name)

		// Создаем аргументы для функции snapshot
		projectArgs := []string{strconv.Itoa(i + 1)}

		// Вызываем snapshot для каждого проекта
		snapshot(projectArgs)

		fmt.Printf("Completed backup for project: %s\n", config.Name)
	}

	fmt.Println("\nAll projects backup completed!")
}

func listSnapshot(arguments []string) {
	// Проверяем аргументы
	if len(arguments) == 0 {
		fmt.Println("Please provide project ID. Usage: snp -ls <project ID>")
		return
	}

	// Парсим ID проекта
	idStr := arguments[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return
	}

	// Получаем конфигурации
	var dataBin = getCachedConfigurations()

	// Проверяем существование проекта
	if id < 1 || id > len(dataBin) {
		fmt.Printf("Project with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]
	saveDir := config.SaveDirectory

	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	fmt.Printf("Available snapshots for project '%s':\n", config.Name)
	fmt.Println(strings.Repeat("-", 50))

	// Ищем все архивы в директории сохранения
	files, err := os.ReadDir(saveDir)
	if err != nil {
		fmt.Printf("Failed to read save directory: %v\n", err)
		return
	}

	snapshotCount := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
			snapshotCount++

			// Получаем информацию о файле
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			fileName := file.Name()

			// Try to load metadata
			metadata, err := loadSnapshotMetadata(saveDir, fileName)
			if err != nil {
				// Fallback to legacy format
				if len(fileName) >= 21 { // backup_ + 15 символов даты + .tar.gz
					dateStr := fileName[7:21] // извлекаем 20060102_150405
					if parsedTime, err := time.Parse("20060102_150405", dateStr); err == nil {
						fmt.Printf("ID: %d | Date: %s | Size: %.2f MB | Type: legacy\n",
							snapshotCount,
							parsedTime.Format("2006-01-02 15:04:05"),
							float64(fileInfo.Size())/(1024*1024))
					} else {
						fmt.Printf("ID: %d | File: %s | Size: %.2f MB | Type: legacy\n",
							snapshotCount,
							fileName,
							float64(fileInfo.Size())/(1024*1024))
					}
				} else {
					fmt.Printf("ID: %d | File: %s | Size: %.2f MB | Type: legacy\n",
						snapshotCount,
						fileName,
						float64(fileInfo.Size())/(1024*1024))
				}
			} else {
				// Display with metadata information
				snapshotType := metadata.SnapshotType
				if metadata.SnapshotType == "incremental" && metadata.ParentSnapshot != "" {
					snapshotType = fmt.Sprintf("incremental (parent: %s)", strings.TrimSuffix(metadata.ParentSnapshot, ".tar.gz"))
				}

				fmt.Printf("ID: %d | Date: %s | Size: %.2f MB | Type: %s | Files: %d\n",
					snapshotCount,
					metadata.Timestamp.Format("2006-01-02 15:04:05"),
					float64(fileInfo.Size())/(1024*1024),
					snapshotType,
					metadata.TotalFiles)
			}
		}
	}

	if snapshotCount == 0 {
		fmt.Println("No snapshots found for this project.")
		fmt.Println("Create a snapshot first using: snp -c", id)
	} else {
		fmt.Printf("\nTotal snapshots: %d\n", snapshotCount)
	}
}

func getSnapshot(arguments []string) {
	// Проверяем аргументы
	if len(arguments) < 2 {
		fmt.Println("Please provide project ID and snapshot ID. Usage: snp -gs <project ID> <snapshot ID>")
		return
	}

	// Парсим ID проекта
	projectIDStr := arguments[0]
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return
	}

	// Парсим ID снимка
	snapshotIDStr := arguments[1]
	snapshotID, err := strconv.Atoi(snapshotIDStr)
	if err != nil {
		fmt.Println("Invalid snapshot ID format:", err)
		return
	}

	// Получаем конфигурации
	var dataBin = getCachedConfigurations()

	// Проверяем существование проекта
	if projectID < 1 || projectID > len(dataBin) {
		fmt.Printf("Project with ID %d not found.\n", projectID)
		return
	}

	config := dataBin[projectID-1]
	saveDir := config.SaveDirectory

	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	// Ищем все архивы в директории сохранения
	files, err := os.ReadDir(saveDir)
	if err != nil {
		fmt.Printf("Failed to read save directory: %v\n", err)
		return
	}

	// Собираем список архивов
	var archiveFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
			archiveFiles = append(archiveFiles, file.Name())
		}
	}

	// Проверяем существование снимка
	if snapshotID < 1 || snapshotID > len(archiveFiles) {
		fmt.Printf("Snapshot with ID %d not found. Available snapshots: %d\n", snapshotID, len(archiveFiles))
		return
	}

	selectedArchive := archiveFiles[snapshotID-1]
	archivePath := filepath.Join(saveDir, selectedArchive)

	// Создаем директорию для извлечения
	extractDir := filepath.Join(saveDir, fmt.Sprintf("extracted_%s", strings.TrimSuffix(selectedArchive, ".tar.gz")))

	fmt.Printf("Extracting snapshot '%s' to directory: %s\n", selectedArchive, extractDir)

	// Создаем директорию для извлечения
	err = os.MkdirAll(extractDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create extraction directory: %v\n", err)
		return
	}

	// Load snapshot metadata
	metadata, err := loadSnapshotMetadata(saveDir, selectedArchive)
	if err != nil {
		fmt.Printf("Failed to load snapshot metadata: %v\n", err)
		fmt.Println("Attempting to extract as legacy snapshot...")

		// Fallback to legacy extraction
		err = utils.ExtractTarGz(archivePath, extractDir)
		if err != nil {
			fmt.Printf("Failed to extract archive: %v\n", err)
			os.RemoveAll(extractDir)
			return
		}
	} else {
		// Extract using incremental logic
		err = extractIncrementalSnapshot(saveDir, selectedArchive, extractDir, metadata, config.RootDirectory)
		if err != nil {
			fmt.Printf("Failed to extract incremental snapshot: %v\n", err)
			os.RemoveAll(extractDir)
			return
		}
	}

	fmt.Printf("Snapshot successfully extracted to: %s\n", extractDir)

	// Показываем содержимое извлеченной директории
	fmt.Println("\nExtracted files:")
	fmt.Println(strings.Repeat("-", 30))

	err = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(extractDir, path)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fmt.Printf("  %s (%.2f KB)\n", relPath, float64(info.Size())/1024)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error listing extracted files: %v\n", err)
	}
}

func getSnapshotFiles(arguments []string) {
	// Проверяем аргументы
	if len(arguments) < 3 {
		fmt.Println("Please provide project ID, snapshot ID and directory path. Usage: snp -gsf <project ID> <snapshot ID> <directory>")
		return
	}

	// Парсим ID проекта
	projectIDStr := arguments[0]
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return
	}

	// Парсим ID снимка
	snapshotIDStr := arguments[1]
	snapshotID, err := strconv.Atoi(snapshotIDStr)
	if err != nil {
		fmt.Println("Invalid snapshot ID format:", err)
		return
	}

	// Получаем путь к директории/файлу
	targetPath := arguments[2]

	// Получаем конфигурации
	var dataBin = getCachedConfigurations()

	// Проверяем существование проекта
	if projectID < 1 || projectID > len(dataBin) {
		fmt.Printf("Project with ID %d not found.\n", projectID)
		return
	}

	config := dataBin[projectID-1]
	saveDir := config.SaveDirectory

	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	// Ищем все архивы в директории сохранения
	files, err := os.ReadDir(saveDir)
	if err != nil {
		fmt.Printf("Failed to read save directory: %v\n", err)
		return
	}

	// Собираем список архивов
	var archiveFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
			archiveFiles = append(archiveFiles, file.Name())
		}
	}

	// Проверяем существование снимка
	if snapshotID < 1 || snapshotID > len(archiveFiles) {
		fmt.Printf("Snapshot with ID %d not found. Available snapshots: %d\n", snapshotID, len(archiveFiles))
		return
	}

	selectedArchive := archiveFiles[snapshotID-1]

	fmt.Printf("Searching for '%s' in snapshot chain starting from '%s'...\n", targetPath, selectedArchive)

	// Строим цепочку снимков
	chain, err := buildSnapshotChain(saveDir, selectedArchive)
	if err != nil {
		fmt.Printf("Failed to build snapshot chain: %v\n", err)
		return
	}

	fmt.Printf("Snapshot chain: %v\n", chain)

	// Ищем файл в цепочке снимков (от более позднего к более раннему)
	var foundArchive string
	for _, archiveName := range chain {
		archivePath := filepath.Join(saveDir, archiveName)

		// Проверяем, есть ли файл в этом архиве
		if fileExistsInArchive(archivePath, targetPath, config.RootDirectory) {
			foundArchive = archiveName
			fmt.Printf("Found '%s' in archive: %s\n", targetPath, archiveName)
			break
		}
	}

	if foundArchive == "" {
		fmt.Printf("Path '%s' not found in any snapshot of the chain.\n", targetPath)
		return
	}

	// Создаем временную директорию для извлечения
	tempDir := filepath.Join(saveDir, fmt.Sprintf("temp_extract_%d", time.Now().Unix()))

	// Создаем временную директорию
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create temporary directory: %v\n", err)
		return
	}

	// Извлекаем файл из найденного архива
	archivePath := filepath.Join(saveDir, foundArchive)
	err = extractFileFromArchive(archivePath, targetPath, tempDir, config.RootDirectory)
	if err != nil {
		fmt.Printf("Failed to extract file from archive: %v\n", err)
		os.RemoveAll(tempDir)
		return
	}

	// Ищем целевой файл/директорию в временной директории
	sourcePath := filepath.Join(tempDir, filepath.Base(targetPath))

	// Проверяем существование
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		fmt.Printf("Path '%s' not found after extraction.\n", targetPath)
		os.RemoveAll(tempDir)
		return
	}

	// Создаем директорию назначения
	destDir := filepath.Join(saveDir, fmt.Sprintf("extracted_files_%s", time.Now().Format("20060102_150405")))
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create destination directory: %v\n", err)
		os.RemoveAll(tempDir)
		return
	}

	// Копируем файл/директорию
	destPath := filepath.Join(destDir, filepath.Base(targetPath))

	// Проверяем, это файл или директория
	info, err := os.Stat(sourcePath)
	if err != nil {
		fmt.Printf("Failed to get file info: %v\n", err)
		os.RemoveAll(tempDir)
		os.RemoveAll(destDir)
		return
	}

	if info.IsDir() {
		// Копируем директорию
		err = copyDirectory(sourcePath, destPath)
		if err != nil {
			fmt.Printf("Failed to copy directory: %v\n", err)
			os.RemoveAll(tempDir)
			os.RemoveAll(destDir)
			return
		}
		fmt.Printf("Directory '%s' extracted to: %s\n", targetPath, destPath)
	} else {
		// Копируем файл
		err = copyFile(sourcePath, destPath)
		if err != nil {
			fmt.Printf("Failed to copy file: %v\n", err)
			os.RemoveAll(tempDir)
			os.RemoveAll(destDir)
			return
		}
		fmt.Printf("File '%s' extracted to: %s\n", targetPath, destPath)
	}

	// Показываем содержимое извлеченного
	fmt.Println("\nExtracted content:")
	fmt.Println(strings.Repeat("-", 30))

	err = filepath.Walk(destPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(destDir, path)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fmt.Printf("  %s (%.2f KB)\n", relPath, float64(info.Size())/1024)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error listing extracted content: %v\n", err)
	}

	// Удаляем временную директорию
	os.RemoveAll(tempDir)
}

// Вспомогательные функции для копирования
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

// downloadFilesParallel downloads files using multiple goroutines
func downloadFilesParallel(client utils.FTPClient, tasks []DownloadTask, maxThreads int) {
	if len(tasks) == 0 {
		return
	}

	// Create channels for tasks and results
	taskChan := make(chan DownloadTask, len(tasks))
	resultChan := make(chan DownloadResult, len(tasks))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxThreads; i++ {
		wg.Add(1)
		go downloadWorker(client, taskChan, resultChan, &wg)
	}

	// Send tasks to workers
	go func() {
		defer close(taskChan)
		for _, task := range tasks {
			taskChan <- task
		}
	}()

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and show progress
	successCount := 0
	errorCount := 0

	for result := range resultChan {
		if result.Success {
			successCount++
			fmt.Printf("[%d/%d] ✓ Downloaded: %s\n",
				result.Task.Index, result.Task.Total, result.Task.RemotePath)
		} else {
			errorCount++
			fmt.Printf("[%d/%d] ✗ Failed: %s - %v\n",
				result.Task.Index, result.Task.Total, result.Task.RemotePath, result.Error)
		}
	}

	// Show summary
	fmt.Printf("\nDownload Summary:\n")
	fmt.Printf("  ✓ Successfully downloaded: %d files\n", successCount)
	if errorCount > 0 {
		fmt.Printf("  ✗ Failed downloads: %d files\n", errorCount)
	}
}

// downloadWorker is a worker goroutine that processes download tasks
func downloadWorker(client utils.FTPClient, taskChan <-chan DownloadTask, resultChan chan<- DownloadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range taskChan {
		// Create local directory if needed
		err := os.MkdirAll(filepath.Dir(task.LocalPath), 0755)
		if err != nil {
			resultChan <- DownloadResult{
				Task:    task,
				Error:   fmt.Errorf("failed to create directory: %v", err),
				Success: false,
			}
			continue
		}

		// Download the file
		err = client.DownloadFile(task.RemotePath, task.LocalPath)
		resultChan <- DownloadResult{
			Task:    task,
			Error:   err,
			Success: err == nil,
		}
	}
}

// calculateHashesInBatches calculates file hashes using batching for memory efficiency
func calculateHashesInBatches(client utils.FTPClient, files []utils.RemoteFile, rootDir string, maxThreads int) []HashResult {
	if len(files) == 0 {
		return []HashResult{}
	}

	// Determine optimal batch size based on file count
	batchSize := getOptimalBatchSize(len(files))
	fmt.Printf("Processing %d files in batches of %d\n", len(files), batchSize)

	var allResults []HashResult

	// Process files in batches
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		fmt.Printf("Processing batch %d-%d of %d files...\n", i+1, end, len(files))

		// Process current batch
		batchResults := calculateHashesParallel(client, batch, rootDir, maxThreads)
		allResults = append(allResults, batchResults...)

		// Force garbage collection between batches for large file sets
		if len(files) > 10000 {
			runtime.GC()
		}
	}

	return allResults
}

// getOptimalBatchSize determines the best batch size based on file count
func getOptimalBatchSize(fileCount int) int {
	if fileCount < 1000 {
		return fileCount // Process all at once for small sets
	} else if fileCount < 10000 {
		return 1000
	} else if fileCount < 100000 {
		return 5000
	} else {
		return 10000 // Cap at 10k for very large sets
	}
}

// getAdaptiveThreads determines optimal thread count based on file count and system resources
func getAdaptiveThreads(configThreads string, fileCount int) int {
	// Parse configured threads
	configured := utils.GetMaxThreads(configThreads)

	// Adaptive logic based on file count
	if fileCount < 100 {
		return min(configured, 2) // Don't over-thread for small sets
	} else if fileCount < 1000 {
		return min(configured, 4)
	} else if fileCount < 10000 {
		return min(configured, 8)
	} else if fileCount < 100000 {
		return min(configured, 16)
	} else {
		// For very large sets, use more threads but cap at system limits
		maxThreads := min(configured, runtime.NumCPU()*4)
		return min(maxThreads, 32) // Cap at 32 threads
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateHashesParallel calculates file hashes using multiple goroutines
func calculateHashesParallel(client utils.FTPClient, files []utils.RemoteFile, rootDir string, maxThreads int) []HashResult {
	if len(files) == 0 {
		return []HashResult{}
	}

	// Create hash tasks (only for files, not directories)
	var tasks []HashTask
	taskIndex := 0

	for _, file := range files {
		// Skip directories - they don't need hashes
		if file.IsDir {
			continue
		}

		// If file.Name already contains the full path, use it as is
		// Otherwise, join with rootDir
		var filePath string
		if strings.HasPrefix(file.Name, "/") {
			filePath = file.Name
		} else {
			filePath = filepath.Join(rootDir, file.Name)
		}
		// Convert to forward slashes for consistency
		filePath = filepath.ToSlash(filePath)

		tasks = append(tasks, HashTask{
			File:     file,
			FilePath: filePath,
			Index:    taskIndex + 1,
			Total:    len(files),
		})
		taskIndex++
	}

	// Create channels for tasks and results
	taskChan := make(chan HashTask, len(tasks))
	resultChan := make(chan HashResult, len(tasks))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxThreads; i++ {
		wg.Add(1)
		go hashWorker(client, taskChan, resultChan, &wg)
	}

	// Send tasks to workers
	go func() {
		defer close(taskChan)
		for _, task := range tasks {
			taskChan <- task
		}
	}()

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []HashResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// hashWorker is a worker goroutine that processes hash calculation tasks
func hashWorker(client utils.FTPClient, taskChan <-chan HashTask, resultChan chan<- HashResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range taskChan {
		// Calculate file hash
		hash, err := client.HashFile(task.FilePath)
		resultChan <- HashResult{
			Task:    task,
			Hash:    hash,
			Error:   err,
			Success: err == nil,
		}
	}
}

// extractIncrementalSnapshot extracts a snapshot by building the complete file set from the chain
func extractIncrementalSnapshot(saveDir, archiveName, extractDir string, metadata *SnapshotMetadata, rootDirectory string) error {
	// Build the snapshot chain
	chain, err := buildSnapshotChain(saveDir, archiveName)
	if err != nil {
		return fmt.Errorf("failed to build snapshot chain: %v", err)
	}

	fmt.Printf("Snapshot chain: %v\n", chain)

	// Step 1: Extract all files from the full snapshot (if it exists) to root/ directory
	for _, snapshotName := range chain {
		snapshotMetadata, err := loadSnapshotMetadata(saveDir, snapshotName)
		if err != nil {
			return fmt.Errorf("failed to load metadata for %s: %v", snapshotName, err)
		}

		if snapshotMetadata.SnapshotType == "full" {
			archivePath := filepath.Join(saveDir, snapshotName)
			fmt.Printf("Extracting ALL files from full snapshot: %s\n", snapshotName)

			// Extract all files from the full snapshot directly to extractDir
			err = extractAllFilesFromArchive(archivePath, extractDir)
			if err != nil {
				return fmt.Errorf("failed to extract full snapshot: %v", err)
			}
			break // Only process the first (oldest) full snapshot
		}
	}

	// Step 2: Apply incremental changes sequentially (from oldest to newest)
	for _, snapshotName := range chain {
		snapshotMetadata, err := loadSnapshotMetadata(saveDir, snapshotName)
		if err != nil {
			return fmt.Errorf("failed to load metadata for %s: %v", snapshotName, err)
		}

		if snapshotMetadata.SnapshotType == "incremental" {
			archivePath := filepath.Join(saveDir, snapshotName)
			fmt.Printf("Applying incremental changes from: %s (%d changed files, %d removed files)\n",
				snapshotName, len(snapshotMetadata.ChangedFiles), len(snapshotMetadata.RemovedFiles))

			// Extract only changed files from this incremental snapshot
			for _, filePath := range snapshotMetadata.ChangedFiles {
				err := extractFileFromArchive(archivePath, filePath, extractDir, rootDirectory)
				if err != nil {
					// Try to find the file in earlier snapshots in the chain
					found := false
					for _, earlierSnapshot := range chain {
						if earlierSnapshot == snapshotName {
							break // Stop at current snapshot
						}
						earlierArchivePath := filepath.Join(saveDir, earlierSnapshot)
						err2 := extractFileFromArchive(earlierArchivePath, filePath, extractDir, rootDirectory)
						if err2 == nil {
							found = true
							break
						}
					}
					if !found {
						fmt.Printf("Warning: failed to extract %s from any snapshot in chain: %v\n", filePath, err)
					}
				}
			}

			// Remove files that were deleted in this incremental snapshot
			for _, filePath := range snapshotMetadata.RemovedFiles {
				// Convert file path to local path
				localPath := filePath
				if strings.HasPrefix(filePath, rootDirectory) {
					localPath = strings.TrimPrefix(filePath, rootDirectory)
					localPath = strings.TrimPrefix(localPath, "/")
				}
				targetPath := filepath.Join(extractDir, localPath)

				// Remove the file if it exists
				if _, err := os.Stat(targetPath); err == nil {
					err := os.Remove(targetPath)
					if err != nil {
						fmt.Printf("Warning: failed to remove deleted file %s: %v\n", targetPath, err)
					} else {
						fmt.Printf("Removed deleted file: %s\n", targetPath)
					}
				}
			}
		}
	}

	fmt.Printf("Successfully extracted snapshot to: %s\n", extractDir)
	return nil
}

// extractAllFilesFromArchive extracts all files from a tar.gz archive
func extractAllFilesFromArchive(archivePath, extractDir string) error {
	// Open archive
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Extract all files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip directories (they will be created automatically)
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Create target path preserving directory structure
		// Files in archive are stored with relative paths, extract them directly
		targetPath := header.Name
		target := filepath.Join(extractDir, targetPath)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		// Create file
		outFile, err := os.Create(target)
		if err != nil {
			return err
		}

		// Copy file content
		_, err = io.Copy(outFile, tr)
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// listFilesInArchive lists all files in a tar.gz archive
func listFilesInArchive(archivePath string) ([]string, error) {
	var files []string

	// Open archive
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// List all files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Add file to list (skip directories)
		if header.Typeflag != tar.TypeDir {
			files = append(files, header.Name)
		}
	}

	return files, nil
}

// fileExistsInArchive checks if a file exists in a tar.gz archive
func fileExistsInArchive(archivePath, filePath, rootDirectory string) bool {
	// Open archive
	file, err := os.Open(archivePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return false
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Look for the specific file
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}

		// Check if this is the file we're looking for
		// Files in metadata have root directory prefix, but in archive they are stored without it
		expectedPath := filePath
		if strings.HasPrefix(filePath, rootDirectory) {
			expectedPath = strings.TrimPrefix(filePath, rootDirectory)
			expectedPath = strings.TrimPrefix(expectedPath, "/")
		}

		// Check if header name matches the expected path
		if header.Name == expectedPath {
			return true
		}
	}

	return false
}

// extractFileFromArchive extracts a specific file from a tar.gz archive
func extractFileFromArchive(archivePath, filePath, extractDir, rootDirectory string) error {
	// Open archive
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Look for the specific file
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Check if this is the file we're looking for
		// Files in metadata have root directory prefix, but in archive they are stored without it
		expectedPath := filePath
		if strings.HasPrefix(filePath, rootDirectory) {
			expectedPath = strings.TrimPrefix(filePath, rootDirectory)
			expectedPath = strings.TrimPrefix(expectedPath, "/")
		}

		// Check if header name matches the expected path
		matched := header.Name == expectedPath

		if matched {
			// Create target path preserving directory structure
			// Files in metadata have root directory prefix, but in archive they are stored without it
			// Extract directly to the path without adding root directory prefix
			targetPath := filePath
			if strings.HasPrefix(filePath, rootDirectory) {
				targetPath = strings.TrimPrefix(filePath, rootDirectory)
				targetPath = strings.TrimPrefix(targetPath, "/")
			}
			target := filepath.Join(extractDir, targetPath)

			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			defer outFile.Close()

			// Copy file content
			_, err = io.Copy(outFile, tr)
			return err
		}
	}

	return fmt.Errorf("file %s not found in archive", filePath)
}

// convertToArchivePath converts full file path to archive format
func convertToArchivePath(filePath, rootDirectory string) string {
	// Try different path formats that might be used in archives
	// 1. Original path (for full snapshots)
	// 2. Path without /root/ prefix
	// 3. Windows-style path

	variants := []string{
		filePath, // Original path
	}

	// If path starts with root directory, try without it
	if strings.HasPrefix(filePath, rootDirectory) {
		withoutRoot := strings.TrimPrefix(filePath, rootDirectory)
		withoutRoot = strings.TrimPrefix(withoutRoot, "/")
		variants = append(variants, withoutRoot)
		variants = append(variants, strings.ReplaceAll(withoutRoot, "/", "\\"))
	}

	// Try with different root prefixes
	if strings.HasPrefix(filePath, "/") {
		variants = append(variants, strings.TrimPrefix(filePath, "/"))
	}

	// Return the first variant (original) - the extraction function will try all variants
	return variants[0]
}

// loadLatestSnapshotHashes loads file hashes from the latest snapshot metadata
func loadLatestSnapshotHashes(saveDir string) map[string]string {
	fileHashes := make(map[string]string)

	// Find the latest snapshot archive file
	files, err := filepath.Glob(filepath.Join(saveDir, "backup_*.tar.gz"))
	if err != nil || len(files) == 0 {
		return fileHashes // Return empty map if no snapshots found
	}

	// Sort files by modification time to get the latest
	latestFile := files[0]
	for _, file := range files[1:] {
		info1, _ := os.Stat(latestFile)
		info2, _ := os.Stat(file)
		if info2.ModTime().After(info1.ModTime()) {
			latestFile = file
		}
	}

	// Load metadata from the latest snapshot
	archiveName := filepath.Base(latestFile)
	metadata, err := loadSnapshotMetadata(saveDir, archiveName)
	if err != nil {
		fmt.Printf("Warning: Failed to load latest snapshot metadata: %v\n", err)
		return fileHashes
	}

	return metadata.FileHashes
}

// cleanupTempFiles removes temporary downloaded files after archive creation
func cleanupTempFiles(saveDir string, filePaths []string) {
	// For full snapshots, remove the entire temp directory
	tempDir := filepath.Join(saveDir, "temp")
	if _, err := os.Stat(tempDir); err == nil {
		os.RemoveAll(tempDir) // Remove entire temp directory
		return
	}

	// For incremental snapshots, remove individual files
	for _, filePath := range filePaths {
		localFilePath := filepath.Join(saveDir, filePath)
		if err := os.Remove(localFilePath); err != nil {
			// Don't print error for individual file removal failures
			continue
		}
	}

	// Remove empty directories (optional - can be left for next backup)
	// This is a simple implementation that removes directories in reverse order
	for i := len(filePaths) - 1; i >= 0; i-- {
		dirPath := filepath.Dir(filepath.Join(saveDir, filePaths[i]))
		if dirPath != saveDir {
			os.Remove(dirPath) // Remove empty directory (ignoring errors)
		}
	}
}

// createFullSnapshotArchive creates an archive containing all files with proper path structure
func createFullSnapshotArchive(archivePath, saveDir string) error {
	outFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	downloadedDir := filepath.Join(saveDir, "temp")

	// Walk through all files in the downloaded directory
	err = filepath.Walk(downloadedDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		// Create relative path from the downloaded directory
		relPath, err := filepath.Rel(downloadedDir, file)
		if err != nil {
			return err
		}

		// Convert to forward slashes - files should be stored with relative paths from the root directory
		// No need to add /root/ prefix since Root Directory is already /root
		archivePath := filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(fi, archivePath)
		if err != nil {
			return err
		}
		header.Name = archivePath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	return err
}

// createIncrementalArchive creates an archive containing only the changed files
func createIncrementalArchive(archivePath, saveDir string, changedFiles []string) error {
	outFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, filePath := range changedFiles {
		// Get the local file path (preserve directory structure)
		// Remove root directory prefix from filePath
		localPath := filePath
		if strings.HasPrefix(filePath, "/root/") {
			localPath = strings.TrimPrefix(filePath, "/root/")
		}
		localFilePath := filepath.Join(saveDir, "temp", localPath)

		// Check if file exists locally
		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			continue // Skip if file doesn't exist locally
		}

		// Get file info
		fileInfo, err := os.Stat(localFilePath)
		if err != nil {
			continue
		}

		// Create tar header with relative path (without root directory prefix)
		archivePath := localPath
		header, err := tar.FileInfoHeader(fileInfo, archivePath)
		if err != nil {
			continue
		}
		// Use the relative path as the archive entry name (without root directory prefix)
		header.Name = archivePath

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			continue
		}

		// Write file content
		file, err := os.Open(localFilePath)
		if err != nil {
			continue
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		if err != nil {
			continue
		}
	}

	return nil
}

// saveSnapshotMetadata saves metadata for a snapshot
func saveSnapshotMetadata(saveDir, archiveName string, metadata SnapshotMetadata) error {
	metadataPath := filepath.Join(saveDir, strings.TrimSuffix(archiveName, ".tar.gz")+".meta")
	data, err := msgpack.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(metadataPath, data, 0644)
}

// loadSnapshotMetadata loads metadata for a snapshot
func loadSnapshotMetadata(saveDir, archiveName string) (*SnapshotMetadata, error) {
	metadataPath := filepath.Join(saveDir, strings.TrimSuffix(archiveName, ".tar.gz")+".meta")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata SnapshotMetadata
	err = msgpack.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

// findLatestFullSnapshot finds the latest full snapshot for a project
func findLatestFullSnapshot(saveDir string) (string, error) {
	files, err := os.ReadDir(saveDir)
	if err != nil {
		return "", err
	}

	var fullSnapshots []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
			metadata, err := loadSnapshotMetadata(saveDir, file.Name())
			if err == nil && metadata.SnapshotType == "full" {
				fullSnapshots = append(fullSnapshots, file.Name())
			}
		}
	}

	if len(fullSnapshots) == 0 {
		return "", fmt.Errorf("no full snapshots found")
	}

	// Sort by timestamp (latest first)
	latest := fullSnapshots[0]
	for _, snapshot := range fullSnapshots[1:] {
		meta1, _ := loadSnapshotMetadata(saveDir, latest)
		meta2, _ := loadSnapshotMetadata(saveDir, snapshot)
		if meta2.Timestamp.After(meta1.Timestamp) {
			latest = snapshot
		}
	}

	return latest, nil
}

// buildSnapshotChain builds the chain of snapshots needed to restore a specific snapshot
func buildSnapshotChain(saveDir, targetSnapshot string) ([]string, error) {
	var chain []string
	current := targetSnapshot

	for {
		chain = append([]string{current}, chain...) // prepend to maintain order

		metadata, err := loadSnapshotMetadata(saveDir, current)
		if err != nil {
			return nil, fmt.Errorf("failed to load metadata for %s: %v", current, err)
		}

		if metadata.SnapshotType == "full" || metadata.ParentSnapshot == "" {
			break
		}

		current = metadata.ParentSnapshot
	}

	return chain, nil
}

func SnapshotComparison(arguments []string) {
	// Проверяем аргументы
	if len(arguments) < 3 {
		fmt.Println("Please provide project ID and two snapshot IDs. Usage: snp -sc <project ID> <snapshot ID 1> <snapshot ID 2>")
		return
	}

	// Парсим ID проекта
	projectIDStr := arguments[0]
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return
	}

	// Парсим ID первого снимка
	snapshot1IDStr := arguments[1]
	snapshot1ID, err := strconv.Atoi(snapshot1IDStr)
	if err != nil {
		fmt.Println("Invalid snapshot ID 1 format:", err)
		return
	}

	// Парсим ID второго снимка
	snapshot2IDStr := arguments[2]
	snapshot2ID, err := strconv.Atoi(snapshot2IDStr)
	if err != nil {
		fmt.Println("Invalid snapshot ID 2 format:", err)
		return
	}

	// Получаем конфигурации
	var dataBin = getCachedConfigurations()

	// Проверяем существование проекта
	if projectID < 1 || projectID > len(dataBin) {
		fmt.Printf("Project with ID %d not found.\n", projectID)
		return
	}

	config := dataBin[projectID-1]
	saveDir := config.SaveDirectory

	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	// Ищем все архивы в директории сохранения
	files, err := os.ReadDir(saveDir)
	if err != nil {
		fmt.Printf("Failed to read save directory: %v\n", err)
		return
	}

	// Собираем список архивов
	var archiveFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_") && strings.HasSuffix(file.Name(), ".tar.gz") {
			archiveFiles = append(archiveFiles, file.Name())
		}
	}

	// Проверяем существование снимков
	if snapshot1ID < 1 || snapshot1ID > len(archiveFiles) {
		fmt.Printf("Snapshot 1 with ID %d not found. Available snapshots: %d\n", snapshot1ID, len(archiveFiles))
		return
	}

	if snapshot2ID < 1 || snapshot2ID > len(archiveFiles) {
		fmt.Printf("Snapshot 2 with ID %d not found. Available snapshots: %d\n", snapshot2ID, len(archiveFiles))
		return
	}

	selectedArchive1 := archiveFiles[snapshot1ID-1]
	selectedArchive2 := archiveFiles[snapshot2ID-1]

	fmt.Printf("Comparing snapshots:\n")
	fmt.Printf("  Snapshot 1: %s\n", selectedArchive1)
	fmt.Printf("  Snapshot 2: %s\n", selectedArchive2)
	fmt.Println(strings.Repeat("-", 50))

	// Получаем полные наборы файлов для обоих снимков
	files1, err := getCompleteFileSet(saveDir, selectedArchive1)
	if err != nil {
		fmt.Printf("Failed to get file set for snapshot 1: %v\n", err)
		return
	}

	files2, err := getCompleteFileSet(saveDir, selectedArchive2)
	if err != nil {
		fmt.Printf("Failed to get file set for snapshot 2: %v\n", err)
		return
	}

	// Сравниваем файлы
	compareFileSets(files1, files2, selectedArchive1, selectedArchive2)
}

// getCompleteFileSet получает полный набор файлов для снимка (включая все файлы из цепочки)
func getCompleteFileSet(saveDir, archiveName string) (map[string]string, error) {
	// Строим цепочку снимков
	chain, err := buildSnapshotChain(saveDir, archiveName)
	if err != nil {
		return nil, err
	}

	// Создаем карту для хранения файлов и их хешей
	fileSet := make(map[string]string)

	// Обрабатываем каждый снимок в цепочке
	for _, snapshotName := range chain {
		metadata, err := loadSnapshotMetadata(saveDir, snapshotName)
		if err != nil {
			return nil, fmt.Errorf("failed to load metadata for %s: %v", snapshotName, err)
		}

		// Для полного снимка - добавляем ВСЕ файлы из FileHashes
		// Для инкрементального - добавляем только измененные файлы
		if metadata.SnapshotType == "full" {
			// Полный снимок содержит все файлы проекта
			for filePath, hash := range metadata.FileHashes {
				fileSet[filePath] = hash
			}
		} else {
			// Инкрементальный снимок содержит только измененные файлы
			for _, filePath := range metadata.ChangedFiles {
				if hash, exists := metadata.FileHashes[filePath]; exists {
					fileSet[filePath] = hash
				}
			}
		}
	}

	return fileSet, nil
}

// compareFileSets сравнивает два набора файлов и показывает различия
func compareFileSets(files1, files2 map[string]string, name1, name2 string) {
	// Находим различия
	var added, removed, changed []string

	// Проверяем файлы из первого снимка
	for filePath, hash1 := range files1 {
		if hash2, exists := files2[filePath]; !exists {
			// Файл удален
			removed = append(removed, filePath)
		} else if hash1 != hash2 {
			// Файл изменен
			changed = append(changed, filePath)
		}
	}

	// Проверяем файлы из второго снимка
	for filePath := range files2 {
		if _, exists := files1[filePath]; !exists {
			// Файл добавлен
			added = append(added, filePath)
		}
	}

	// Выводим результаты
	fmt.Printf("Comparison Results:\n\n")

	if len(added) > 0 {
		fmt.Printf("📁 Added files (%d):\n", len(added))
		for _, file := range added {
			fmt.Printf("  + %s\n", file)
		}
		fmt.Println()
	}

	if len(removed) > 0 {
		fmt.Printf("🗑️  Removed files (%d):\n", len(removed))
		for _, file := range removed {
			fmt.Printf("  - %s\n", file)
		}
		fmt.Println()
	}

	if len(changed) > 0 {
		fmt.Printf("📝 Changed files (%d):\n", len(changed))
		for _, file := range changed {
			fmt.Printf("  ~ %s\n", file)
		}
		fmt.Println()
	}

	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		fmt.Println("✅ No differences found - snapshots are identical")
	} else {
		fmt.Printf("📊 Summary: %d added, %d removed, %d changed\n", len(added), len(removed), len(changed))
	}
}

func pressCommand(flag string, arguments []string) {
	if cmd, err := commandProcessing(flag, arguments); err != nil {

	} else {
		fmt.Printf("> no such command `%s`\n", cmd)
	}

}

func commandProcessing(input string, arguments []string) (string, error) {
	// check for incorrectly specified order of flags
	input = checkFlags(input)
	// calling the necessary function determined by the passed flags
	if _, ok := cmd[input]; ok {
		cmd[input].(func(arguments []string))(arguments)
		return input, fmt.Errorf("total errors: %d", 0)
	} else {
		return input, nil
	}
}

func checkFlags(flag string) string {

	for name := range cmd {
		if len(name) == len(flag) {
			hitCounter := 0
			for _, letterName := range name {
				for _, letterInput := range flag {
					if letterName == letterInput {
						hitCounter++
					}
				}
			}

			if hitCounter == len(name) {
				return name
			}
		}
	}

	return flag
}

func Initialization() {
	arrayArguments := os.Args[1:]

	if len(arrayArguments) == 0 {
		fmt.Println("Command not recognized.\n\rTo display help, enter: snp -h")
		return
	}

	// define flags
	flag := strings.Replace(arrayArguments[0], "-", "", 1)

	pressCommand(flag, arrayArguments[1:])
}
