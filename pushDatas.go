package main

import (
	"bytes"
	//"encoding/json"
	//"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	g "github.com/fcadvisor/getdata"
	"github.com/tidwall/gjson"
	//"regexp"
	//"io"
	//"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 1 || len(os.Args) != 2 {
		log.Infoln("Usage: ./fcadvisor eth1")
		os.Exit(1)
	}
	if os.Args[1] == "help" {
		log.Infoln("Usage: ./fcadvisor eth1")
		os.Exit(1)
	}
	adpname := os.Args[1]
	localhost := getlocalhost(adpname)
	//go dockerinfo(localhost)
	for {
		/*	containerid := ""
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
			}*/
		s := g.GetContainersJson()
		//log.Infoln(s)
		m := gjson.Parse(s).Array()
		for _, i := range m {
			//log.Infoln(i.Get("Path").String())
			/*if !strings.Contains(i.Get("Path").String(), "/bin/bash") {
				continue
			}*/
			//log.Infoln(m)
			containerid := i.Get("Id").String()
			//endpoint := i.Get("NetworkSettings.Networks.bridge.IPAddress").String()
			//log.Infoln(endpoint)
			endpoint := getcip(containerid)
			if endpoint == "" {
				//log.Errorln("Get the virtual machine Ip failed.")
				continue
			}
			l := []byte(g.GetContainerStats(containerid))
			//log.Infoln(string(l))
			cpu := gjson.GetBytes(l, "cpu_stats.cpu_usage.total_usage").Uint()
			precpu := gjson.GetBytes(l, "precpu_stats.cpu_usage.total_usage").Uint()
			system := gjson.GetBytes(l, "cpu_stats.system_cpu_usage").Uint()
			presystem := gjson.GetBytes(l, "precpu_stats.system_cpu_usage").Uint()
			lenth := len(gjson.GetBytes(l, "cpu_stats.cpu_usage.percpu_usage").Array())

			memusage := gjson.GetBytes(l, "memory_stats.usage").String()
			memlimit := gjson.GetBytes(l, "memory_stats.limit").String()

			rxbytes := gjson.GetBytes(l, "networks.eth0.rx_bytes").String()
			rxpackets := gjson.GetBytes(l, "networks.eth0.rx_packets").String()
			rxerrors := gjson.GetBytes(l, "networks.eth0.rx_errors").String()
			rxdropped := gjson.GetBytes(l, "networks.eth0.rx_dropped").String()
			txbytes := gjson.GetBytes(l, "networks.eth0.tx_bytes").String()
			txpackets := gjson.GetBytes(l, "networks.eth0.tx_packets").String()
			txerrors := gjson.GetBytes(l, "networks.eth0.tx_errors").String()
			txdropped := gjson.GetBytes(l, "networks.eth0.tx_dropped").String()

			timestamp := fmt.Sprintf("%d", time.Now().Unix())
			tag := ""

			vip := strings.Split(endpoint, ".")[2] + "." + strings.Split(endpoint, ".")[3]
			//log.Infoln(vip)
			if err := g.PushIt(vip, timestamp, string([]byte(containerid)[:12])+"_ip", tag, containerid, "GAUGE", localhost); err != nil {
				log.Errorln(err, "g.PushIt err in push endpoint")
			}
			host := strings.Split(localhost, ".")[2] + "." + strings.Split(localhost, ".")[3]
			log.Infoln(host)
			if err := g.PushIt(host, timestamp, "host_ip", tag, containerid, "GAUGE", endpoint); err != nil {
				log.Errorln(err, "g.PushIt err in push localhost")
			}
			if err := g.PushIt("1", timestamp, "agent.alive", tag, containerid, "GAUGE", endpoint); err != nil {
				log.Errorln(err, "g.PushIt err in pushagent")
			}
			log.Infoln("Push agent ok " + endpoint)

			log.Infoln("=>" + localhost + "=>" + endpoint)
			//DockerInspect, err := g.GetDockerData(containerid)
			//if err != nil {
			//	log.Errorln(err, "from pushDatas.go g.GetDockerData ..continue ")
			//	continue
			//} //---获取物理机docker对应目录下host.config
			//dockerpid := gjson.Get(DockerInspect, "State.Pid")
			//pid := strconv.Itoa(int(dockerpid.Int()))
			//log.Infoln(pid)
			if err := g.PushIt("1", timestamp, "agent.alive", tag, containerid, "GAUGE", endpoint); err != nil {
				log.Errorln(err, "g.PushIt err in pushagent")
			}
			log.Infoln("Push agent ok " + endpoint)

			log.Infoln("=>" + localhost + "=>" + endpoint) //  可以通过inspect获取容器的IP

			//---内存上报---------
			/*if err := pushMem(containerid, timestamp, tag, containerid, endpoint); err != nil {
				log.Errorln(err, "from pushDatas.go pushMem function  ")
			}*/
			if err := pushMem(memusage, memlimit, timestamp, tag, containerid, endpoint); err != nil {
				log.Errorln(err, "from pushDatas.go pushMem function  ")
			}
			//------网卡流量上报--------
			/*if err := NewPushNetwork(pid, timestamp, tag, containerid, endpoint); err != nil {
				log.Errorln(err, "from pushDatas.go NewPushNetwork function  ")
			}*/
			if err := NewPushNetwork(rxbytes, rxpackets, rxerrors, rxdropped, txbytes, txpackets, txerrors, txdropped, timestamp, tag, containerid, endpoint); err != nil {
				log.Errorln(err, "from pushDatas.go NewPushNetwork function  ")
			}

			//-----cpu上报------------
			if err := NewPushCPU(precpu, cpu, system, presystem, lenth, timestamp, tag, containerid, endpoint); err != nil {
				log.Errorln(err, "from pushDatas.go NewPushCPU function  ")
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func pushMem(memusage, memlimit string, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Mem Info")
	if err := g.PushIt(memusage, timestamp, "mem.memused", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	// 上报内存总量
	if err := g.PushIt(memlimit, timestamp, "mem.memtotal", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	Usage, _ := strconv.ParseFloat(memusage, 64)
	//log.Infoln(Usage)
	Limit, _ := strconv.ParseFloat(memlimit, 64)
	//log.Infoln(Limit)
	memUsagePrecent := Usage / Limit
	log.Infoln(memUsagePrecent) // 注意上报的这个数值已经乘以了100，即：上报的是百分数
	if err := g.PushIt(fmt.Sprintf("%.3f", memUsagePrecent*100), timestamp, "mem.memused.percent", tags, containerID, "GAUGE", endpoint); err != nil {
		// log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	return nil
}

// 本函数上报了四个数值，内存使用量，内存最大值，内存百分比， 如果需要上报的更多， 可以增加这个函数的参数。

/*func pushMem(meminfo t.Stats, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Mem Info")
	memLimit := meminfo.MemoryStats.Limit //资源配额是个固定值，取一次即可
	//log.Infoln(memLimit)
	//-----求出memUsage平均值－－－－－－－－
	memUseage := meminfo.MemoryStats.Usage // 内存用量是个变动数值，把stats数组中的所有数值加起来，求平均，以防止某一个为空造成的取值不准。
	//log.Infoln(memUseage)
	if err := g.PushIt(string(memUseage), timestamp, "mem.memused", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	// 上报内存总量
	if err := g.PushIt(string(memLimit), timestamp, "mem.memtotal", tags, containerID, "GAUGE", endpoint); err != nil {
		//log.Errorln(err, "g.PushIt err in pushMem")
		return err
	}
	Usage, _ := strconv.ParseFloat(string(memUseage), 64)
	//log.Infoln(Usage)
	Limit, _ := strconv.ParseFloat(string(memLimit), 64)
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
func NewPushCPU(cpuinfo t.Stats, timestamp, tags, containerID, endpoint string) error {
	cpuPercent := 0.0
	// calculate the change for the cpu usage of the container in between readings
	cpuDelta := float64(cpuinfo.CpuStats.CpuUsage.TotalUsage - cpuinfo.PreCpuStats.CpuUsage.TotalUsage)
	// calculate the change for the entire system between readings
	systemDelta := float64(cpuinfo.CpuStats.SystemUsage - cpuinfo.PreCpuStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(cpuinfo.CpuStats.CpuUsage.PercpuUsage)) * 100.0
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", cpuPercent), timestamp, "cpu.busy", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.busy err %s", err)
		return err
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", float64(100)-cpuPercent), timestamp, "cpu.idle", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.idle err %s", err)
		return err
	}
	return nil
}*/

func NewPushNetwork(rb, rp, re, rd, tb, tp, te, td, timestamp, tags, containerID, endpoint string) error {

	if err := g.PushIt(rb, timestamp, "net.if.in.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(rp, timestamp, "net.if.in.packets", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(re, timestamp, "net.if.in.errors", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(rb, timestamp, "net.if.in.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(tb, timestamp, "net.if.out.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(tp, timestamp, "net.if.out.packets", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(te, timestamp, "net.if.out.errors", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	if err := g.PushIt(td, timestamp, "net.if.out.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	/*TotalBytes := tb + rb
	if err := g.PushIt(string(TotalBytes), timestamp, "net.if.total.bytes", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	TotalPackages := tp + rp
	if err := g.PushIt(string(TotalPackages), timestamp, "net.if.total.packets", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	TotalErrors := te + re
	if err := g.PushIt(string(TotalErrors), timestamp, "net.if.total.errors", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}
	TotalDropped := td + rd
	if err := g.PushIt(string(TotalDropped), timestamp, "net.if.total.dropped", tags, containerID, "COUNTER", endpoint); err != nil {
		return err
	}*/
	return nil
}

func dockerinfo(localhost string) {
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

func NewPushCPU(precpu, cpu, system, presystem uint64, cpunum int, timestamp, tags, containerID, endpoint string) error {
	cpuPercent := 0.0
	// calculate the change for the cpu usage of the container in between readings
	cpuDelta := float64(cpu - precpu)
	// calculate the change for the entire system between readings
	systemDelta := float64(system - presystem)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(cpunum) * 100.0
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", cpuPercent), timestamp, "cpu.busy", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.busy err %s", err)
		return err
	}
	if err := g.PushIt(fmt.Sprintf("%.5f", float64(100)-cpuPercent), timestamp, "cpu.idle", tags, containerID, "GAUGE", endpoint); err != nil {
		log.Errorf("Push cpu.idle err %s", err)
		return err
	}
	return nil
}

func getcip(cid string) string {
	p := bytes.NewBuffer(nil)
	cmd := "docker exec " + cid + " ifconfig eth0|grep inet |awk '{print $2}'|sed 's/addr://g'"
	//log.Infoln(cmd)
	command := exec.Command("/bin/sh", "-c", cmd)
	command.Stdout = p
	command.Run()
	f := string(p.Bytes())
	r := strings.TrimSpace(f)
	log.Infoln(r)
	return r
}

func getlocalhost(adpname string) string {
	var ip string
	address, err := net.InterfaceByName(adpname)
	if err != nil {
		log.Fatalln("failed to get localhost ip,please check network card name")
	}
	ip_info, _ := address.Addrs()
	ipn := strings.Split(ip_info[0].String(), "/")
	ip = ipn[0]
	log.Infoln(ip)
	return ip
}

/*func pushMem(containerid, timestamp, tags, containerID, endpoint string) error {
	log.Infoln("begin to push Mem Info")
	var memLimit, memUseage string
	memLimit, _ = g.OpenMemfile(fmt.Sprintf("/sys/fs/cgroup/memory/docker/%s/memory.limit_in_bytes", containerid)) //资源配额是个固定值，取一次即可
	//log.Infoln(memLimit)
	//-----求出memUsage平均值－－－－－－－－
	memUseage, _ = g.OpenMemfile(fmt.Sprintf("/sys/fs/cgroup/memory/docker/%s/memory.usage_in_bytes", containerid)) // 内存用量是个变动数值，把stats数组中的所有数值加起来，求平均，以防止某一个为空造成的取值不准。
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
}*/

/*func NewPushNetwork(pid, timestamp, tags, containerID, endpoint string) error {
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
			tags := fields[0]
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
		}
	}
	return nil
}*/
