package main

import (
	"bufio"
	"bytes"
	//"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	c "github.com/fcadvisor/cpuacct"
	"github.com/fcadvisor/etcdclient"
	g "github.com/fcadvisor/getdata"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Info struct {
	ContainerIP    string
	CpuNum         int
	PreviousCPU    uint64
	PreviousSystem uint64
	Cores          int
}

type SafeContainerinfo struct {
	sync.RWMutex
	M map[string]*Info
}

func (this *SafeContainerinfo) Put(id string, info *Info) {

	this.Lock()
	defer this.Unlock()
	this.M[id] = info
}

/*func (this *SafeContainerinfo) Cores(id string) int {
	this.RLock()
	defer this.RUnlock()
	return this.M[id].Cores
}*/

var Containerinfo = &SafeContainerinfo{M: make(map[string]*Info)}

var localhost string

func main() {
	go cpuinit()
	time.Sleep(1 * time.Second)
	//go dockerinfo()
	for {
		time.Sleep(60 * time.Second)
		collect()
	}
}
func dockerinfo() {
	for {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		tag := ""
		containerid := ""
		//------docker info 上报
		if percent, available, err := g.RequestUnixSocket("/info", "GET"); err != nil {
			log.Errorf("can't connect docker daemon %s", err)
			if err := g.PushIt("0", timestamp, "dockerdaemon.stat", tag, containerid, "GAUGE", localhost); err != nil {
				log.Errorf("Push docker daemon stat err %s", err)
			}
		} else {
			g.PushIt(percent, timestamp, "df.dockerspaceused.percent", tag, "", "GAUGE", localhost)
			g.PushIt(available, timestamp, "df.dataspace.free", tag, "", "GAUGE", localhost)
			if err := g.PushIt("1", timestamp, "dockerdaemon.stat", tag, containerid, "GAUGE", localhost); err != nil {
				log.Errorf("Push docker daemon stat err %s", err)
			}

		}
		time.Sleep(3600 * time.Second)
	}
}

func cpuinit() {
	log.Infoln("Start Init")
	localhost = g.Getip()
	for {
		etcdPath := os.Getenv("ETCDPATH")
		log.Infoln(etcdPath)
		container_list, e := ioutil.ReadDir("/var/lib/docker/containers")
		if e != nil {
			log.Errorln(e, "read dir error")
			return
		}
		//localhost = "172.16.30.100"
		for _, v := range container_list {
			containerid := v.Name()
			Containerinfo.RLock()
			if _, ok := Containerinfo.M[containerid]; ok {
				endpoint, err := queryif(containerid, etcdPath)
				if err != nil {
					log.Errorln("query container ip failed please check etcdpath")
					Containerinfo.RUnlock()
					continue
				}
				if endpoint == "" {
					//log.Infoln("this container is not include in k8s_rc_pod ")
					log.Infof("This container %s does not need to be monitored.Because I can't get IP from etcd. you can check it by docker ps -a command.If it doesn't work,you can delete it", containerid)
					Containerinfo.RUnlock()
					Containerinfo.Lock()
					delete(Containerinfo.M, containerid)
					Containerinfo.Unlock()
					continue
				}
				Containerinfo.RUnlock()
				continue
			}
			Containerinfo.RUnlock()
			endpoint, err := query(containerid, etcdPath)
			if err != nil {
				log.Errorln("query container ip failed please check etcdpath")
				return
			}
			if endpoint == "" {
				//log.Infoln("this container is not include in k8s_rc_pod ")
				continue
			}
			log.Infoln(containerid)
			//Containerinfo[containerid].ContainerIP = "172.17.0.2"
			path := fmt.Sprintf("/cgroup/cpuacct/docker/%s/", containerid)
			usage, system, cpunum, _ := c.GetStats(path)
			dockername, err := g.GetDockerName(containerid)
			if err != nil {
				log.Errorln(err, "from pushDatas.go g.GetDockerName err ..continue ")
				continue
			}
			cores, err := querycores(dockername, etcdPath)
			if err != nil {
				log.Errorln(err, "from pushDatas.go querycores err ..continue ")
				continue
			}
			log.Infof("%s's pod cores is %d", containerid, cores)
			s := &Info{
				endpoint,
				cpunum,
				usage,
				system,
				cores,
			}
			Containerinfo.Put(containerid, s)
			log.Infoln("Start monitoring " + endpoint)
			//Containerinfo[containerid] = s

		}
		time.Sleep(60 * time.Second)
	}
}

func collect() {
	tag := ""
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	Containerinfo.Lock()
	defer Containerinfo.Unlock()
	for i, v := range Containerinfo.M {
		containerid := i
		endpoint := v.ContainerIP
		//endpoint := "172.0.1." + strconv.Itoa(i)
		vip := strings.Split(endpoint, ".")[2] + "." + strings.Split(endpoint, ".")[3]
		log.Infoln(vip)
		if err := g.PushIt(vip, timestamp, string([]byte(containerid)[:12])+"_ip", tag, containerid, "GAUGE", localhost); err != nil {
			log.Errorln(err, "g.PushIt err in push endpoint")
		}
		host := strings.Split(localhost, ".")[2] + "." + strings.Split(localhost, ".")[3]
		log.Infoln(host)
		if err := g.PushIt(host, timestamp, "host_ip", tag, containerid, "GAUGE", endpoint); err != nil {
			log.Errorln(err, "g.PushIt err in push localhost")
		}
		log.Infoln("=========>Begin [" + i + "/" + "] ->" + v.ContainerIP + "=========>")
		DockerInspect, err := g.GetDockerData(containerid)
		if err != nil {
			log.Errorln(err, "from pushDatas.go g.GetDockerData ..continue ")
			continue
		} //---获取物理机docker对应目录下host.config
		dockerpid := gjson.Get(DockerInspect, "State.Pid")
		pid := strconv.Itoa(int(dockerpid.Int()))
		//log.Infoln(pid)
		run := gjson.Get(DockerInspect, "State.Running")
		//log.Infoln(run.Bool())
		if !run.Bool() {
			if err := g.PushIt("0", timestamp, "agent.alive", tag, containerid, "GAUGE", endpoint); err != nil {
				log.Errorln(err, "g.PushIt err in push agent")
			}
			continue
		}
		if err := g.PushIt("1", timestamp, "agent.alive", tag, containerid, "GAUGE", endpoint); err != nil {
			log.Errorln(err, "g.PushIt err in pushagent")
		}
		log.Infoln("Push agent ok " + endpoint)

		log.Infoln("=>" + localhost + "=>" + endpoint) //  可以通过inspect获取容器的IP

		//ROOTFS
		rootfs, _ := getrootfs(fmt.Sprintf("/var/lib/docker/devicemapper/mnt/%s/rootfs", containerid))
		if err := g.PushIt(rootfs, timestamp, "df.rootfsused.percent", tag, containerid, "GAUGE", endpoint); err != nil {
			log.Errorln(err, "g.PushIt err in pushrootfs")
		}

		//---内存上报---------
		if err := pushMem(containerid, timestamp, tag, containerid, endpoint); err != nil { //g.Get cadvisor data about Memery
			log.Errorln(err, "from pushDatas.go pushMem function  ")
		}

		/*if err := pushBlk(containerid, timestamp, tag, containerid, endpoint); err != nil { //g.Get cadvisor data about Memery
			log.Errorln(err, "from pushDatas.go pushMem function  ")
		}*/

		//-----cpu上报------------
		if err := NewPushCPU(v.PreviousCPU, v.PreviousSystem, timestamp, tag, containerid, endpoint, v.Cores); err != nil {
			log.Errorln(err, "from pushDatas.go NewPushCPU function  ")

		}

		//------网卡流量上报--------
		if err := NewPushNetwork(pid, timestamp, tag, containerid, endpoint); err != nil {
			log.Errorln(err, "from pushDatas.go NewPushNetwork function  ")
		}
	}

	// os.Exit(1)
}

// 本函数上报了四个数值，内存使用量，内存最大值，内存百分比， 如果需要上报的更多， 可以增加这个函数的参数。

func pushMem(containerid, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Mem Info")
	/*f, err := os.Open(fmt.Sprintf("/cgroup/memory/docker/%s/memory.stat", containerid))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadString('\n') //以'\n'为结束符读入一行

		if err != nil || io.EOF == err {
			break
		}
		s := strings.Split(s, " ")
		if err := g.PushIt(string(s[1]), timestamp, "container_memory_"+s[0], tags, containerID, "GAUGE", endpoint); err != nil {
			//log.Errorln(err, "g.PushIt err in pushMem")
			return err
		}
	}*/

	var memLimit, memUseage string
	memLimit, _ = g.OpenMemfile(fmt.Sprintf("/cgroup/memory/docker/%s/memory.limit_in_bytes", containerid)) //资源配额是个固定值，取一次即可
	//log.Infoln(memLimit)
	//-----求出memUsage平均值－－－－－－－－
	memUseage, _ = g.OpenMemfile(fmt.Sprintf("/cgroup/memory/docker/%s/memory.usage_in_bytes", containerid)) // 内存用量是个变动数值，把stats数组中的所有数值加起来，求平均，以防止某一个为空造成的取值不准。
	//log.Infoln(memUseage)
	if err := g.PushIt(string(memUseage), timestamp, "mem.memused", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	// 上报内存总量
	if err := g.PushIt(fmt.Sprint(memLimit), timestamp, "mem.memtotal", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	Usage, _ := strconv.ParseFloat(memUseage, 64)
	//log.Infoln(Usage)
	Limit, _ := strconv.ParseFloat(memLimit, 64)
	//log.Infoln(Limit)
	//上报总内存使用率
	memUsagePrecent := Usage / Limit
	log.Infoln(memUsagePrecent) // 注意上报的这个数值已经乘以了100，即：上报的是百分数
	if err := g.PushIt(fmt.Sprintf("%.3f", memUsagePrecent*100), timestamp, "mem.memused.percent", tags, containerID, "GAUGE", endpoint); err != nil {
		// log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	return nil
}
func NewPushCPU(precpu, presystem uint64, timestamp, tags, containerID, endpoint string, cores int) error {
	log.Infoln("begin to push Cpu Info")
	var cpu, system uint64
	var cpunum int
	path := fmt.Sprintf("/cgroup/cpuacct/docker/%s/", containerID)
	cpu, system, cpunum, _ = c.GetStats(path)
	s := &Info{
		endpoint,
		cpunum,
		cpu,
		system,
		cores,
	}
	//Containerinfo[containerID] = s
	Containerinfo.M[containerID] = s
	cpuPercent := 0.0
	// calculate the change for the cpu usage of the container in between readings
	cpuDelta := float64(cpu - precpu)
	// calculate the change for the entire system between readings
	systemDelta := float64(system - presystem)
	//log.Infof("cpunum is %d cores is %d cpudelta is %f systemdelta is %f", cpunum, cores, cpuDelta, systemDelta)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		//docker stats data
		//cpuPercent = (cpuDelta / systemDelta) * float64(cpunum) * float64(100)
		//actual data
		cpuPercent = (cpuDelta / systemDelta) * float64(cpunum) / float64(cores) * float64(100)
		//log.Infof("current CpuPercent is %f", cpuPercent)
		//log.Infof("divide cpunum CpuPercent is %f", cpuPercent/float64(cpunum))
		//log.Infof("real CpuPercent is %f", cpuPercent/float64(100)/float64(cores))
		//cpuPercent = (cpuDelta / systemDelta) * 100.0
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", cpuPercent), timestamp, "cpu.busy", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.busy err %s", err)
		return err
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", float64(100)-cpuPercent), timestamp, "cpu.idle", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.idle err %s", err)
		return err
	}
	/*userModeUsage, kernelModeUsage, err := getCpuUsageBreakdown(path)
	if err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(userModeUsage), timestamp, "container_cpu_usage_seconds_total", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push container_cpu_usage_seconds_total %s", err)
		return err
	}
	if err := g.PushIt(fmt.Sprint(kernelModeUsage), timestamp, "container_cpu_system_seconds_total", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push container_cpu_system_seconds_total %s", err)
		return err
	}
	if err := g.PushIt(fmt.Sprint(cpunum), timestamp, "container_cpu_cores", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push container_cpu_cores %s", err)
		return err
	}
	f, err := os.Open(filepath.Join(path, "cpu.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return err
		}
		switch t {
		case "nr_periods":
			//stats.CpuStats.ThrottlingData.Periods = v
			if err := g.PushIt(fmt.Sprint(v), timestamp, "container_cpu_nr_periods", tags, containerID, "GAUGE", endpoint); err != nil {
				log.Errorf("Push container_cpu_nr_periods %s", err)
				return err
			}

		case "nr_throttled":
			//stats.CpuStats.ThrottlingData.ThrottledPeriods = v
			if err := g.PushIt(fmt.Sprint(v), timestamp, "container_cpu_nr_throttled", tags, containerID, "GAUGE", endpoint); err != nil {
				log.Errorf("Push container_cpu_nr_throttled %s", err)
				return err
			}

		case "throttled_time":
			//stats.CpuStats.ThrottlingData.ThrottledTime = v
			if err := g.PushIt(fmt.Sprint(v), timestamp, "container_cpu_throttled_time", tags, containerID, "GAUGE", endpoint); err != nil {
				log.Errorf("Push container_cpu_throttled_time %s", err)
				return err
			}
		}
	}*/
	return nil
}

func NewPushNetwork(pid, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Net Info")
	path := fmt.Sprintf("/proc/%s/net/dev", pid)
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorln(err)
		return err
	}
	reader := bufio.NewReader(bytes.NewBuffer(contents))
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			break
		}
		if strings.Contains(line, "eth") {
			fields := strings.Fields(line)
			//tags := fields[0]
			tags := ""
			InBytes := fields[1]
			if err := g.PushIt(InBytes, timestamp, "net.if.in.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			InPackages := fields[2]
			if err := g.PushIt(InPackages, timestamp, "net.if.in.packets", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			InErrors := fields[3]
			if err := g.PushIt(InErrors, timestamp, "net.if.in.errors", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			InDropped := fields[4]
			if err := g.PushIt(InDropped, timestamp, "net.if.in.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			OutBytes := fields[9]
			if err := g.PushIt(OutBytes, timestamp, "net.if.out.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			OutPackages := fields[10]
			if err := g.PushIt(OutPackages, timestamp, "net.if.out.packets", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			OutErrors := fields[11]
			if err := g.PushIt(OutErrors, timestamp, "net.if.out.errors", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			OutDropped := fields[12]
			if err := g.PushIt(OutDropped, timestamp, "net.if.out.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			/*TotalBytes := InBytes + OutBytes
			if err := g.PushIt(TotalBytes, timestamp, "net.if.total.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			TotalPackages := InPackages + OutPackages
			if err := g.PushIt(TotalPackages, timestamp, "net.if.total.packets", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			TotalErrors := InErrors + OutErrors
			if err := g.PushIt(TotalErrors, timestamp, "net.if.total.errors", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			TotalDropped := InDropped + OutDropped
			if err := g.PushIt(TotalDropped, timestamp, "net.if.total.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
				return err
			}
			if err := scanTcpStats(fmt.Sprintf("/proc/%s/net/tcp", pid), timestamp, tags, containerID, endpoint); err != nil {
				return err
			}
			if err := scanTcpStats(fmt.Sprintf("/proc/%s/net/udp", pid), timestamp, tags, containerID, endpoint); err != nil {
				return err
			}*/
		}
	}
	return nil
}

func query(containerid string, etcdPath string) (string, error) {
	log.Infoln("Start query")
	client, err := etcdclient.NewEtcdClient(etcdPath)
	if err != nil {
		log.Errorf("cli.query():%+v\n", err)
		return "", err
	}

	//query from etcd
	ip, err := client.QueryContainerid(containerid)
	if err != nil {
		log.Warnf("cli.query():%+v\n", err)
	} else {
		log.Infof("cli.query(): result ip=%+v", ip)
	}

	log.Infoln("cli.query():Query success")
	return ip, nil
}

func queryif(containerid string, etcdPath string) (string, error) {
	client, err := etcdclient.NewEtcdClient(etcdPath)
	if err != nil {
		log.Errorf("cli.query():%+v\n", err)
		return "", err
	}
	//query from etcd
	ip, err := client.QueryContainerid(containerid)
	if err != nil {
		log.Warnf("cli.query():%+v\n", err)
	}
	return ip, nil
}

func querycores(podname string, etcdPath string) (int, error) {
	log.Infoln("Start query cores")
	client, err := etcdclient.NewEtcdClient(etcdPath)
	if err != nil {
		log.Errorf("cli.querycores():%+v\n", err)
		return 0, err
	}
	// try to query from idspath
	cores, err := client.QueryContainerCores(podname)
	if err != nil {
		log.Errorf("cli.querycores():%+v\n", err)
		return 0, err
	}
	return cores, nil
}

/*func scanTcpStats(tcpStatsFile, timestamp, tags, containerID, endpoint string) error {

	var stats g.TcpStat

	data, err := ioutil.ReadFile(tcpStatsFile)
	if err != nil {
		return stats, fmt.Errorf("failure opening %s: %v", tcpStatsFile, err)
	}

	tcpStateMap := map[string]uint64{
		"01": 0, //ESTABLISHED
		"02": 0, //SYN_SENT
		"03": 0, //SYN_RECV
		"04": 0, //FIN_WAIT1
		"05": 0, //FIN_WAIT2
		"06": 0, //TIME_WAIT
		"07": 0, //CLOSE
		"08": 0, //CLOSE_WAIT
		"09": 0, //LAST_ACK
		"0A": 0, //LISTEN
		"0B": 0, //CLOSING
	}

	reader := strings.NewReader(string(data))
	scanner := bufio.NewScanner(reader)

	scanner.Split(bufio.ScanLines)

	// Discard header line
	if b := scanner.Scan(); !b {
		return stats, scanner.Err()
	}

	for scanner.Scan() {
		line := scanner.Text()

		state := strings.Fields(line)
		// TCP state is the 4th field.
		// Format: sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt  uid timeout inode
		tcpState := state[3]
		_, ok := tcpStateMap[tcpState]
		if !ok {
			return stats, fmt.Errorf("invalid TCP stats line: %v", line)
		}
		tcpStateMap[tcpState]++
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["01"]), timestamp, "container_network_tcp_Established", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["02"]), timestamp, "container_network_tcp_SynSent", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["03"]), timestamp, "container_network_tcp_SynRecv", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["04"]), timestamp, "container_network_tcp_FinWait1", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["05"]), timestamp, "container_network_tcp_FinWait2", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["06"]), timestamp, "container_network_tcp_TimeWait", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["07"]), timestamp, "container_network_tcp_Close", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["08"]), timestamp, "container_network_tcp_CloseWait", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["09"]), timestamp, "container_network_tcp_LastAck", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["0A"]), timestamp, "container_network_tcp_Listen", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(tcpStateMap["0B"]), timestamp, "container_network_tcp_Closing", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	return nil
}

func udpStatsFromProc(udpStatsFile, timestamp, tags, containerID, endpoint string) error {
	var err error
	var udpStats g.UdpStat
	r, err := os.Open(udpStatsFile)
	if err != nil {
		return udpStats, fmt.Errorf("failure opening %s: %v", udpStatsFile, err)
	}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	// Discard header line
	if b := scanner.Scan(); !b {
		return stats, scanner.Err()
	}

	listening := uint64(0)
	dropped := uint64(0)
	rxQueued := uint64(0)
	txQueued := uint64(0)

	for scanner.Scan() {
		line := scanner.Text()
		// Format: sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt  uid timeout inode ref pointer drops

		listening++

		fs := strings.Fields(line)
		if len(fs) != 13 {
			continue
		}

		rx, tx := uint64(0), uint64(0)
		fmt.Sscanf(fs[4], "%X:%X", &rx, &tx)
		rxQueued += rx
		txQueued += tx

		d, err := strconv.Atoi(string(fs[12]))
		if err != nil {
			continue
		}
		dropped += uint64(d)
	}
	if err := g.PushIt(fmt.Sprint(listening), timestamp, "container_network_udp_Listen", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(dropped), timestamp, "container_network_udp_Dropped", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(rxQueued), timestamp, "container_network_udp_RxQueued", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(fmt.Sprint(txQueued), timestamp, "container_network_udp_TxQueued", tags, containerID, "GAUGE", endpoint); err != nil {
		return err
	}
	return nil
}*/

/*func pushBlk(containerid, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Blk Info")
	f, err := os.Open(fmt.Sprintf("/cgroup/blkio/docker/%s/blkio.throttle.io_service_bytes", containerid))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadString('\n') //以'\n'为结束符读入一行

		if err != nil || io.EOF == err {
			break
		}
		s := strings.Split(s, " ")
		if len(s) == 3 {
			if err := g.PushIt(string(s[2]), timestamp, "blkio.throttle.io_service_bytes_"+s[1], tags, containerID, "GAUGE", endpoint); err != nil {
				//log.Errorln(err, "g.PushIt err in pushMem")
				return err
			}
		}
	}
	return nil
}*/
/*func getrootfs(path string) string {
	fileInfo, _ := os.Stat(path)
	//文件大小
	filesize := fileInfo.Size()
	size := filesize / int64(1024*1024*1024) //返回的是G
	return fmt.Sprint(size)
}*/
func getrootfs(s string) (string, error) {
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	command := "du -sh " + s + "|awk '{print $1}'"
	cmd := exec.Command("/bin/sh", "-c", command)

	//读取io.Writer类型的cmd.Stdout，再通过bytes.Buffer(缓冲byte类型的缓冲器)将byte类型转化为string类型(out.String():这是bytes类型提供的接口)
	var out bytes.Buffer
	cmd.Stdout = &out

	//Run执行c包含的命令，并阻塞直到完成。  这里stdout被取出，cmd.Wait()无法正确获取stdin,stdout,stderr，则阻塞在那了
	err := cmd.Run()
	v := strings.TrimSpace(out.String())
	var ok bool
	if ok = strings.HasSuffix(v, "K"); ok {
		return "0", err
	}
	if ok = strings.HasSuffix(v, "G"); ok {
		value := strings.TrimRight(v, "G")
		return value, err
	}
	if ok = strings.HasSuffix(v, "M"); ok {
		return "1", err
	}

	return "", err
}
