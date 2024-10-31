package handlers

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mr-kaspel/automatic-site-backup.git/internal/storages"
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
	// argument validation utils
	// ...
	if len(arguments) == 0 {
		fmt.Println("Not all parameters are listed, please refer to the help")
		return
	}

	// pass all arguments in one request
	// ...

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

	// saving data
	data.SavingReceivedData(answersQuestions)
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
		fmt.Printf("Configuration %d:\n", i+1)
		fmt.Printf("Name: %s, Port: %s, Host: %s, Login: %s, Password: %s, DB Login: %s, DB Password: %s, Root Directory: %s, Save Directory: %s\n",
			config.Name, config.Port, config.Host, config.Login, config.Password, config.DBlogin, config.DBpassword, config.RootDirectory, config.SaveDirectory)
		fmt.Println() // empty line to separate configurations
	}
}

func delet(arguments []string) {
	/*
		To remove the project from the list with all previously created snapshots, enter `snt -d *project ID*
	*/
}

func settings(arguments []string) {
	/*
		To get a list of all settings for a specific project, type `snt -s *project ID*`
	*/
}

func snapshot(arguments []string) {
	/*
		To create a snapshot of a specific project, type `snt -sn *project ID*`
	*/
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

	// define flags
	flag := strings.Replace(arrayArguments[0], "-", "", 1)

	pressCommand(flag, arrayArguments[1:])

	// check and create config file
	storages.CreatingConfigurationFile()
}
