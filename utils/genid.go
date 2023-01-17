package utils

import (
	"net"
	"strconv"
	"strings"
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

func saddMachineIDToRedisSet() (result int, err error) {
	return
}

func checkMachineID(machineID uint16) bool {
	saddResult, err := saddMachineIDToRedisSet()
	if err != nil || saddResult == 0 {
		return true
	}

	return false
}
func GenID() (uid uint64, err error) {
	t, _ := time.Parse("2006-01-02", "2023-01-01")
	settings := sonyflake.Settings{
		StartTime:      t,
		MachineID:      getMachineID,
		CheckMachineID: checkMachineID,
	}

	sf := sonyflake.NewSonyflake(settings)
	return sf.NextID()
}
