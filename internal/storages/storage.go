package storages

import (
	"io/ioutil"
	"os"
)

const сonfig = "config.json"

type Configuration struct {
	Name          string `json:"name"`
	Port          string `json:"port"`
	Host          string `json:"host"`
	Login         string `json:"login"`
	Password      string `json:"password"`
	DBlogin       string `json:"bdlogin"`
	DBpassword    string `json:"bdpassword"`
	BackupDate    string `json: "backupdate"`
	RootDirectory string `json: "directory"`
	SaveDirectory string `json: "savedirectory"`
}

func ReadConfigurationFile() []byte {
	file, err := os.OpenFile(сonfig, os.O_RDONLY, 0777)

	if err != nil {
		panic(err)
	}

	jsonStr, err := ioutil.ReadAll(file)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	return jsonStr
}

func CreatingConfigurationFile() {
	if _, err := os.Stat(сonfig); err != nil {
		if os.IsNotExist(err) {
			_, err := os.Create(сonfig)

			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
}
