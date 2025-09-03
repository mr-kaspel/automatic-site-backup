package handlers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mr-kaspel/automatic-site-backup.git/internal/storages"
	"github.com/mr-kaspel/automatic-site-backup.git/internal/utils"
	"github.com/vmihailenco/msgpack/v5"
)

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
		{"backupscript", "Enter the path to the backup script on the server (e.g., db_backup.php) or leave empty:"},
		{"rootdirectory", "Enter the full path to the project root directory on the remote server:"},
		{"savedirectory", "Enter the address of the local directory where you want to save the project snapshot:"},
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
		BackupScript:  answersQuestions["backupscript"],
		RootDirectory: answersQuestions["rootdirectory"],
		SaveDirectory: answersQuestions["savedirectory"],
	}

	// reading current configurations from a file
	var existingConfigs = data.GetFileConfiguration()

	// adding a new configuration
	existingConfigs = append(existingConfigs, newConfig)

	// save all configurations
	data.SaveConfigurations(existingConfigs)
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
}

func configurationDataOutput(arguments []string) {
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

	// output each configuration on a new line
	for i, config := range dataBin {
		fmt.Printf("\033[38;2;31;111;235m Configuration %d:\033[0m\n", i+1)
		fmt.Printf("\tName: %s\r\n\tProtocol: %s\r\n\tPort: %s\r\n\tHost: %s\r\n\tLogin: %s\r\n\tPassword: %s\r\n\tDB Login: %s\r\n\tDB Password: %s\r\n\tDB Type: %s\r\n\tDB Host: %s\r\n\tDB Port: %s\r\n\tDB Database: %s\r\n\tBackup Script: %s\r\n\tRoot Directory: <%s>\r\n\tSave Directory: <%s>\n",
			config.Name, config.Protocol, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.DBType, config.DBHost, config.DBPort, config.DBDatabase, config.BackupScript, config.RootDirectory, config.SaveDirectory)
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

	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

	// check for the presence of a configuration with a given id
	if id < 0 || id > len(dataBin) {
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
	data.SaveConfigurations(dataBin)

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

	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

	// search configuration by ID
	if id < 0 || id > len(dataBin) {
		fmt.Printf("Configuration with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]

	fmt.Printf("Configuration %d:\n", id)
	fmt.Printf("\tName: %s\r\n\tProtocol: %s\r\n\tPort: %s\r\n\tHost: %s\r\n\tLogin: %s\r\n\tPassword: %s\r\n\tDB Login: %s\r\n\tDB Password: %s\r\n\tDB Type: %s\r\n\tDB Host: %s\r\n\tDB Port: %s\r\n\tDB Database: %s\r\n\tBackup Script: %s\r\n\tRoot Directory: <%s>\r\n\tSave Directory: <%s>\n",
		config.Name, config.Protocol, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.DBType, config.DBHost, config.DBPort, config.DBDatabase, config.BackupScript, config.RootDirectory, config.SaveDirectory)

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
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

	// search configuration by ID
	if id < 0 || id > len(dataBin) {
		fmt.Printf("Configuration with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]
	saveDir := config.SaveDirectory
	if saveDir == "" {
		fmt.Println("Save directory is not specified in the configuration.")
		return
	}

	// create a path to the hash file for a specific project
	hashFilePath := filepath.Join(saveDir, fmt.Sprintf(".filehashes_%s", config.Name))
	fileHashes := make(map[string]string)

	// preparing a client connection (FTP/SFTP)
	var client utils.FTPClient // interface for working with FTP/SFTP
	if config.Protocol == "FTP" {
		client, err = utils.NewFTPClient(config.Host, config.Login, config.Password, config.Port)
		if err != nil {
			fmt.Println("Invalid FTP:", err)
			return
		}
	} else if config.Protocol == "SFTP" {
		client, err = utils.NewSFTPClient(config.Host, config.Login, config.Password, config.Port)
		if err != nil {
			fmt.Println("Invalid SFTP:", err)
			return
		}
	} else {
		fmt.Println("Unsupported protocol for connection.")
		return
	}
	defer client.Close()

	// reading existing hashes
	if _, err := os.Stat(hashFilePath); err == nil {
		hashData, err := os.ReadFile(hashFilePath)
		if err == nil {
			_ = msgpack.Unmarshal(hashData, &fileHashes)
		}
	}

	if config.RootDirectory == "" {
		currentDir, _ := client.CurrentDir()
		config.RootDirectory = currentDir
	}

	// getting a list of files from the server
	files, err := client.ListFiles(config.RootDirectory)
	if err != nil {
		fmt.Println("Failed to list files on the server:", err)
		return
	}

	// list for updated hashes and files to download
	updatedHashes := make(map[string]string)
	filesToDownload := []string{}

	// compare files
	for _, file := range files {
		filePath := filepath.Join(config.RootDirectory, file.Name)

		// current file hash
		fileHash, err := client.HashFile(filePath)
		if err != nil {
			fmt.Printf("Failed to calculate hash for file %s: %v\n", filePath, err)
			continue
		}

		// checking for changes
		if existingHash, exists := fileHashes[filePath]; !exists || existingHash != fileHash {
			filesToDownload = append(filesToDownload, filePath)
		}

		// saving a new hash
		updatedHashes[filePath] = fileHash
	}

	// removing missing files from hashes
	for path := range fileHashes {
		if _, exists := updatedHashes[path]; !exists {
			fmt.Printf("File %s has been removed from the server.\n", path)
		}
	}

	// downloading modified files
	for _, filePath := range filesToDownload {
		localFilePath := filepath.Join(saveDir, filepath.Base(filePath))

		err := os.MkdirAll(filepath.Dir(localFilePath), 0755)
		if err != nil {
			fmt.Printf("Failed to create directory for file %s: %v\n", localFilePath, err)
			continue
		}

		err = client.DownloadFile(filePath, localFilePath)
		if err != nil {
			fmt.Printf("Failed to download file %s: %v\n", filePath, err)
		} else {
			fmt.Printf("Downloaded: %s\n", filePath)
		}
	}

	// updating the hash file
	hashData, err := msgpack.Marshal(updatedHashes)
	if err == nil {
		err = os.WriteFile(hashFilePath, hashData, 0644)
		if err != nil {
			fmt.Println("Failed to update hash file:", err)
		}
	}

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

	// archiving
	archiveName := fmt.Sprintf("backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	archivePath := filepath.Join(saveDir, archiveName)
	err = utils.CreateTarGz(archivePath, saveDir)
	if err != nil {
		fmt.Println("Failed to create archive:", err)
	} else {
		fmt.Printf("Backup archive created: %s\n", archivePath)
	}
}

func snapshotAll(arguments []string) {
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

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
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

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

			// Парсим дату из имени файла (формат: backup_20060102_150405.tar.gz)
			fileName := file.Name()
			if len(fileName) >= 21 { // backup_ + 15 символов даты + .tar.gz
				dateStr := fileName[7:21] // извлекаем 20060102_150405
				if parsedTime, err := time.Parse("20060102_150405", dateStr); err == nil {
					fmt.Printf("ID: %d | Date: %s | Size: %.2f MB\n",
						snapshotCount,
						parsedTime.Format("2006-01-02 15:04:05"),
						float64(fileInfo.Size())/(1024*1024))
				} else {
					fmt.Printf("ID: %d | File: %s | Size: %.2f MB\n",
						snapshotCount,
						fileName,
						float64(fileInfo.Size())/(1024*1024))
				}
			} else {
				fmt.Printf("ID: %d | File: %s | Size: %.2f MB\n",
					snapshotCount,
					fileName,
					float64(fileInfo.Size())/(1024*1024))
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
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

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

	// Извлекаем архив
	err = utils.ExtractTarGz(archivePath, extractDir)
	if err != nil {
		fmt.Printf("Failed to extract archive: %v\n", err)
		// Удаляем созданную директорию в случае ошибки
		os.RemoveAll(extractDir)
		return
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
	var data storages.Configuration
	var dataBin = data.GetFileConfiguration()

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

	// Создаем временную директорию для извлечения
	tempDir := filepath.Join(saveDir, fmt.Sprintf("temp_extract_%d", time.Now().Unix()))

	fmt.Printf("Extracting files from snapshot '%s'...\n", selectedArchive)

	// Создаем временную директорию
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create temporary directory: %v\n", err)
		return
	}

	// Извлекаем архив
	err = utils.ExtractTarGz(archivePath, tempDir)
	if err != nil {
		fmt.Printf("Failed to extract archive: %v\n", err)
		os.RemoveAll(tempDir)
		return
	}

	// Ищем целевой файл/директорию
	sourcePath := filepath.Join(tempDir, targetPath)

	// Проверяем существование
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		fmt.Printf("Path '%s' not found in snapshot.\n", targetPath)
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

func SnapshotComparison(arguments []string) {
	/*
		To compare two snapshots, type 'snp -sc *project ID* *snapshot ID* *snapshot ID'
	*/
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
