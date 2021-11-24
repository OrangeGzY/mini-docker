package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"./container"
	log "github.com/sirupsen/logrus"
)

func logContianer(containerName string) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	logFileLocation := dirURL + container.ContainerLogFile

	// Open log files
	file, err := os.Open(logFileLocation)
	defer file.Close()
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFileLocation, err)
		return
	}
	// Read the file's Content
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFileLocation, err)
		return
	}
	fmt.Fprint(os.Stdout, string(content))
}
