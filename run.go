package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"strconv"
	"strings"
	"time"

	"./container"
	log "github.com/sirupsen/logrus"
)

// func Run(tty bool, comArray []string, res *subsystems.ResourceConfig) {
func Run(tty bool, comArray []string, volume string, containerName string, imageName string, envSlice []string) {

	containerID := randStringBytes(10)
	if containerName == "" {
		containerName = containerID
	}

	//NewParentProcess 负责构建隔离的newspace 其中包含了docker init
	//NewParentProcess 返回构建好的命令

	/* parent, writePipe := container.NewParentProcess(tty) */
	parent, writePipe := container.NewParentProcess(tty, volume, containerName, imageName, envSlice)

	if parent == nil {
		log.Errorf("[Run] new parent process error")
		return
	}
	//运行对应的命令并等待结束
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	// Add the recordContainerInfo to recored the container information
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName, containerID, volume)
	if err != nil {
		log.Errorf("Record container info error: %v", err)
		return

	}

	/*

		//创建cgroup manager
		cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
		defer cgroupManager.Destroy()
		cgroupManager.Set(res)
		//将容器进程加入对应的各个subsystem的cgroup中
		cgroupManager.Apply(parent.Process.Pid)

	*/

	sendInitCommand(comArray, writePipe)

	if tty {
		parent.Wait()
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(volume, containerName)
	}

	// Remove it because it will delete the detach container's WorkSpace
	// mntURL := "/root/go/mydocker/mydocker/mnt/"
	// rootURL := "/root/go/mydocker/mydocker/"

	// /* container.DeleteWorkSpace(rootURL, mntURL) */
	// // Add volume
	// container.DeleteWorkSpace(rootURL, mntURL, volume)

	os.Exit(-1)
}

func deleteContainerInfo(containerId string) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerId)
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove dir %s error %v ", dirURL, err)
	}
}
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	//log.Infof("[sendInitCommand] command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

// Generate the container's ID
func randStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyz1234567890"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func recordContainerInfo(containerPID int, commandArray []string, containerName, id, volume string) (string, error) {
	// 1. Generate container's ID
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := strings.Join(commandArray, "")

	if containerName == "" {
		containerName = id
	}

	// 2. Create the containerInfo struct
	containerInfo := &container.ContainerInfo{
		Pid:         strconv.Itoa(containerPID),
		Command:     command,
		Name:        containerName,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Id:          id,
		Volume:      volume,
	}

	// 3. Json to string
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record containerInfo failed : %v", err)
		return "", err
	}
	jsonStr := string(jsonBytes)

	// 4. Format and generate the file path
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	// if the path doesn't exist, we create them.
	if err := os.MkdirAll(dirURL, 0622); err != nil {
		log.Errorf("Mkdir error: %v , dir: %s", err, dirURL)
		return "", err
	}
	fileName := dirURL + "/" + container.ConfigName
	//log.Infof("config")

	// 5. Create the final config file
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		log.Errorf("Create file: %s ; error: %v", fileName, err)
		return "", err
	}
	log.Infof("Configure File: %s", fileName)

	// 6. Write the information to Configure file
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File Write string Error: %v", err)
		return "", err
	}
	return containerName, nil
}
