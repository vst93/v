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

// Set writes a value to the ini config and persists it to disk.
// section is optional; defaults to the empty (default) section.
func Set(key, val, section string) error {
	err := ini.Set(key, val, section)
	if err != nil {
		return err
	}
	return SaveSetting()
}
