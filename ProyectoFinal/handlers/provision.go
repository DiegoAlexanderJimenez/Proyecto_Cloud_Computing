package handlers

import (
    "fmt"
    "httpaas/dns"
    "httpaas/vbox"
    "net/http"
    "os"
    "time"
    "path/filepath"
)

// Provision maneja POST /provision
func Provision(w http.ResponseWriter, r *http.Request) {
    fmt.Println("====================================")
    fmt.Println("ENTRÓ A Provision")

    if r.Method != http.MethodPost {
        fmt.Println("Método incorrecto:", r.Method)
        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
    }

    fmt.Println("Parseando formulario...")
    if err := r.ParseMultipartForm(50 << 20); err != nil {
        fmt.Println("Error ParseMultipartForm:", err)
        http.Error(w, err.Error(), 500)
        return
    }

    hostname := r.FormValue("hostname")
    fmt.Println("Hostname:", hostname)

    if hostname == "" {
        fmt.Println("Hostname vacío")
        http.Error(w, "El nombre de host es obligatorio", http.StatusBadRequest)
        return
    }

    // IPs usadas
    ips := make([]string, 0, len(store))
    for _, inst := range store {
        ips = append(ips, inst.IP)
    }

    ip := dns.NextIP(ips)
    fmt.Println("IP asignada:", ip)

    if ip == "" {
        fmt.Println("No hay IPs disponibles")
        http.Error(w, "No hay IPs disponibles", 500)
        return
    }

    // Archivo
    fmt.Println("Leyendo archivo zip...")
    file, _, err := r.FormFile("zipfile")
    if err != nil {
        fmt.Println("Error FormFile:", err)
        http.Error(w, "Archivo zip requerido", 400)
        return
    }
    defer file.Close()

    tmpZip := filepath.Join(os.TempDir(), hostname+".zip")
    fmt.Println("Guardando en:", tmpZip)

    dst, err := os.Create(tmpZip)
    if err != nil {
        fmt.Println("Error creando archivo:", err)
        http.Error(w, err.Error(), 500)
        return
    }

    if _, err := dst.ReadFrom(file); err != nil {
        fmt.Println("Error guardando zip:", err)
        http.Error(w, "Error guardando zip", 500)
        return
    }
    dst.Close()

    fmt.Println("ZIP guardado correctamente")

    // Crear VM
    fmt.Println("Creando VM...")
    if err := vbox.CreateVM(hostname, ip); err != nil {
        fmt.Println("ERROR CreateVM:", err)
        http.Error(w, err.Error(), 500)
        return
    }

    fmt.Println("Esperando SSH...")
    if err := vbox.WaitForSSH(ip, 24); err != nil {
        fmt.Println("ERROR SSH:", err)
        http.Error(w, err.Error(), 500)
        return
    }

    fmt.Println("Configurando VM...")
    configCmd := fmt.Sprintf(
        `sudo hostnamectl set-hostname %s.cloud.local && \
         sudo sed -i 's/address .*/address %s/' /etc/network/interfaces && \
         sudo systemctl restart networking`,
        hostname, ip,
    )

    if err := vbox.RunSSH(ip, configCmd); err != nil {
        fmt.Println("ERROR configuración:", err)
    }

    fmt.Println("Actualizando DNS...")
    if err := dns.AddRecord(hostname, ip); err != nil {
        fmt.Println("ERROR DNS:", err)
        http.Error(w, err.Error(), 500)
        return
    }

    fmt.Println("Desplegando sitio...")
    if err := vbox.CopyFile(ip, tmpZip, "/tmp/site.zip"); err != nil {
        fmt.Println("ERROR CopyFile:", err)
    }

    deployCmd := `sudo rm -rf /var/www/html/* && \
                  sudo unzip -o /tmp/site.zip -d /var/www/html/ && \
                  sudo chown -R www-data:www-data /var/www/html/`

    if err := vbox.RunSSH(ip, deployCmd); err != nil {
        fmt.Println("ERROR Deploy:", err)
    }

    storeMu.Lock()
    store[hostname] = Instance{
        Hostname:  hostname,
        IP:        ip,
        CreatedAt: time.Now(),
    }
    storeMu.Unlock()

    fmt.Println("Provision completado correctamente")

    http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Delete maneja POST /delete
func Delete(w http.ResponseWriter, r *http.Request) {
    hostname := r.FormValue("hostname")

    storeMu.Lock()
    inst, ok := store[hostname]
    if ok {
        delete(store, hostname)
    }
    storeMu.Unlock()

    if ok {
        dns.RemoveRecord(hostname)
        vbox.DeleteVM(inst.Hostname)
    }

    http.Redirect(w, r, "/", http.StatusSeeOther)
}