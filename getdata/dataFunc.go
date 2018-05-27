//main 本函数向flacon客户端推送数据
package getdata

import (
	//"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
)

func PushIt(value, timestamp, metric, tags, containerId, counterType, endpoint string) error {

	postThing := `[{"metric": "` + metric + `", "endpoint": "` + endpoint + `", "timestamp": ` + timestamp + `,"step": ` + "60" + `,"value": ` + value + `,"counterType": "` + counterType + `","tags": "` + tags + `"}]`
	log.Infoln(postThing)
	//push data to falcon-agent
	url := "http://127.0.0.1:1988/v1/push"
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
func check(e error) {
	if e != nil {
		panic(e)
	}
}
func OpenMemfile(path string) (string, error) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	return strings.TrimSpace(string(dat)), nil
}
