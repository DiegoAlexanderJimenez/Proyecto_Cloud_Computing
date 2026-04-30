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

// Agrega un registro A a la zona DNS e incrementa el serial
func AddRecord(hostname, ip string) error {
    // 1. Agregar el registro A
    addRecord := fmt.Sprintf(
        "echo '%s IN A %s' | sudo tee -a %s",
        hostname, ip, ZoneFile,
    )

    // 2. Incrementar el serial (reemplaza el número antes de "; serial")
    bumpSerial := fmt.Sprintf(
        `sudo sed -i 's/\([0-9]\+\)\(\s*;\s*serial\)/'"$(( $(grep -oP '[0-9]+(?=\s*;\s*serial)' %s) + 1 ))"'\2/' %s`,
        ZoneFile, ZoneFile,
    )

    // 3. Recargar Bind9
    reload := "sudo systemctl reload bind9"

    for _, cmd := range []string{addRecord, bumpSerial, reload} {
        if err := vbox.RunSSH(NSHost, cmd); err != nil {
            return fmt.Errorf("DNS AddRecord: %w", err)
        }
    }
    return nil
}

// Elimina un registro A de la zona DNS
func RemoveRecord(hostname string) error {
    remove := fmt.Sprintf(
        "sudo sed -i '/^%s IN A/d' %s", hostname, ZoneFile,
    )
    bumpSerial := fmt.Sprintf(
        `sudo sed -i 's/\([0-9]\+\)\(\s*;\s*serial\)/'"$(( $(grep -oP '[0-9]+(?=\s*;\s*serial)' %s) + 1 ))"'\2/' %s`,
        ZoneFile, ZoneFile,
    )
    reload := "sudo systemctl reload bind9"

    for _, cmd := range []string{remove, bumpSerial, reload} {
        if err := vbox.RunSSH(NSHost, cmd); err != nil {
            return fmt.Errorf("DNS RemoveRecord: %w", err)
        }
    }
    return nil
}

// Genera la próxima IP disponible en el rango .30 en adelante
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