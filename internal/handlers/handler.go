package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mr-kaspel/automatic-site-backup.git/internal/storages"
)

var cmd = map[string]interface{}{
	"help":     help, // 1
	"h":        help,
	"a":        add, // 2 // -a site.ru
	"add":      add,
	"e":        edit, // 3 // -e *project ID*
	"edit":     edit,
	"list":     configurationDataOutput, // 4 // data output from the configuration file
	"l":        configurationDataOutput,
	"d":        delet, // 5 // -d *project ID*
	"delet":    delet,
	"s":        settings, // 6 // s- *project ID*
	"settings": settings,
	"sn":       snapshot, // 7 // -sn *project ID*
	"snapshot": snapshot,
	"sna":      snapshotAll,  // 8
	"ls":       listSnapshot, // 9 // -ls *project ID*
	"gs":       getSnapshot,  // 10 // -gs *snapshot ID*
}

func help(argument []string) {
	fmt.Println(Logo)
	fmt.Println(CommandList)
}

func add(argument []string) {
	/*
		To add a project, type `snt -a site.ru`
		Next, you will be prompted to enter:
		    - stap project name;
		    - stap port (21 FTP, 22 SFTP);
		    - stap server address;
		    - stap login S\\FTP;
		    - stap password S\\FTP;
		    - stap bd login;
		    - stap bd password;
		    - stap backup frequency in cron task syntax.
		    - stap root directory string `json: "directory"`
		    - stap save directory string `json: "savedirectory"`
	*/
}

func edit(argument []string) {
	/*
		To change project settings, type `snt -e *project ID* *data field name* *new value*`. Settings can be changed in the config.json file.
	*/
}

func configurationDataOutput(argument []string) {
	var dataJson []storages.Configuration

	jsonStr := storages.ReadConfigurationFile()

	//if the file is empty we request project data
	if len(jsonStr) != 0 {
		json.Unmarshal(jsonStr, &dataJson)

		fmt.Println("\x1b[32mAdded projects:\x1b[0m")
		for id, project := range dataJson {
			fmt.Printf("%d. %s backup frequency '%s'\n", id+1, project.Name, project.BackupDate)
		}
	}
}

func delet(argument []string) {
	/*
		To remove the project from the list with all previously created snapshots, enter `snt -d *project ID*
	*/
}

func settings(argument []string) {
	/*
		To get a list of all settings for a specific project, type `snt -s *project ID*`
	*/
}

func snapshot(argument []string) {
	/*
		To create a snapshot of a specific project, type `snt -sn *project ID*`
	*/
}

func snapshotAll(argument []string) {
	/*
		To create snapshots of all added projects, type `snt -sna`
	*/
}

func listSnapshot(argument []string) {
	/*
		To get a list of available backups for a specific project, type `snt -ls *project ID*`
	*/
}

func getSnapshot(argument []string) {
	/*
		To create a snapshot of a specific project, type `snt -gs *snapshot ID*`
	*/
}

func PressCommand(flag string, argument []string) {
	if cmd, err := CommandProcessing(flag, argument); err != nil {

	} else {
		fmt.Printf("> no such command `%s`\n", cmd)
	}

}

func CommandProcessing(input string, argument []string) (string, error) {
	if _, ok := cmd[input]; ok {
		cmd[input].(func(argument []string))(argument)
		return input, fmt.Errorf("total errors: %d", 0)
	} else {
		return input, nil
	}
}

func Initialization() {
	arrayArguments := os.Args[1:]

	// define flags
	flag := strings.Replace(arrayArguments[0], "-", "", 1)

	PressCommand(flag, arrayArguments[1:])

	// check and create config file
	storages.CreatingConfigurationFile()
}
