package handlers

import (
	"fmt"
	"httpaas/dns"
	"httpaas/vbox"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func sshPortForIP(ip string) int {
	var a, b, c, d int
	fmt.Sscanf(ip, "%d.%d.%d.%d", &a, &b, &c, &d)
	return 2200 + d
}

func logAndBroadcast(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	BroadcastLog(msg)
}

func Provision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(50 << 20)
	hostname := r.FormValue("hostname")
	if hostname == "" {
		http.Error(w, "El nombre de host es obligatorio", http.StatusBadRequest)
		return
	}
	logAndBroadcast("[Provision] Iniciando para hostname: %s", hostname)

	storeMu.Lock()
	ips := make([]string, 0, len(store))
	for _, inst := range store {
		ips = append(ips, inst.IP)
	}
	storeMu.Unlock()

	ip := dns.NextIP(ips)
	if ip == "" {
		http.Error(w, "No hay IPs disponibles", http.StatusInternalServerError)
		return
	}
	logAndBroadcast("[Provision] IP asignada: %s", ip)

	sshPort := sshPortForIP(ip)
	logAndBroadcast("[Provision] Puerto SSH NAT: %d", sshPort)

	file, _, err := r.FormFile("zipfile")
	if err != nil {
		logAndBroadcast("[Provision] Error leyendo zip: %v", err)
		http.Error(w, "Archivo zip requerido", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpZip := filepath.Join(os.TempDir(), hostname+".zip")
	dst, err := os.Create(tmpZip)
	if err != nil {
		logAndBroadcast("[Provision] Error creando archivo temporal: %v", err)
		http.Error(w, "Error guardando zip", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpZip)

	if _, err := dst.ReadFrom(file); err != nil {
		dst.Close()
		logAndBroadcast("[Provision] Error escribiendo zip: %v", err)
		http.Error(w, "Error guardando zip", http.StatusInternalServerError)
		return
	}
	dst.Close()
	logAndBroadcast("[Provision] ZIP guardado en: %s", tmpZip)

	logAndBroadcast("[Provision] Creando VM...")
	if err := vbox.CreateVM(hostname, ip, sshPort); err != nil {
		logAndBroadcast("[Provision] Error creando VM: %v", err)
		http.Error(w, "Error creando VM: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logAndBroadcast("[Provision] VM creada OK")

	logAndBroadcast("[Provision] Esperando SSH via NAT en localhost:%d...", sshPort)
	if err := vbox.WaitForSSHPort("localhost", sshPort, 24); err != nil {
		logAndBroadcast("[Provision] SSH NAT no disponible: %v", err)
		http.Error(w, "VM no respondió a SSH: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logAndBroadcast("[Provision] SSH NAT disponible")

	configCmd := fmt.Sprintf(
		`sudo hostnamectl set-hostname %s.cloud.local && `+
			`sudo sed -i 's/^127\.0\.1\.1.*/127.0.1.1\t%s.cloud.local\t%s/' /etc/hosts && `+
			`echo -e '# Loopback\nauto lo\niface lo inet loopback\n\n# Red interna cloudnet\nauto enp0s8\niface enp0s8 inet static\n  address %s\n  netmask 255.255.255.0' | sudo tee /etc/network/interfaces && `+
			`sudo systemctl restart networking`,
		hostname, hostname, hostname, ip,
	)
	logAndBroadcast("[Provision] Configurando hostname e IP via NAT...")
	if err := vbox.RunSSHPort("localhost", sshPort, configCmd); err != nil {
		logAndBroadcast("[Provision] Error configurando VM: %v", err)
		http.Error(w, "Error configurando VM: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logAndBroadcast("[Provision] Hostname e IP configurados")

	logAndBroadcast("[Provision] Esperando que la red se estabilice...")
	time.Sleep(5 * time.Second)
	logAndBroadcast("[Provision] Red estabilizada")

	logAndBroadcast("[Provision] Registrando en DNS...")
	if err := dns.AddRecord(hostname, ip); err != nil {
		logAndBroadcast("[Provision] Error DNS: %v", err)
		http.Error(w, "Error actualizando DNS: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logAndBroadcast("[Provision] DNS OK")

	logAndBroadcast("[Provision] Copiando ZIP a la VM...")
	if err := vbox.CopyFilePort("localhost", sshPort, tmpZip, "/tmp/site.zip"); err != nil {
		logAndBroadcast("[Provision] Error copiando ZIP: %v", err)
		http.Error(w, "Error copiando contenido: "+err.Error(), http.StatusInternalServerError)
		return
	}

	deployCmd := `sudo apt-get install -y unzip && ` +
		`sudo rm -rf /var/www/html/* && ` +
		`sudo unzip -o /tmp/site.zip -d /tmp/site_extract/ && ` +
		`sudo cp -r /tmp/site_extract/*/* /var/www/html/ && ` +
		`sudo rm -rf /tmp/site_extract && ` +
		`sudo chown -R www-data:www-data /var/www/html/`
	logAndBroadcast("[Provision] Desplegando contenido web...")
	if err := vbox.RunSSHPort("localhost", sshPort, deployCmd); err != nil {
		logAndBroadcast("[Provision] Error desplegando: %v", err)
		http.Error(w, "Error desplegando contenido: "+err.Error(), http.StatusInternalServerError)
		return
	}

	storeMu.Lock()
	store[hostname] = Instance{
		Hostname:  hostname,
		IP:        ip,
		CreatedAt: time.Now(),
	}
	storeMu.Unlock()

	logAndBroadcast("[Provision] Completado: %s -> %s", hostname, ip)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Delete(w http.ResponseWriter, r *http.Request) {
	hostname := r.FormValue("hostname")
	logAndBroadcast("[Delete] Eliminando: %s", hostname)

	storeMu.Lock()
	inst, ok := store[hostname]
	if ok {
		delete(store, hostname)
	}
	storeMu.Unlock()

	if ok {
		dns.RemoveRecord(hostname)
		vbox.DeleteVM(inst.Hostname)
		logAndBroadcast("[Delete] Eliminado OK: %s", hostname)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}