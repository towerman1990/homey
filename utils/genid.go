package utils

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sony/sonyflake"
)

// use the machine's low 16-bit ip as it's ID
func getMachineID() (machineID uint16, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	var lowIP int
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipSlice := strings.Split(ipnet.IP.To4().String(), ".")

				ip3, err := strconv.Atoi(ipSlice[2])
				if err != nil {
					return machineID, err
				}

				ip4, err := strconv.Atoi(ipSlice[3])
				if err != nil {
					return machineID, err
				}

				lowIP = ip3<<8 + ip4
				break
			}
		}
	}
	machineID = uint16(lowIP)

	return
}

var generatorOnce sync.Once
var generator *sonyflake.Sonyflake

// NewIDGenerator creates a generator for a deployment-assigned machine ID.
func NewIDGenerator(machineID uint16) *sonyflake.Sonyflake {
	return sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		MachineID: func() (uint16, error) { return machineID, nil },
	})
}

func GenID() (uint64, error) {
	generatorOnce.Do(func() {
		generator = sonyflake.NewSonyflake(sonyflake.Settings{
			StartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			MachineID: getMachineID,
		})
	})
	if generator == nil {
		return 0, errors.New("homey: initialize Sonyflake generator")
	}
	return generator.NextID()
}
