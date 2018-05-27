package getdata

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/toolkits/file"
	"sync/atomic"
	"unsafe"
)

type GlobalConfig struct {
	DB          string `json:"db"`
	IpField     string `json:"ipfield"`
	IdField     string `json:"idfield"`
	AdapterName string `json:"adaptername"`
	Table       string `json:"table"`
}

var (
	ConfigFile string
	ptr        unsafe.Pointer
)

func Config() *GlobalConfig {
	return (*GlobalConfig)(atomic.LoadPointer(&ptr))
}

func ParseConfig(cfg string) {
	if cfg == "" {
		log.Fatalln("config file not specified: use -c $filename")
	}

	if !file.IsExist(cfg) {
		log.Fatalln("config file specified not found:", cfg)
	}

	ConfigFile = cfg

	configContent, err := file.ToTrimString(cfg)
	if err != nil {
		log.Fatalln("read config file", cfg, "error:", err.Error())
	}

	var c GlobalConfig
	err = json.Unmarshal([]byte(configContent), &c)
	if err != nil {
		log.Fatalln("parse config file", cfg, "error:", err.Error())
	}

	// set config
	atomic.StorePointer(&ptr, unsafe.Pointer(&c))

	log.Println("g.ParseConfig ok, file", cfg)
}
