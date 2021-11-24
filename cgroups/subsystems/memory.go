package subsystems

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/sirupsen/logrus"
)

type MemorySubSystem struct {
}

func (s *MemorySubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	//获取当前subsystem在虚拟文件系统中的路径
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err == nil {
		//logrus.Infof("[Memory Set Cgroup] %s", subsysCgroupPath)
		// 如果设置了对应的内存限制
		// 那么写入cgroup对应目录的memory.limit_in_bytes文件中
		if res.MemoryLimit != "" {
			if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"), []byte(res.MemoryLimit), 0644); err != nil {
				return fmt.Errorf("set cgroup memory failed %v", err)
			}
		}
		return nil
	} else {
		return err
	}
}

func (s *MemorySubSystem) Name() string {
	return "memory"
}

func (s *MemorySubSystem) Remove(cgroupPath string) error {
	//logrus.Infof("[Memory::Remove() Cgroup PATH] %s", path.Join(s.Name(), cgroupPath))

	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		//logrus.Infof("[Memory Remove Cgroup SUCCESS] %s", subsysCgroupPath)
		//删除cgroupPath 对应的目录
		return os.Remove(subsysCgroupPath)
	} else {
		//subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false)
		logrus.Infof("[Memory Remove Cgroup FAILED] %s", subsysCgroupPath)
		return err
	}
}

func (s *MemorySubSystem) Apply(cgroupPath string, pid int) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err == nil {
		//writePath := path.Join(subsysCgroupPath, "tasks")
		//logrus.Infof("[Memroy Apply Cgroup] %s", writePath)
		if err := ioutil.WriteFile(
			path.Join(subsysCgroupPath, "tasks"),
			[]byte(strconv.Itoa(pid)),
			0644); err != nil {
			return fmt.Errorf("set cgroup failed %v", err)
		}
		return nil

	} else {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
}
