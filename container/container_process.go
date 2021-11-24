package container

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	EXIT                string = "exited"
	DefaultInfoLocation string = "/var/run/mydocker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        // container's init process's PID in host
	Id          string `json:"id"`         // container's ID
	Name        string `json:"name"`       // container's Name
	Command     string `json:"command"`    // container's init Command
	CreatedTime string `json:"createTime"` // container's Created Time
	Status      string `json:"status"`     // container's Status
	Volume      string `json:"volume"`     // container's Volume
}

var (
	RootUrl       string = "/root/go/mydocker/mydocker/"
	MntUrl        string = "/root/go/mydocker/mydocker/mnt/%s"
	WriteLayerURL string = "/root/go/mydocker/mydocker/writeLayer/%s"
)

func NewParentProcess(tty bool, volume string, containerName string, imageName string, envSlice []string) (*exec.Cmd, *os.File) {

	readPipe, writePipe, err := NewPipe()

	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	//args := []string{"init", command}

	// 构建命令
	// 相当于运行了mydocker init [option]
	// mydocker init WILL CALL THE FIRST PROCESS: RunContainerInitProcess
	cmd := exec.Command("/proc/self/exe", "init")
	//创建newspace隔离的环境，这里在文件系统上是隔离的
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// generate container.log
		dirURL := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			log.Errorf("[NewParentProcess] Mkdir %s error %v", dirURL, err)
			return nil, nil
		}
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("[NewParentProcess] Create file %s error %v", stdLogFile, err)
			return nil, nil
		}
		// redirect the file ouput stream
		cmd.Stdout = stdLogFile
	}

	//在这里传入管道读端的句柄到子进程
	cmd.ExtraFiles = []*os.File{readPipe}
	cmd.Env = append(os.Environ(), envSlice...)
	// Add Dir
	//cmd.Dir = "/root/go/mydocker/mydocker/busybox"

	// Use AUFS to boot the container
	// mntURL := "/root/go/mydocker/mydocker/mnt/"
	// rootURL := "/root/go/mydocker/mydocker/"
	/* NewWorkSpace(rootURL, mntURL) */
	/* NewWorkSpace(rootURL, mntURL, volume) */

	NewWorkSpace(volume, imageName, containerName)
	cmd.Dir = fmt.Sprintf(MntUrl, containerName)

	return cmd, writePipe

}

//准备在父子进程之间用匿名管道传递参数
func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}
