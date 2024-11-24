package handlers

import (
	"bufio"
	"fmt"
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
		{"port", "Enter the port to connect to the remote server (21 FTP, 22 SFTP):"},
		{"host", "Enter the address of the remote server:"},
		{"login", "Enter the login from the account to connect to the remote server:"},
		{"password", "Enter the password for the account to connect to the remote server:"},
		{"dblogin", "Enter the login from the account to connect to the database:"},
		{"dbpassword", "Enter the password for the account to connect to the database:"},
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
		Port:          answersQuestions["port"],
		Host:          answersQuestions["host"],
		Login:         answersQuestions["login"],
		Password:      answersQuestions["password"],
		DBlogin:       answersQuestions["dblogin"],
		DBpassword:    answersQuestions["dbpassword"],
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
		fmt.Printf("\tName: %s\r\n\tPort: %s\r\n\tHost: %s\r\n\tLogin: %s\r\n\tPassword: %s\r\n\tDB Login: %s\r\n\tDB Password: %s\r\n\tRoot Directory: <%s>\r\n\tSave Directory: <%s>\n",
			config.Name, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.RootDirectory, config.SaveDirectory)
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
	if id <= 0 || id > len(dataBin) {
		fmt.Printf("Configuration with ID %d not found.\n", id)
		return
	}

	config := dataBin[id-1]

	fmt.Printf("Configuration %d:\n", id)
	fmt.Printf("\tName: %s\r\n\tPort: %s\r\n\tHost: %s\r\n\tLogin: %s\r\n\tPassword: %s\r\n\tDB Login: %s\r\n\tDB Password: %s\r\n\tRoot Directory: <%s>\r\n\tSave Directory: <%s>\n",
		config.Name, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.RootDirectory, config.SaveDirectory)

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
	if id <= 0 || id > len(dataBin) {
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
	if config.Port == "21" {
		client, err = utils.NewFTPClient(config.Host, config.Login, config.Password)
		if err != nil {
			fmt.Println("Invalid FTP:", err)
			return
		}
	} else if config.Port == "22" {
		client, err = utils.NewSFTPClient(config.Host, config.Login, config.Password)
		if err != nil {
			fmt.Println("Invalid SFTP:", err)
			return
		}
	} else {
		fmt.Println("Unsupported port for connection.")
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
		// localFilePath := filepath.Join(saveDir, file.Name)

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
		err := client.DownloadFile(filePath, localFilePath)
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
	/*
		To create snapshots of all added projects, type `snt -sna`
	*/
}

func listSnapshot(arguments []string) {
	/*
		To get a list of available backups for a specific project, type `snt -ls *project ID*`
	*/
}

func getSnapshot(arguments []string) {
	/*
		To create a snapshot of a specific project, type `snt -gs *snapshot ID*`
	*/
}

func getSnapshotFiles(arguments []string) {
	/*
		To get the files or directory from the selected snapshot, type 'snp -gsf *project ID* *snapshot ID* *directory*'
	*/
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

	for name, _ := range cmd {
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
