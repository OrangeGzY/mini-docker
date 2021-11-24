package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

func NewWorkSpace(volume string, imageName string, containerName string) {
	/*
		mntURL := "/root/mnt/"
		rootURL := "/root/go/mydocker/mydocker/"
	*/
	CreateReadOnlyLayer(imageName)
	CreateWriteLayer(containerName)
	CreateMountPoint(containerName, imageName)

	// Check if user wanna mount the volume
	if volume != "" && volume != "false" {
		//Source -> Target
		//volumeURLs := volumeUrlExtract(volume)
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(volumeURLs, containerName)
			log.Infof("volumeURLs: %q", volumeURLs)
		} else {
			log.Errorf("Volume param input error!\n volume:%s\n", volume)
		}
	}

}

//parse the volume string
func volumeUrlExtract(volume string) []string {
	var volumeURLs []string
	volumeURLs = strings.Split(volume, ":")
	return volumeURLs
}

func MountVolume(volumeURLs []string, containerName string) error {
	// 1. Create  dir in host
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Errorf("mkdir %s failed, error: %v", parentUrl, err)
	}

	// 2. Create mount point in container filesystem
	containerURL := volumeURLs[1]
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerVolumeURL := mntURL + "/" + containerURL
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Errorf("mkdir %s failed, error: %v", containerVolumeURL, err)
	}

	// 3. Mount host's DIR to container's mount point
	dirs := "dirs=" + parentUrl
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL).CombinedOutput()
	if err != nil {
		log.Errorf("Mount volume Failed: %v", err)
		return err
	}
	return nil
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("Mount volume failed: %v", err)
	// }

}

func DeleteMountPointWithVolume(volumeURLs []string, containerName string) error {
	// 1. Unload the volume's mount point inside of the container
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerUrl := mntURL + "/" + volumeURLs[1]
	if _, err := exec.Command("umount", containerUrl).CombinedOutput(); err != nil {
		log.Errorf("umount Volume : %s failed , error: %v", containerUrl, err)
		return err
	}
	// cmd := exec.Command("umount", containerUrl)
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stdout
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("umount Volume : %s failed , error: %v", containerUrl, err)
	// }

	// 2. Unload the whole fs 's mount point
	if _, err := exec.Command("umount", mntURL).CombinedOutput(); err != nil {
		log.Errorf("umount mount point : %s failed , error: %v", mntURL, err)
		return err
	}
	// cmd = exec.Command("umount", mntURL)
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stdout
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("umount mount point : %s failed , error: %v", containerUrl, err)
	// }

	// 3. Delete the whole fs 's mount point
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove mountpint dir %s error: %v", mntURL, err)
	}
	return nil

}

// undecompress the busybox.tar to busybox/
// as the container's Read-Only Layer
func CreateReadOnlyLayer(imageName string) error {
	// busyboxURL := rootURL + "busybox/"
	// busyboxTarURL := rootURL + "busybox.tar"

	unTarFolderUrl := RootUrl + "/" + imageName + "/"
	imageUrl := RootUrl + "/" + imageName + ".tar"
	exist, err := PathExists(unTarFolderUrl)

	if err != nil {
		log.Errorf("[CreateReadOnlyLayer] Failed to judge the %s , error: %v", unTarFolderUrl, err)
		return err
	}

	// If the directory doesn't exist, now we create the dir and undecompress the .tar to it.
	if exist == false {
		if err := os.Mkdir(unTarFolderUrl, 0777); err != nil {
			log.Errorf("[CreateReadOnlyLayer] Failed to created busyboxURL: %s , error: %v", unTarFolderUrl, err)
			return err
		}

		log.Infof("Create %s ", unTarFolderUrl)

		// tar -xvf busybox.tar -C busybox/
		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", unTarFolderUrl).CombinedOutput(); err != nil {
			log.Errorf("[CreateReadOnlyLayer] Untar dir: %s , error: %v", unTarFolderUrl, err)
			return err
		}

		log.Infof("Untar %s to %s ", imageUrl, unTarFolderUrl)
	}
	return nil
}

// Create/Mkdir writeLayer as the container's Only-Write Layer
func CreateWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerURL, containerName)
	if err := os.MkdirAll(writeURL, 0777); err != nil {
		log.Errorf("[CreateWriteLayer] mkdir: %s ,error: %v", writeURL, err)
	}
	log.Infof("CreateWriteLayer: %s", writeURL)
}

func CreateMountPoint(containerName string, imageName string) error {
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	// mkdir mnt/ as the mount point
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Errorf("Mkdir %s , error: %v", mntUrl, err)
	}

	tmpWriteLayer := fmt.Sprintf(WriteLayerURL, containerName)
	tmpImageLocation := RootUrl + "/" + imageName
	mntURL := fmt.Sprintf(MntUrl, containerName)
	// Try to mount writeLayer/ and busybox/ to mnt/
	dirs := "dirs=" + tmpWriteLayer + ":" + tmpImageLocation
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL).CombinedOutput()

	if err != nil {
		log.Errorf("Create mount point failed : %v", err)
		return err
	}
	return nil
	// log.Infof("Exec: %s %s %s %s %s %s %s", "mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("%v", err)
	// }
}

// Check if the file's path exists
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// Delect AUFS when container exit
/*

	First, umount mnt dir
	Then,  delete mnt
	Finally, delete writeLayer dir.

	After these steps, any changes we done to the FS has been removed !

*/
func DeleteWorkSpace(volume string, containerName string) {
	if volume != "" {
		//volumeURLs := volumeUrlExtract(volume)
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(volumeURLs, containerName)
		} else {
			DeleteMountPoint(containerName)
		}
	} else {
		DeleteMountPoint(containerName)
	}
	DeleteWriteLayer(containerName)
}

func DeleteMountPoint(containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	_, err := exec.Command("umount", mntURL).CombinedOutput()
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	if err != nil {
		log.Errorf("[DeleteMountPoint] %v", err)
		return err
	}

	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove dir %s error %v", mntURL, err)
		return err
	}
	return nil
}

func DeleteWriteLayer(containerName string) {
	//writeURL := rootURL + "writeLayer/"
	writeURL := fmt.Sprintf(WriteLayerURL, containerName)
	if err := os.RemoveAll(writeURL); err != nil {
		log.Errorf("[DeleteWriteLayer] Remove dir %s error %v", writeURL, err)
	}
}
