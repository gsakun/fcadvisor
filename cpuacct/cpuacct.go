package cpuacct

import (
	"bufio"
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	cgroupCpuacctStat   = "cpuacct.stat"
	nanosecondsInSecond = 1000000000
)

var clockTicks = uint64(GetClockTicks())

func GetStats(path string) (uint64, uint64, int, error) {
	totalUsage, err := getCgroupParamUint(path, "cpuacct.usage")
	if err != nil {
		log.Errorln(err)
	}

	percpuUsage, err := getPercpuUsage(path)
	if err != nil {
		log.Errorln(err)
	}

	systemUsage, err := getSystemCpuUsage()
	if err != nil {
		log.Errorln(err)
	}
	return totalUsage, systemUsage, len(percpuUsage), nil
}

func getPercpuUsage(path string) ([]uint64, error) {
	percpuUsage := []uint64{}
	data, err := ioutil.ReadFile(filepath.Join(path, "cpuacct.usage_percpu"))
	if err != nil {
		return percpuUsage, err
	}
	for _, value := range strings.Fields(string(data)) {
		value, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return percpuUsage, fmt.Errorf("Unable to convert param value to uint64: %s", err)
		}
		percpuUsage = append(percpuUsage, value)
	}
	return percpuUsage, nil
}

const nanoSeconds = 1e9

// getSystemCpuUSage returns the host system's cpu usage in nanoseconds
// for the system to match the cgroup readings are returned in the same format.
func getSystemCpuUsage() (uint64, error) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		log.Errorln(err)
		return 0, err
	}
	reader := bufio.NewReader(bytes.NewBuffer(contents))
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var sum uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				sum += v
			}
			return (sum * nanoSeconds) / clockTicks, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format")
}

/*func getCpuUsageBreakdown(path string) (uint64, uint64, error) {
	userModeUsage := uint64(0)
	kernelModeUsage := uint64(0)
	const (
		userField   = "user"
		systemField = "system"
	)

	// Expected format:
	// user <usage in ticks>
	// system <usage in ticks>
	data, err := ioutil.ReadFile(filepath.Join(path, cgroupCpuacctStat))
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		return 0, 0, fmt.Errorf("failure - %s is expected to have 4 fields", filepath.Join(path, cgroupCpuacctStat))
	}
	if fields[0] != userField {
		return 0, 0, fmt.Errorf("unexpected field %q in %q, expected %q", fields[0], cgroupCpuacctStat, userField)
	}
	if fields[2] != systemField {
		return 0, 0, fmt.Errorf("unexpected field %q in %q, expected %q", fields[2], cgroupCpuacctStat, systemField)
	}
	if userModeUsage, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
		return 0, 0, err
	}
	if kernelModeUsage, err = strconv.ParseUint(fields[3], 10, 64); err != nil {
		return 0, 0, err
	}

	return (userModeUsage * nanosecondsInSecond) / clockTicks, (kernelModeUsage * nanosecondsInSecond) / clockTicks, nil
}*/

func getCgroupParamUint(cgroupPath, cgroupFile string) (uint64, error) {
	contents, err := ioutil.ReadFile(filepath.Join(cgroupPath, cgroupFile))
	if err != nil {
		return 0, err
	}

	return parseUint(strings.TrimSpace(string(contents)), 10, 64)
}
func parseUint(s string, base, bitSize int) (uint64, error) {
	value, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. Handle negative values greater than MinInt64 (and)
		// 2. Handle negative values lesser than MinInt64
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil && intErr.(*strconv.NumError).Err == strconv.ErrRange && intValue < 0 {
			return 0, nil
		}

		return value, err
	}

	return value, nil
}
