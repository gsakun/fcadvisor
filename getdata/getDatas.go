//main  获取数据的，具体的动作函数都在这里
package getdata

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	//"github.com/gooops/micadvisor_open/docker"
)

//v0.25 默认暴露8080端口
//var CadvisorPort = "18080"
type DInfo struct {
	ID                 string
	Containers         int
	Images             int
	Driver             string
	DriverStatus       [][2]string
	MemoryLimit        bool
	SwapLimit          bool
	CpuCfsPeriod       bool
	CpuCfsQuota        bool
	IPv4Forwarding     bool
	Debug              bool
	NFd                int
	OomKillDisable     bool
	NGoroutines        int
	SystemTime         string
	ExecutionDriver    string
	LoggingDriver      string
	NEventsListener    int
	KernelVersion      string
	OperatingSystem    string
	IndexServerAddress string
	RegistryConfig     interface{}
	InitSha1           string
	InitPath           string
	NCPU               int
	MemTotal           int64
	DockerRootDir      string
	HttpProxy          string
	HttpsProxy         string
	NoProxy            string
	Name               string
	Labels             []string
	ExperimentalBuild  bool
}

var info = &DInfo{}

func GetDockerData(containerId string) (string, error) {
	fi, err := os.Open("/var/lib/docker/containers/" + containerId + "/config.v2.json")
	if err != nil {
		log.Errorln("get docker data failed")
	}
	defer fi.Close()
	fd, err := ioutil.ReadAll(fi)
	// fmt.Println(string(fd))
	return string(fd), nil
} //zk
func Getip() string {
	var ip string
	address, err := net.InterfaceByName(Config().AdapterName)
	if err != nil {
		//log.Infoln("failed to get br0 ip start query bond0 ip")
		log.Errorln("failed to get adapter ip")
	}
	ip_info, _ := address.Addrs()
	ip2 := strings.Split(ip_info[0].String(), "/")
	ip = ip2[0]
	log.Infoln("localendpoint " + " " + ip)
	return ip
}

func GetContainersJson() string {
	addr := "/containers/json"
	body, err := RequestUnixSocket2(addr, "GET")
	if err != nil {
		log.Errorln("Error request container stats", err)
	}
	//log.Infoln(string(body))
	return string(body)
}

func GetContainerStats(containerId string) string {
	addr := "/containers/" + containerId + "/stats?stream=0"
	body, err := RequestUnixSocket2(addr, "GET")
	if err != nil {
		log.Errorln("Error request container stats", err)
	}
	//log.Infoln(string(body))
	return string(body)
}

func RequestUnixSocket2(address, method string) ([]byte, error) {
	DOCKER_UNIX_SOCKET := "unix:///var/run/docker.sock"
	// Example: unix:///var/run/docker.sock:/images/json?since=1374067924
	unix_socket_url := DOCKER_UNIX_SOCKET + ":" + address
	u, err := url.Parse(unix_socket_url)
	if err != nil || u.Scheme != "unix" {
		log.Errorln("Error to parse unix socket url "+unix_socket_url, err)
		return nil, err
	}

	hostPath := strings.Split(u.Path, ":")
	u.Host = hostPath[0]
	u.Path = hostPath[1]

	conn, err := net.Dial("unix", u.Host)
	if err != nil {
		log.Errorln("Error to connect to"+u.Host, err)
		// fmt.Println("Error to connect to", u.Host, err)
		return nil, err
	}

	reader := strings.NewReader("")
	query := ""
	if len(u.RawQuery) > 0 {
		query = "?" + u.RawQuery
	}

	request, err := http.NewRequest(method, u.Path+query, reader)
	if err != nil {
		log.Errorln("Error to create http request", err)
		// fmt.Println("Error to create http request", err)
		return nil, err
	}

	client := httputil.NewClientConn(conn, nil)
	response, err := client.Do(request)
	if err != nil {
		log.Errorln("Error to achieve http request over unix socket", err)
		// fmt.Println("Error to achieve http request over unix socket", err)
		return nil, err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		//log.Errorln("Error, get invalid body in answer", err)
		// fmt.Println("Error, get invalid body in answer")
		fmt.Println(err)
	}
	return body, err
}

//RequestUnixSocket 使用docker自身的api获取数据
func RequestUnixSocket(address, method string) (string, string, error) {
	DOCKER_UNIX_SOCKET := "unix:///var/run/docker.sock"
	// Example: unix:///var/run/docker.sock:/images/json?since=1374067924
	unix_socket_url := DOCKER_UNIX_SOCKET + ":" + address
	u, err := url.Parse(unix_socket_url)
	if err != nil || u.Scheme != "unix" {
		log.Errorln(err, "getDatas.go  Error to parse unix socket url "+unix_socket_url)
		return "", "", err
	}

	hostPath := strings.Split(u.Path, ":")
	u.Host = hostPath[0]
	u.Path = hostPath[1]

	conn, err := net.DialTimeout("unix", u.Host, time.Second*30)
	if err != nil {
		log.Errorln(err, "getDatas.go  Error to connect to"+u.Host)
		// fmt.Println("Error to connect to", u.Host, err)
		return "", "", err
	}

	reader := strings.NewReader("")
	query := ""
	if len(u.RawQuery) > 0 {
		query = "?" + u.RawQuery
	}

	request, err := http.NewRequest(method, u.Path+query, reader)
	if err != nil {
		log.Errorln(err, "getDatas.go Error to create http request")
		// fmt.Println("Error to create http request", err)
		return "", "", err
	}

	client := httputil.NewClientConn(conn, nil)
	response, err := client.Do(request)
	if err != nil {
		log.Errorln(err, "getDatas.go  Error to achieve http request over unix socket")
		// fmt.Println("Error to achieve http request over unix socket", err)
		return "", "", err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(err, "getDatas.go  Error, get invalid body in answer")
		// fmt.Println("Error, get invalid body in answer")
		return "", "", err
	}
	log.Infoln(body)
	defer response.Body.Close()
	if err := json.Unmarshal(body, info); err != nil {
		log.Errorf("Error decode info %s", err)
	}
	spaceUsed := info.DriverStatus[5][1]
	spaceTotal := info.DriverStatus[6][1]
	spaceAvailable := info.DriverStatus[7][1]
	sUsed, _ := strConvert(spaceUsed)
	sTotal, _ := strConvert(spaceTotal)
	sAvailable, _ := strConvert(spaceAvailable)
	sUsedPercent := (sUsed / sTotal) * 100.0
	dataSapceUsedPercent := fmt.Sprint(sUsedPercent)
	return dataSapceUsedPercent, fmt.Sprint(sAvailable), err
}
func strConvert(str string) (float64, error) {
	var (
		strf float64
		err  error
	)

	if strings.Contains(str, "GB") {
		str = strings.Replace(str, " GB", "", -1)
		strf, err = strconv.ParseFloat(str, 64)
		if err != nil {
			log.Errorln(err, "Error when strconv to fload")
			return 0, err
		}

		return strf, err
	}

	return 1.0, err
}
