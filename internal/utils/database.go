package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mr-kaspel/automatic-site-backup.git/internal/storages"
)

// DatabaseBackupResult represents the result of a database backup
type DatabaseBackupResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Data      struct {
		Dump        string `json:"dump"`
		TablesCount int    `json:"tables_count"`
		Format      string `json:"format"`
	} `json:"data,omitempty"`
}

// BackupDatabase performs database backup using PHP script
func BackupDatabase(config storages.Configuration, client FTPClient) error {
	// Check if database configuration is provided
	if config.DBType == "" {
		return fmt.Errorf("database type not specified")
	}

	// Script path is now fixed - no need to check config.BackupScript

	fmt.Printf("Starting database backup for project: %s\n", config.Name)
	fmt.Printf("Database type: %s\n", config.DBType)

	// Check if script already exists on server
	scriptExists, err := CheckBackupScriptExists(config, client)
	if err != nil {
		fmt.Printf("Warning: Could not check if backup script exists: %v\n", err)
	}

	// Upload backup script to server if it doesn't exist
	if !scriptExists {
		err = UploadBackupScript(config, client)
		if err != nil {
			return fmt.Errorf("failed to upload backup script: %v", err)
		}
	} else {
		fmt.Printf("Backup script already exists on server, skipping upload\n")
	}

	// Ensure script is deleted after backup (even if backup fails)
	defer func() {
		if deleteErr := DeleteBackupScript(config, client); deleteErr != nil {
			fmt.Printf("Warning: Failed to delete backup script: %v\n", deleteErr)
		}
	}()

	// Prepare backup script URL
	scriptURL, err := buildBackupScriptURL(config)
	if err != nil {
		return fmt.Errorf("failed to build backup script URL: %v", err)
	}

	// Call PHP script to get database dump
	result, err := callBackupScript(scriptURL)
	if err != nil {
		return fmt.Errorf("failed to call backup script: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("backup script failed: %s", result.Message)
	}

	// Save database dump to file
	dumpFileName := fmt.Sprintf("database_backup_%s_%s.sql",
		config.Name,
		time.Now().Format("20060102_150405"))

	dumpFilePath := filepath.Join(config.SaveDirectory, dumpFileName)

	err = saveDatabaseDump(dumpFilePath, result.Data.Dump)
	if err != nil {
		return fmt.Errorf("failed to save database dump: %v", err)
	}

	fmt.Printf("Database backup completed successfully\n")
	fmt.Printf("Tables backed up: %d\n", result.Data.TablesCount)
	fmt.Printf("Dump saved to: %s\n", dumpFilePath)

	return nil
}

// buildBackupScriptURL constructs the URL for the backup script
func buildBackupScriptURL(config storages.Configuration) (string, error) {
	// Determine the base URL for the script
	var baseURL string
	if config.Protocol == "FTP" {
		baseURL = fmt.Sprintf("http://%s", config.Host)
	} else if config.Protocol == "SFTP" {
		baseURL = fmt.Sprintf("http://%s", config.Host)
	} else {
		return "", fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}

	// Build the script URL - using fixed script name
	scriptURL := fmt.Sprintf("%s/db_backup.php", baseURL)

	// Parse URL to add query parameters
	u, err := url.Parse(scriptURL)
	if err != nil {
		return "", err
	}

	// Add database parameters
	params := u.Query()
	params.Set("type", config.DBType)

	switch config.DBType {
	case "mysql", "postgresql":
		params.Set("host", config.DBHost)
		params.Set("port", config.DBPort)
		params.Set("user", config.DBlogin)
		params.Set("pass", config.DBpassword)
		params.Set("db", config.DBDatabase)
	case "sqlite":
		// For SQLite, we need to construct the path
		sqlitePath := filepath.Join(config.RootDirectory, config.DBDatabase)
		params.Set("path", sqlitePath)
	default:
		return "", fmt.Errorf("unsupported database type: %s", config.DBType)
	}

	u.RawQuery = params.Encode()
	return u.String(), nil
}

// callBackupScript makes HTTP request to the backup script
func callBackupScript(scriptURL string) (*DatabaseBackupResult, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make GET request
	resp, err := client.Get(scriptURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backup script returned status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result DatabaseBackupResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse backup script response: %v", err)
	}

	return &result, nil
}

// saveDatabaseDump saves the database dump to a file
func saveDatabaseDump(filePath string, dump string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Write dump to file
	err = os.WriteFile(filePath, []byte(dump), 0644)
	if err != nil {
		return err
	}

	return nil
}

// UploadBackupScript uploads the PHP backup script to the server
func UploadBackupScript(config storages.Configuration, client FTPClient) error {
	// Path to the local backup script
	localScriptPath := "scripts/db_backup.php"

	// Check if local script exists
	if _, err := os.Stat(localScriptPath); os.IsNotExist(err) {
		return fmt.Errorf("backup script not found: %s", localScriptPath)
	}

	// Use fixed script name in root directory
	remoteScriptPath := filepath.Join(config.RootDirectory, "db_backup.php")

	fmt.Printf("Uploading backup script to: %s\n", remoteScriptPath)

	// Upload the script using UploadFile method
	err := client.UploadFile(localScriptPath, remoteScriptPath)
	if err != nil {
		return fmt.Errorf("failed to upload backup script: %v", err)
	}

	fmt.Printf("Backup script uploaded successfully\n")
	return nil
}

// DeleteBackupScript removes the PHP backup script from the server
func DeleteBackupScript(config storages.Configuration, client FTPClient) error {
	// Use fixed script name in root directory
	remoteScriptPath := filepath.Join(config.RootDirectory, "db_backup.php")

	fmt.Printf("Deleting backup script from: %s\n", remoteScriptPath)

	// Delete the script using DeleteFile method
	err := client.DeleteFile(remoteScriptPath)
	if err != nil {
		return fmt.Errorf("failed to delete backup script: %v", err)
	}

	fmt.Printf("Backup script deleted successfully\n")
	return nil
}

// CheckBackupScriptExists checks if the backup script already exists on the server
func CheckBackupScriptExists(config storages.Configuration, client FTPClient) (bool, error) {
	// Use fixed script name in root directory
	remoteScriptPath := filepath.Join(config.RootDirectory, "db_backup.php")

	// Try to get file info to check if it exists
	_, err := client.HashFile(remoteScriptPath)
	if err != nil {
		// If we can't get hash, file probably doesn't exist
		return false, nil
	}

	return true, nil
}

// ValidateDatabaseConfig validates database configuration
func ValidateDatabaseConfig(config storages.Configuration) error {
	if config.DBType == "" {
		return nil // Database backup is optional
	}

	// Validate database type
	validTypes := []string{"mysql", "postgresql", "sqlite"}
	validType := false
	for _, t := range validTypes {
		if config.DBType == t {
			validType = true
			break
		}
	}

	if !validType {
		return fmt.Errorf("invalid database type: %s. Supported types: %s",
			config.DBType, strings.Join(validTypes, ", "))
	}

	// Validate required fields based on database type
	switch config.DBType {
	case "mysql", "postgresql":
		if config.DBHost == "" {
			return fmt.Errorf("database host is required for %s", config.DBType)
		}
		if config.DBPort == "" {
			return fmt.Errorf("database port is required for %s", config.DBType)
		}
		if config.DBlogin == "" {
			return fmt.Errorf("database login is required for %s", config.DBType)
		}
		if config.DBDatabase == "" {
			return fmt.Errorf("database name is required for %s", config.DBType)
		}
	case "sqlite":
		if config.DBDatabase == "" {
			return fmt.Errorf("database file path is required for SQLite")
		}
	}

	// Script path is now fixed - no validation needed

	return nil
}

// GetDatabaseInfo returns information about the database configuration
func GetDatabaseInfo(config storages.Configuration) string {
	if config.DBType == "" {
		return "Database backup: Not configured"
	}

	var info strings.Builder
	info.WriteString(fmt.Sprintf("Database backup: %s\n", config.DBType))

	switch config.DBType {
	case "mysql", "postgresql":
		info.WriteString(fmt.Sprintf("  Host: %s:%s\n", config.DBHost, config.DBPort))
		info.WriteString(fmt.Sprintf("  Database: %s\n", config.DBDatabase))
		info.WriteString(fmt.Sprintf("  User: %s\n", config.DBlogin))
	case "sqlite":
		info.WriteString(fmt.Sprintf("  File: %s\n", config.DBDatabase))
	}

	info.WriteString("  Script: db_backup.php (auto-uploaded)\n")

	return info.String()
}
