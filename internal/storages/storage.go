package storages

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
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

func (c *Configuration) TransformationReceivedData(input []string) map[string]string {
	var mapData map[string]string

	jsonData, err := json.Marshal(c)

	if err != nil {
		panic(err)
	}

	json.Unmarshal(jsonData, &mapData)
	mappedToSlice := reflect.ValueOf(mapData).MapKeys()

	for id, key := range mappedToSlice {
		mapData[key.Interface().(string)] = input[id]
	}

	return mapData
}

func (c *Configuration) SavingReceivedData(m map[string]string) {
	var oldArray []Configuration

	// get the current data from a file, there may be a problem with the amount of data
	// ...
	oldData := ReadConfigurationFile()
	json.Unmarshal(oldData, &oldArray)

	// converting new data
	var newArray Configuration

	jsonData, err := json.Marshal(m)

	if err != nil {
		panic(err)
	}

	json.Unmarshal(jsonData, &newArray)

	oldArray = append(oldArray, newArray)

	// oldArray convert to json
	// overwrite file
	// ...
}
