package dns

import (
	"fmt"
	"httpaas/vbox"
	"strconv"
	"strings"
)

const (
	NSHost   = "192.168.10.10"
	ZoneFile = "/etc/bind/db.cloud.local"
)

func AddRecord(hostname, ip string) error {
	addRecord := fmt.Sprintf(
		"echo '%s IN A %s' | sudo tee -a %s",
		hostname, ip, ZoneFile,
	)

	addCNAME := fmt.Sprintf(
		"echo 'www.%s IN CNAME %s' | sudo tee -a %s",
		hostname, hostname, ZoneFile,
	)

	bumpSerial := "sudo /usr/local/bin/bump-serial.sh"

	reload := "sudo systemctl reload bind9"

	for _, cmd := range []string{addRecord, addCNAME, bumpSerial, reload} {
		if err := vbox.RunSSH(NSHost, cmd); err != nil {
			return fmt.Errorf("DNS AddRecord: %w", err)
		}
	}
	return nil
}

func RemoveRecord(hostname string) error {
	remove := fmt.Sprintf(
		"sudo sed -i '/^%s IN A/d' %s", hostname, ZoneFile,
	)

	removeCNAME := fmt.Sprintf(
		"sudo sed -i '/^www.%s IN CNAME/d' %s", hostname, ZoneFile,
	)

	bumpSerial := "sudo /usr/local/bin/bump-serial.sh"

	reload := "sudo systemctl reload bind9"

	for _, cmd := range []string{remove, removeCNAME, bumpSerial, reload} {
		if err := vbox.RunSSH(NSHost, cmd); err != nil {
			return fmt.Errorf("DNS RemoveRecord: %w", err)
		}
	}
	return nil
}

func NextIP(instances []string) string {
	used := map[int]bool{}
	for _, inst := range instances {
		parts := strings.Split(inst, ".")
		if len(parts) == 4 {
			n, err := strconv.Atoi(parts[3])
			if err == nil {
				used[n] = true
			}
		}
	}
	for i := 30; i < 255; i++ {
		if !used[i] {
			return fmt.Sprintf("192.168.10.%d", i)
		}
	}
	return ""
}