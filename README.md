[TOC]

# 实现基本的容器隔离（Pipe + Namespace + Cgroup）

```sh
├── cgroups
│   ├── cgroup_manager.go
│   └── subsystems
│       ├── cpu.go
│       ├── cpuset.go
│       ├── memory.go
│       ├── subsystem.go
│       └── utils.go
├── container
│   ├── container_process.go
│   └── init.go
├── main_command.go
├── main.go
├── mydocker
├── network
├── restore.sh
└── run.go
```

首先，main.go 中创建了基于cli的基本的App()

main_command.go 中是定义了两个cli下容器运行时的命令：```init``` 、```run``` 。其中init只允许从内部调用。而run可以实现 ```./mydocker run [Flag] [Args]``` 。

再runCommand中我们实现了几个最基本的Flag：

```go
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		&cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		&cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		&cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
```

以及一个Action操作对应的func，当我们运行 ```mydocker run``` 时它会被调用：

```go
	Action: func(context *cli.Context) error {
		// 检查run时的参数个数
		if context.NArg() < 1 {
			return fmt.Errorf("[runCommand] Missing container command ??????")
		}

		args := context.Args()
		cmdArray := make([]string, args.Len()) // command
		for index, cmd := range args.Slice() {
			cmdArray[index] = cmd
		}

		// 检验是否使用tty交互模式
		tty := context.Bool("ti")
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuSet:      context.String("cpuset"),
			CpuShare:    context.String("cpushare"),
		}

		Run(tty, cmdArray, resConf)
		return nil
	},
```

- 首先它完成参数检查。
- 然后将我们的参数转移到参数数组 cmdArray[]中，并check是否需要tty交互模式。
- 接下来通过 ```ResourceConfig``` 构建对资源的限制 。
- 最后调用 run.go 里面的 ```func Run(tty bool, comArray []string, res *subsystems.ResourceConfig)``` 
- 控制流转移到run.go

在 run.go 中，首先调用container/container_process.go 中的 ```NewParentProcess``` 进行命令构建。

## 匿名管道与命令传递

在 ```NewParentProcess``` 中，我们首先通过 ```os.Pipe()``` 拿到一个匿名管道，这里比较关键的是返回了读写的句柄（文件？）

当我们拿到 ```readPipe``` 和 ```writePipe``` 之后，我们正常的构建 ```cmd := exec.Command("/proc/self/exe", "init")``` ，其实就是构建命令（此时还没执行） ```./mydocker init``` 

这里还有一个地方，我们实际是通过：

```go
	//在这里传入管道读端的句柄到子进程
	cmd.ExtraFiles = []*os.File{readPipe}
```

将fd传给子进程的。

我们将构建好的cmd和writePipe返回。

```go
parent, writePipe := container.NewParentProcess(tty)
```

然后接下来我们运行构建好的命令：

```go
if err := parent.Start(); err != nil {
		log.Error(err)
	}
```

在运行了init之后，会调用 container/init.go 中的 ```RunContainerInitProcess``` 作为容器启动的第一个进程，此时他会阻塞在 ```cmdArray := readUserCommand()``` 。

因为 ```readUserCommand``` 被我们设计为从新的pipe中进行读取，但此时pipe是空的。所以容器的init进程卡在这里。（注意，此时我们在外部获取的管道的读侧已经通过上文所述方法传递给子进程了）

然后我们加入对应的cgroup限制：

```go
	//创建cgroup manager
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	cgroupManager.Set(res)
	//将容器进程加入对应的各个subsystem的cgroup中
	cgroupManager.Apply(parent.Process.Pid)
```

此时对应的cgroup限制就创建完毕了。

然后由我们的父进程通过匿名管道向容器（子进程）发送命令：

```go
sendInitCommand(comArray, writePipe)
```

```go
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("[sendInitCommand] command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}
```

我们再将视线转移到刚刚被阻塞的子进程中：

```go
cmdArray := readUserCommand()
```

当他收到父进程传递过来的命令之后，初始化了对应的cmdArray

然后完成了mount操作：

```go
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// mount proc
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
```

并且通过：```exec.LookPath(cmdArray[0])``` 找到了对应命令的绝对路径，比如 ls 就是 /bin/ls。

接下来，在容器的子进程中直接通过execve执行命令，替换当前上下文：

```go
	// 利用exec系统调用执行command，直接替换当前进程上下文
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		logrus.Errorf(err.Error())
	}
```

至此，子进程即容器中一条命令的执行就完成了。等价于 ```docker run```

## 资源限制

我们重新看一下cgroup在最顶层的这一部分：

```go
	//创建cgroup manager
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	cgroupManager.Set(res)
	//将容器进程加入对应的各个subsystem的cgroup中
	cgroupManager.Apply(parent.Process.Pid)
```

首先我们根据名字创建一个新的cgroupmanager

defer延时保证了最终退出时会被清理。

在调用Run时，对应的res为：

```go
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuSet:      context.String("cpuset"),
			CpuShare:    context.String("cpushare"),
		}

		Run(tty, cmdArray, resConf)
```

也就是这里有三个limit。

我们以memory为例子。

首先调用的Set就是先获取了当前Subsysmtem在系统中的绝对路径：```subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true);``` 

```sh
{"level":"info","msg":"[Memory Set Cgroup] /sys/fs/cgroup/memory/mydocker-cgroup","time":"2021-11-17T17:40:33+08:00"}
```

然后直接对其进行写入：

```go
if res.MemoryLimit != "" {
			if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"), []byte(res.MemoryLimit), 0644); err != nil {
				return fmt.Errorf("set cgroup memory failed %v", err)
			}
		}
```

这之后，通过Apply()函数将容器进程的pid加入对应的文件中：

```go
if err := ioutil.WriteFile(
			path.Join(subsysCgroupPath, "tasks"),
			[]byte(strconv.Itoa(pid)),
			0644); err != nil {
			return fmt.Errorf("set cgroup failed %v", err)
		}
```

```go
{"level":"info","msg":"[Memroy Apply Cgroup] /sys/fs/cgroup/memory/mydocker-cgroup/tasks","time":"2021-11-17T17:46:41+08:00"}
```

最后在Destory中会remove掉对应的path：

```go
//释放cgroup
func (c *CgroupManager) Destroy() error {
	for _, subSysIns := range subsystems.SubsystemIns {
		if err := subSysIns.Remove(c.Path); err != nil {
			logrus.Warnf("remove cgroup fail %v", err)
		}
	}
	return nil
}
```

不过有个问题是每次exit之后要重新mount一下 /proc 

## 关于mount

在运行程序的时候发现一个很有趣的点：

如果我们先运行：

```sh
./mydocker run -ti ls 
```

然后你就会发现，在 ```/proc``` 目录下的文件发生了变化。

```sh
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ls /proc/
1        1199     1423571  24   612  81   96           fb           meminfo       sysrq-trigger
10       12       1423786  25   613  815  967          filesystems  misc          sysvipc
1086     125      1423842  26   614  817  977          fs           modules       thread-self
109      1258217  1423898  27   637  818  98           interrupts   mounts        timer_list
11       13       1433     273  718  82   99           iomem        mtrr          tty
1100     14       15       274  720  825  acpi         ioports      net           uptime
112      1405835  16       28   735  83   buddyinfo    irq          pagetypeinfo  version
1128687  1406045  169      29   738  847  bus          kallsyms     partitions    version_signature
1173     1406752  17       3    745  85   cgroups      kcore        pressure      vmallocinfo
1176     1406905  172      30   746  852  cmdline      keys         sched_debug   vmstat
1183     1418531  174      344  750  88   consoles     key-users    schedstat     zoneinfo
1195662  1419     18       365  752  89   cpuinfo      kmsg         scsi
1196339  1420179  2        366  76   9    crypto       kpagecgroup  self
1196438  1420180  20       377  77   91   devices      kpagecount   slabinfo
1196522  1423206  21       4    78   92   diskstats    kpageflags   softirqs
1196542  1423207  22       6    79   93   dma          loadavg      stat
1196933  1423341  228      610  792  94   driver       locks        swaps
1197238  1423342  23       611  80   95   execdomains  mdstat       sys
```

运行之后：

```sh
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ls /proc/
ls: cannot read symbolic link '/proc/self': No such file or directory
ls: cannot read symbolic link '/proc/thread-self': No such file or directory
acpi       diskstats    ioports      kpageflags  net           softirqs       uptime
buddyinfo  dma          irq          loadavg     pagetypeinfo  stat           version
bus        driver       kallsyms     locks       partitions    swaps          version_signature
cgroups    execdomains  kcore        mdstat      pressure      sys            vmallocinfo
cmdline    fb           keys         meminfo     sched_debug   sysrq-trigger  vmstat
consoles   filesystems  key-users    misc        schedstat     sysvipc        zoneinfo
cpuinfo    fs           kmsg         modules     scsi          thread-self
crypto     interrupts   kpagecgroup  mounts      self          timer_list
devices    iomem        kpagecount   mtrr        slabinfo      tty
```

注意一下，很特别的，此时在 ```/proc``` 下， 的self变成了无法读取的软链接，直接坏掉了。并且还少了很多其他文件 。

我个人认为，其原因在于：

- 在我们启动容器子进程是，首先指定了：

  ```go
  cmd.SysProcAttr = &syscall.SysProcAttr{
  		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
  	}
  ```

  可以看到 ```CLONE_NEWNS``` 代表其在mount namespace上是隔离的。

- 接下来在容器子进程中做了一个mount的操作：

  ```go
  	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
  	// mount proc
  	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
  ```

  注意，这里将proc mount到了 /proc 下。而这里的 /proc 表征的是容器内的情况。

- 由于 “同一个设备可以有多个挂载点，同一个挂载点同时只能加载一个设备。” 的缘故。当我们exit出容器后，此时的 /proc 下仍然是容器内部隔离的proc。但是此时容器已退出，那么proc中很多表征容器内运行时环境的文件自然也就无了，或者broken/unreachable了.

- 此时我们只需要再进行同样的 mount proc一次即可，对/proc 进行覆盖：

  ```sh
  mount -t proc proc /proc
  ```

# 实现镜像与容器分离

这一部分我们主要是解决前面的一个问题，可以看到在前面我们容器中的目录仍然是继承容器外host的目录，那么这一步我们要完成目录的分离。

其实就是实现如下几步：

首先构建一个busybox文件系统

```sh
docker pull busybox
docker run -d busybox top -b
docker export -o busybox.tar {ID}
tar -xvf busybox.tar -C busybox/
```

这样我们就得到了一个busybox文件夹。

接下来我们通过 ```pivot_root``` 系统调用，改变当前的root文件系统，切换整个系统到一个新的目录。切换完之后我们再移除旧的目录。

这里在pivot之前我们要remount root一次，目的是让root成为mount point：

```go
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v ", err)
	}
```

而pivotRoot由 ```setUpMount()```  调用。

在setUpMount()中，首先做了一个：

```GO
	/*

		After systemd added into Linux,
		mount namespace becomes shared by default
		So we have to declare the new namespace is quite independent and isolated

	*/
	// ensure that container mount and parent mount has no shared propagation
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		logrus.Errorf("mount / fails: %v", err)
		return err
	}
```

保证mount ns不是默认的shared by default。

接下里才是获取当前目录然后pivotRoot，结束之后 / 已经切换。

我们接下来挂上proc和tmpfs：

```go
	// mount proc
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		log.Errorf("[setUpMount] Failed to mount proc: %v", err)
		return err
	}
	// mount tmpfs
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		log.Errorf("[setUpMount] Failed to mount tmpfs: %v", err)
		return err
	}
```

接下来是实现容器内的不同层次，主要是依赖AUFS来做的。

```go
func NewWorkSpace(rootURL string, mntURL string) {
	/*
		mntURL := "/root/mnt/"
		rootURL := "/root/go/mydocker/mydocker/"
	*/
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)
}
```

- CreateReadOnlyLayer(rootURL) 用来创建容器内的只读层，我们将busybox.tar 解压到busybox/ 

- CreateWriteLayer(rootURL) 用来创建容器内的可写层，writeLayer/

- 在这两层的目录创建完毕后，我们用mount配合AUFS的CoW来实现image layer共享，文件管理。

  ```sh
  mount -t aufs -o dirs=/root/go/mydocker/mydocker/writeLayer:/root/go/mydocker/mydocker/busybox none /root/go/mydocker/mydocker/mnt/
  ```

  在mount aufs时，默认的是dirs左边第一个目录是rw权限；后面都是ro权限。

  也就是此时/root/go/mydocker/mydocker/writeLayer是可写层，/root/go/mydocker/mydocker/busybox是只读层，挂载点是 /root/go/mydocker/mydocker/mnt/ 。

- 我们运行启动看一下：

  如果我们在启动后的root下新建一个a

  ```sh
  / # pwd
  /
  / # mount
  none on / type aufs (rw,relatime,si=815a243759ee58bb)
  proc on /proc type proc (rw,nosuid,nodev,noexec,relatime)
  tmpfs on /dev type tmpfs (rw,nosuid,mode=755)
  / # ls
  bin   dev   etc   home  proc  root  sys   tmp   usr   var
  / # cd root/
  ~ # ls
  ~ # touch a
  ```

  那么我们发现在host的writeLayer中多了一个root/a

  而 /busybox/root 下没有任何文件夹。



再总结一下整体的实现过程：

启动容器时：

1. 创建只读层（busybox）
2. 创建容器读写层，用于CoW（writeLayer）
3. 创建挂载点（mnt），并把只读层和只写层挂载到挂载点
4. 将挂载点设置为容器根目录

容器退出时：

1. 卸载挂载点（mnt）的文件系统
2. 删除挂载点
3. 删除用于CoW的读写层（writeLayer）

# 实现镜像

## volume与数据可持久化

在上一部分的实现中实际上还有一些问题，就是我们针对容器所有的修改实际上是 “无状态” 的——当我们退出容器之后所有的修改就没有了。

所以我们要用volume来实现针对容器内数据的可持久化。

实现类似于：

```sh
./mydocker run -ti -v /root/go/mydocker/mydocker/volume:/containerVolume sh
```

其中 ```/root/go/mydocker/mydocker/volume``` 是host目录，```/containerVolume``` 是容器启动起来后容器进程内的目录。

我认为这里本质上就是一个分层挂载的过程。

首先把容器内基本的那些挂出来。然后：

```go
func MountVolume(rootURL string, mntURL string, volumeURLs []string) {
	// 1. Create  dir in host
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Errorf("mkdir %s failed, error: %v", parentUrl, err)
	}

	// 2. Create mount point in container filesystem
	containerURL := volumeURLs[1]
	containerVolumeURL := mntURL + containerURL
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Errorf("mkdir %s failed, error: %v", containerVolumeURL, err)
	}

	// 3. Mount host's DIR to container's mount point
	dirs := "dirs=" + parentUrl
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Mount volume failed: %v", err)
	}

}
```

可以看到，首先创建了host上的目录dirs。

然后创建了容器内部的挂咋点（目录）。

最后调用mount将 dirs 挂到容器内部 的containerVolumeURL。即可

有点像一个二层的挂载。

而对应目录的内容在容器退出后会被保留下来。

## 实现基本的容器打包

我们新添加一个命令 ```commit```

```go
// mydocker commit
var commitCommand = &cli.Command{
	Name: "commit",

	Usage: "Commit a contaienr to image",

	//获取command 容器初始化
	Action: func(context *cli.Context) error {
		if context.NArg() < 1 {
			return fmt.Errorf("Missing container name ")
		}

		imageName := context.Args().Get(0)

		commitContainer(imageName)
		return nil
	},
}

```

然后在新的 commit.go 中进行一个很简单的tar压缩操作，作为打包。

```go
func commitContainer(imageName string) {
	mntURL := "./mnt/"
	imageTar := imageName + ".tar"
	fmt.Printf("%s\n", imageTar)

	cmd := "tar " + "-czf " + imageTar + mntURL
	fmt.Printf("%s\n", cmd)

	if _, err := exec.Command("tar", "-czf", imageTar, mntURL).CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error: %v", mntURL, err)
	}
}
```

我们可以先启动起来一个 sh

```sh
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ./mydocker run -ti sh
{"level":"info","msg":"CreateWriteLayer: /root/go/mydocker/mydocker/writeLayer/","time":"2021-11-19T17:43:55+08:00"}
{"level":"info","msg":"Exec: mount -t aufs -o dirs=/root/go/mydocker/mydocker/writeLayer:/root/go/mydocker/mydocker/busybox none /root/go/mydocker/mydocker/mnt/","time":"2021-11-19T17:43:55+08:00"}
{"level":"info","msg":"******* Container Initializing *******","time":"2021-11-19T17:43:55+08:00"}
/ # 
```

然后在新的tarminal中：

```sh
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ./mydocker commit image
image.tar
tar -czf image.tar./mnt/
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ls -lah ./image.tar 
-rw-r--r-- 1 root root 733K Nov 19 17:44 ./image.tar
```

可以看到生成了新的 image.tar ，就完成了一个简单的镜像打包的功能。

# 实现容器的后台运行

在我们之前实现的部分，我们扔有一些限制，比如说，我们无法通过 ```docker exec``` 进入一个已经启动后的容器。也没法通过 ```docker ps ``` 查看正在运行的容器。

也没法通过 ```docker logs``` 查看容器输出。我们这部分就来补全这些。

摘抄写得很好的一句话：

>容器，在操作系统看来，其实就是一个进程。当前运行命令的 mydocker 是主进程，容器 是被当前 mydocker 进程 fork 出来的子进程。子进程的结束和父进程的运行是一个异步的过程， 即父进程永远不知道子进程到底什么时候结束。如果创建子进程的父进程退出，那么这个子进 程就成了没人管的孩子，俗称孤儿进程。为了避免孤儿进程退出时无法释放所占用的资源而僵 死，进程号为 的进程 init 就会接受这些孤儿进程。

## 添加后台模式

我们添加一个 -d 子模式。

在启用 -d 时，容器进程后台运行（注意，-d 和 -ti 是冲突的），此时父进程不等待容器进程结束而直接退出。而容器进程就会被 1 号进程（init）接管。

此时容器内top为容器内的前台进程：

```sh
./mydocker run -d top
```

我们在容器外的host上看一下：

```sh
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ps -ef | grep top
root      737224       1  0 13:27 pts/1    00:00:00 top
```

可以看到有一个父进程是 1 号进程的 737224 进程，这就是我们的容器进程。此时他被1号进程接管了。这就是一个简单的后台运行。

 ## 查询正在运行的容器

实际上就是实现 ```docker ps``` 命令。

大体流程就是。我们在创建container的时候用以下的结构：

```go
var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	EXIT                string = "exited"
	DefaultInfoLocation string = "/var/run/mydocker/%s/"
	ConfigName          string = "config.json"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        // container's init process's PID in host
	Id          string `json:"id"`         // container's ID
	Name        string `json:"name"`       // container's Name
	Command     string `json:"command"`    // container's init Command
	CreatedTime string `json:"createTime"` // container's Created Time
	Status      string `json:"status"`     //	container's Status
}

```

```go
	// Add the recordContainerInfo to recored the container information
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName)
```

记录container的对应信息，然后json化以后写入对应的Configure 文件。容器的id我们用0~9，随机数表示

然后ps命令实际就是读对应的json文件然后格式化显示出来。

```go
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ./mydocker run -d --name fuck top
```

效果：

```go
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ./mydocker ps
ID          NAME         PID         STATUS      COMMAND     CREATED
            1559960968   816523      running     top         2021-11-20 17:51:03
            2815403318   816240      running     top         2021-11-20 17:49:51
            fuck         817366      running     top         2021-11-20 17:55:03
```

## 实现容器日志

这一步其实很简单，我们新加一个log命令，然后创建对应的文件夹，里面写入container.json文件。

如果是detach运行的：

```go
else {
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
```

那么我们这里将容器进程内的 ```cmd.Stdout``` 定向到 ```stdLogFile``` 中，而这个对应的 ```stdLogFile``` 中是我们创建的log文件。

当我们需要读log的时候：

```go
logContianer(containerName)
> 

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
```

打开对应的文件，然后 ReadAll，然后通过 ```fmt.Fprint(os.Stdout, string(content))``` 重定向到标准输出后打印出来。

## * 实现进入容器namespace（docker exec）*

这一部分很重要，我们想实现 ```exec``` 命令，实质上就是对于一个detach创建的容器，我们想要在容器外重新切入他的namespace，就实现了一个进入容器的功能。

我们这一步要用 ```setns``` 配合 Cgo 来完成。

setns是一个系统调用，它可以根据提供的pid进入指定的namespace中。

首先我们在这里新开一个exec命令：

```go
// mydocker exec
var execCommand = &cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(context *cli.Context) error {
		// For callback
		// The second time we will enter the if branch which means the env has been set and the Cgo code has been executed
		if os.Getenv(ENV_EXEC_PID) != "" {
			log.Infof("pid callback pid %d", os.Getgid())
			return nil
		}

		// The first time we will pass the if, and try to exec the Cgo code
		if context.NArg() < 2 {
			return fmt.Errorf("Misssing container name or command")
		}
		containerName := context.Args().Get(0)
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		ExecContainer(containerName, commandArray)
		return nil
	},
}
```

注意，在这里实际上 ```docker exec``` 被调用了两遍。

当我们首先创建一个 detach 的镜像：

```sh
./mydocker run --name nobody -d top

./mydocker ps
ID           NAME        PID         STATUS      COMMAND     CREATED
0867279723   nobody      693650      running     top         2021-11-22 16:49:43
```

可以看一下目前的状态：

```sh
root      693650       1  0 16:49 pts/0    00:00:00 top
```

接下来我们执行 ```docker exec nobody sh```

1. 当我们手动执行exec时，func中的 ENV_EXEC_PID 是没有被设置的，此时func检测到是第一次调用exec，会pass的if分支，向下运行。

2. 最终执行了：```ExecContainer(containerName, commandArray)```  。

3. 在 ```ExecContainer``` 中首先我们获得对应container的进程pid

4. 然后先设置cmd，设置环境变量：```ENV_EXEC_PID```  , ```ENV_EXEC_CMD``` 。这两个环境变量像开关一样，这里我们打开了这两个开关。最后再执行一次 ```docker exec``` 

   ```go
   func ExecContainer(containerName string, commandArray []string) {
   	pid, err := getContainerPidByName(containerName)
   	if err != nil {
   		log.Errorf("[ExecContainer] Get Name: %s , error: %v", containerName, err)
   		return
   	}
   	cmdStr := strings.Join(commandArray, " ")
   	log.Infof("Command: %s", cmdStr)
   	log.Infof("Container's pid: %s", pid)
   
   	// The second Exec: docker exec 
   	cmd := exec.Command("/proc/self/exe", "exec")
   	cmd.Stdin = os.Stdin
   	cmd.Stdout = os.Stdout
   	cmd.Stderr = os.Stderr
   
   	os.Setenv(ENV_EXEC_PID, pid)
   	os.Setenv(ENV_EXEC_CMD, cmdStr)
   
   	if err := cmd.Run(); err != nil {
   		log.Errorf("Exec container: %s ; error: %v", containerName, err)
   	}
   
   }
   ```

5. 当这次exec执行之后，我们在最初的exec处理func中进入了if分支，这里直接return了。我们把视线转向nsenter.go 这是一个Cgo的程序：

   ```go
   package nsenter
   
   /*
   #define _GNU_SOURCE
   #include <errno.h>
   #include <sched.h>
   #include <stdio.h>
   #include <stdlib.h>
   #include <string.h>
   #include <fcntl.h>
   #include <unistd.h>
   
   
   __attribute__( (constructor) ) void enter_namespace(void) {
   
   
       char *mydocker_pid ;
       mydocker_pid = getenv("mydocker_pid");
       if(mydocker_pid){
   		
       }else{
   		fprintf(stdout, "missing mydocker_pid env skip nsenter");
           return;
       }
       char *mydocker_cmd;
       mydocker_cmd = getenv("mydocker_cmd");
     if (mydocker_cmd) {
   		
   	} else {
   		fprintf(stdout, "missing mydocker_cmd env skip nsenter");
   		return;
   	}
   
       int i;
       char nspath[0x1000];
       char *namespace[] = {
           "ipc",
           "uts",
           "net",
           "mnt",
           "pid",
       };
   
       for( i=0 ; i<5;i++){
   
           // Open and Setns the namespace one by one
           sprintf(nspath , "/proc/%s/ns/%s" , mydocker_pid , namespace[i]);
           int fd = open(nspath , O_RDONLY);
   
   
   		if (setns(fd, 0) == -1) {
   			fprintf(stderr, "setns on %s namespace failed: %s\n", namespace[i], strerror(errno));
   		} else {
   			fprintf(stdout, "setns on %s namespace succeeded\n", namespace[i]);
   		};
           close(fd);
       }
   
       // After we enter the namespace, we execute the command in the target namespace.
       int ret = system(mydocker_cmd);
       exit(0);
       return ;
   
   }
   */
   import "C"
   
   ```

   这个Cgo的程序，由于函数的属性是 ```__attribute__((constructor))```  所以他会在程序的一开始就调用。当我们手动执行exec时，也就是第一次exec，这个函数被第一次调用，然后他检测到那两个开关（即那两个关键的环境变量没有被set）。他会自动return，不做任何事情。

   而当我们在 ```ExecContainer``` 中打开了开关，即设置了对应的环境变量。最终又执行了一遍 ```/proc/self/exe exec``` 。此时 ```enter_namespace``` 被第二次调用。

   在第二次调用中，他检测到开关已经打开，那么接着往下运行，在for循环中，用setns完成了一个命名空间的切换，即，指定当前的进程加入container进程的namespace中，最终在container namespace中运行用户指定的命令，就完成了一个 ```docker exec``` 的操作。

6. 需要额外提的一个问题时，在这一步，我们需要暂时注释掉run.go中 Run 函数的：

   ```go
   	// mntURL := "/root/go/mydocker/mydocker/mnt/"
   	// rootURL := "/root/go/mydocker/mydocker/"
   
   	// /* container.DeleteWorkSpace(rootURL, mntURL) */
   	// // Add volume
   	// container.DeleteWorkSpace(rootURL, mntURL, volume)
   ```

   原因在于，如果不注释掉这一部分，当程序检测到，我们不是以-ti启动镜像的，那么会删掉对应容器的WorkSpace。导致最后namespace的切换出现一些问题。

最终成功切换namespace，进入container：

```go
root@VM-16-10-ubuntu:~/go/mydocker/mydocker# ./mydocker exec nobody sh
switch namespace [ipc] succeeded
switch namespace [uts] succeeded
switch namespace [net] succeeded
switch namespace [pid] succeeded
switch namespace [mnt] succeeded
/ # ps -ef 
PID   USER     TIME  COMMAND
    1 root      0:00 top
   10 root      0:00 sh
   11 root      0:00 ps -ef
```

不过这里有个小插曲，貌似pid namespace 必须在mount namespace 之前切换，要么就会出问题，不知道为啥。。

## 实现容器停止（docker stop）

stop的实现比较简单，其实就是向对应detach的容器进程发一个SIGTERM信号（kill）。然后修改config文件中的状态即可。

## 容器删除

在容器停止之后，删掉对应的容器信息的目录即可。

## 实现不同容器运行时的镜像隔离

在之前的部分我们仍然遗留了一个问题，如果不同的容器都以detach运行起来，那他们会共用一个目录，导致发生一些冲突问题。这一部分我们想做运行时容器的镜像隔离。

这一步其实也非常简单，其实我们就是把针对不同容器的目录分离解压挂载即可。主要就是在 mnt/ writeLayer/ 下分离出不同的容器 container1 container2等等

## 传递环境变量

这里整体很简单，住的注意的是如果我们一开始是 detach 启动的容器。那么想传递env需要做一些额外的处理。直接设置env的话用docker exec再切进去是看不到env的。

其原因是，```docker exec``` 本质上是另外的一个进程，然后我们通过 ```setns``` 让他切入的之前 detach 启动的docker的namespace中。但是本质他还是一个继承了宿主机env的新进程，而不是容器进程本身。

所以我们需要在exec中加上一个从要切入的容器进程中读取环境变量的过程。那么就从 ```/proc/$PID/environ``` 中读容器进程的环境变量就行，然后读完了：

```go
	// Get container's ENVs
	containerEnvs := getEnvsByPid(pid)
	// Set both host's ENVs and container's ENVs
	cmd.Env = append(os.Environ(), containerEnvs...)
```

让cmd拿到环境变量后再Run()起来。





