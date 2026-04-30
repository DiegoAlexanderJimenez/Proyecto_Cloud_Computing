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

    fmt.Println("====================================")
    fmt.Println("Iniciando AddRecord:", hostname, ip)

    // 1. Agregar registro A
    addRecord := fmt.Sprintf(
        "echo '%s IN A %s' | sudo tee -a %s",
        hostname, ip, ZoneFile,
    )

    // 2. Incrementar serial
    bumpSerial := fmt.Sprintf(
        `sudo sed -i 's/\([0-9]\+\)\(\s*;\s*serial\)/'"$(( $(grep -oP '[0-9]+(?=\s*;\s*serial)' %s) + 1 ))"'\2/' %s`,
        ZoneFile, ZoneFile,
    )

    // 3. Recargar bind
    reload := "sudo systemctl reload bind9"

    cmds := []string{addRecord, bumpSerial, reload}

    for _, cmd := range cmds {
        fmt.Println("------------------------------------")
        fmt.Println("Ejecutando en DNS:", cmd)

        err := vbox.RunSSH(NSHost, cmd)

        if err != nil {
            fmt.Println("ERROR en comando DNS:", err)
            return fmt.Errorf("DNS AddRecord fallo en comando '%s': %w", cmd, err)
        }

        fmt.Println("Comando ejecutado correctamente")
    }

    fmt.Println("Registro DNS agregado correctamente")
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