package main

import (
	"fmt"
	"os/exec"

	"./container"
	log "github.com/sirupsen/logrus"
)

func commitContainer(containerName string, imageName string) {
	mntURL := fmt.Sprintf(container.MntUrl, containerName)
	mntURL += "/"
	imageTar := container.RootUrl + "/" + imageName + ".tar"
	fmt.Printf("%s\n", imageTar)

	cmd := "tar " + "-czf " + imageTar + mntURL
	fmt.Printf("%s\n", cmd)

	if _, err := exec.Command("tar", "-czf", imageTar, mntURL).CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error: %v", mntURL, err)
	}
}
