package setting

import (
	"os"

	"github.com/gookit/ini/v2"
)

var userHomePath string

func InitSetting() {
	var err error
	userHomePath, err = os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(userHomePath + "/.v_tools"); os.IsNotExist(err) {
		os.Mkdir(userHomePath+"/.v_tools", 0755)
	}
	if _, err := os.Stat(userHomePath + "/.v_tools/settings.ini"); os.IsNotExist(err) {
		f, err := os.Create(userHomePath + "/.v_tools/settings.ini")
		if err != nil {
			panic(err)
		}
		f.Close()
	}
	// loading ini info
	err = ini.LoadExists(userHomePath + "/.v_tools/settings.ini")
	if err != nil {
		panic(err)
	}
}

func SaveSetting() error {
	if userHomePath == "" {
		return nil
	}
	_, err := ini.Default().WriteToFile(userHomePath + "/.v_tools/settings.ini")
	if err != nil {
		return err
	}
	return nil
}

func Set(section, key, value string) error {
	err := ini.Set(section, key, value)
	if err != nil {
		return err
	}
	return SaveSetting()
}
