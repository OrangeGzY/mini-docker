package subsystems

type ResourceConfig struct {
	MemoryLimit string
	CpuShare    string
	CpuSet      string
}

//subsystem的接口（可实现）
type Subsystem interface {
	Name() string                               // 获取subsystem的名字
	Set(path string, res *ResourceConfig) error //设置cgroup在这个subsystem中的资源限制
	Apply(path string, pid int) error           //将进程添加到某个cgroup
	Remove(path string) error                   //移除某个cgroup
}

// 资源限制处理链
var (
	SubsystemIns = []Subsystem{
		&CpusetSubSystem{},
		&MemorySubSystem{},
		&CpuSubSystem{},
	}
)
