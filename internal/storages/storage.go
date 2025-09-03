package storages

import (
	"fmt"
	"os"
	"strconv"

	"github.com/vmihailenco/msgpack/v5"
)

const сonfig = "config.json"

type Configuration struct {
	Name          string `msgpack:"name"`
	Protocol      string `msgpack:"protocol"`
	Port          string `msgpack:"port"`
	Host          string `msgpack:"host"`
	Login         string `msgpack:"login"`
	Password      string `msgpack:"password"`
	DBlogin       string `msgpack:"dblogin"`
	DBpassword    string `msgpack:"dbpassword"`
	DBType        string `msgpack:"dbtype"`     // mysql, postgresql, sqlite
	DBHost        string `msgpack:"dbhost"`     // хост БД (обычно localhost)
	DBPort        string `msgpack:"dbport"`     // порт БД
	DBDatabase    string `msgpack:"dbdatabase"` // имя БД
	RootDirectory string `msgpack:"rootdirectory"`
	SaveDirectory string `msgpack:"savedirectory"`
	MaxThreads    string `msgpack:"maxthreads"` // максимальное количество потоков (по умолчанию 4)
}

func (c *Configuration) SaveConfigurations(configs []Configuration) {
	const configDir = "sites"
	const configFile = configDir + "/.config"

	// checking and creating a directory if it does not exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		err := os.Mkdir(configDir, os.ModePerm)
		if err != nil {
			panic("Failed to create directory: " + err.Error())
		}
	}

	// serializing data in MsgPack format
	outData, err := msgpack.Marshal(configs)
	if err != nil {
		panic("Failed to marshal updated configuration data: " + err.Error())
	}

	// writing data to a file
	err = os.WriteFile(configFile, outData, 0644)
	if err != nil {
		panic("Failed to write to configuration file: " + err.Error())
	}
}

func (c *Configuration) EditReceivedData(arguments []string) {
	// parsing arguments
	idStr, field, newValue := arguments[0], arguments[1], arguments[2]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Printf("invalid id format: %v", err)
		return
	}

	var data Configuration
	var dataBin = data.GetFileConfiguration()
	configFile := "sites/.config"

	// check for the presence of a configuration with a given id
	if id < 0 || id > len(dataBin) {
		fmt.Printf("configuration with id %d not found", id)
		return
	}

	// we get a link to the required configuration
	config := &dataBin[id-1]

	// check and change the value of the specified field
	switch field {
	case "name":
		config.Name = newValue
	case "protocol":
		config.Protocol = newValue
	case "port":
		config.Port = newValue
	case "host":
		config.Host = newValue
	case "login":
		config.Login = newValue
	case "password":
		config.Password = newValue
	case "dblogin":
		config.DBlogin = newValue
	case "dbpassword":
		config.DBpassword = newValue
	case "dbtype":
		config.DBType = newValue
	case "dbhost":
		config.DBHost = newValue
	case "dbport":
		config.DBPort = newValue
	case "dbdatabase":
		config.DBDatabase = newValue
	case "rootdirectory":
		config.RootDirectory = newValue
	case "savedirectory":
		config.SaveDirectory = newValue
	case "maxthreads":
		config.MaxThreads = newValue
	default:
		fmt.Printf("invalid field name: %s", field)
		return
	}

	// serializing the modified configuration array and writing to a file
	updatedData, err := msgpack.Marshal(dataBin)
	if err != nil {
		fmt.Printf("failed to marshal updated configuration data: %v", err)
		return
	}

	err = os.WriteFile(configFile, updatedData, 0644)
	if err != nil {
		fmt.Printf("failed to write updated configuration data to file: %v", err)
		return
	}

	fmt.Println("Configuration updated successfully.")
}

func (c *Configuration) GetFileConfiguration() (m []Configuration) {
	const configFile = "sites/.config"
	var dataBin []Configuration

	// Checking if a file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("Configuration file not found.")
		return
	}

	// Reading data from a file
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("Failed to read configuration file:", err)
		return
	}

	// Checking if a file is empty
	if len(data) == 0 {
		fmt.Println("Configuration file is empty.")
		return
	}

	// Decoding MsgPack data
	err = msgpack.Unmarshal(data, &dataBin)
	if err != nil {
		fmt.Println("Failed to unmarshal configuration data:", err)
		return
	}

	// Check that the data was decoded successfully and is not empty
	if len(dataBin) == 0 {
		fmt.Println("No configuration data found in the file.")
		return
	}

	return dataBin
}
