//main 本函数向flacon客户端推送数据
package getdata

import (
	"io/ioutil"
	"net/http"
	//"fmt"
	//"bufio"
	log "github.com/sirupsen/logrus"
	//"io"
	//"os"
	"strings"
)

func PushIt(value, timestamp, metric, tags, containerId, counterType, endpoint string) error {

	postThing := `[{"metric": "` + metric + `", "endpoint": "` + endpoint + `", "timestamp": ` + timestamp + `,"step": ` + "60" + `,"value": ` + value + `,"counterType": "` + counterType + `","tags": "` + tags + `"}]`
	log.Infoln(postThing)
	//push data to falcon-agent
	url := "http://127.0.0.1:1988/v1/push/container"
	resp, err := http.Post(url,
		"application/x-www-form-urlencoded",
		strings.NewReader(postThing))
	if err != nil {
		log.Errorln("Post err in pushIt +%s", err)
		return err
	}
	defer resp.Body.Close()
	_, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		log.Errorln("ReadAll err in pushIt + %s", err1)
		return err1
	}
	log.Infoln("push " + metric + " ok")
	return nil
}

/*func PushMemstats(path, timestamp string) {
	f, err := os.Open(path)
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
		s := strings.Split(line, " ")
		metric := "container_memory_" + s[0]
		if err := g.PushIt(s[1], timestamp, metric, tags, containerID, "GAUGE", endpoint); err != nil {
			// log.Errorln(err, "g.PushIt err in pushMem")
			return err
		}
	}
}*/

func OpenMemfile(path string) (string, error) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	return strings.TrimSpace(string(dat)), nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
