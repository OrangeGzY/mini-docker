package cgroups

import (
	"./subsystems"
	"github.com/sirupsen/logrus"
)

type CgroupManager struct {
	// cgroup 在hierarchy 的绝对路径
	Path string
	// 资源配置
	Resource *subsystems.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{Path: path}
}

// traverse每一个资源限制处理链，将进程的pid加到每个cgroup中
func (c *CgroupManager) Apply(pid int) error {
	for _, subSysIns := range subsystems.SubsystemIns {
		subSysIns.Apply(c.Path, pid)
	}
	return nil
}

// 遍历每一个资源限制链，都调用Set设置资源限制
func (c *CgroupManager) Set(res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemIns {
		subSysIns.Set(c.Path, res)
	}
	return nil
}

//释放cgroup
func (c *CgroupManager) Destroy() error {
	for _, subSysIns := range subsystems.SubsystemIns {
		//logrus.Infof("[Destroy] c.Path %s", c.Path)
		if err := subSysIns.Remove(c.Path); err != nil {
			logrus.Warnf("remove cgroup fail %v", err)
		}
	}
	return nil
}
