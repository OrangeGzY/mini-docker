package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"./container"
	_ "./nsenter"
	log "github.com/sirupsen/logrus"
)

const ENV_EXEC_PID = "mydocker_pid"
const ENV_EXEC_CMD = "mydocker_cmd"

func ExecContainer(containerName string, commandArray []string) {
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		log.Errorf("[ExecContainer] Get Name: %s , error: %v", containerName, err)
		return
	}
	cmdStr := strings.Join(commandArray, " ")
	//log.Infof("Command: %s", cmdStr)
	//log.Infof("Container's pid: %s", pid)

	// Exec: docker exec
	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, cmdStr)

	// Get container's ENVs
	containerEnvs := getEnvsByPid(pid)
	// Set both host's ENVs and container's ENVs
	cmd.Env = append(os.Environ(), containerEnvs...)

	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container: %s ; error: %v", containerName, err)
	}

}

// Get container_PID through container_NAME
func getContainerPidByName(containerName string) (string, error) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFilePath := dirURL + container.ConfigName
	contentBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return "", err
	}

	// Unseriliazed to containerInfo Object
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return "", err
	}
	return containerInfo.Pid, nil
}

func getEnvsByPid(pid string) []string {
	/* /proc/<PID>/environ */
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Read environ file error : %v", err)
		return nil
	}
	// different env use \u0000 as the split byte
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs
}
