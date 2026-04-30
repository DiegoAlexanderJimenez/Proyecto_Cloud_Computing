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

// sshPortForVM calcula un puerto host único por VM basado en el último octeto de la IP
// 192.168.10.30 → puerto 2230, .31 → 2231, etc.
func sshPortForIP(ip string) int {
	var a, b, c, d int
	fmt.Sscanf(ip, "%d.%d.%d.%d", &a, &b, &c, &d)
	return 2200 + d
}

// Provision maneja POST /provision
func Provision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 1. Leer campos del formulario
	r.ParseMultipartForm(50 << 20)
	hostname := r.FormValue("hostname")
	if hostname == "" {
		http.Error(w, "El nombre de host es obligatorio", http.StatusBadRequest)
		return
	}
	log.Printf("[Provision] Iniciando para hostname: %s", hostname)

	// 2. Calcular IP disponible
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
	log.Printf("[Provision] IP asignada: %s", ip)

	// Puerto SSH del host para esta VM (via NAT port forwarding)
	sshPort := sshPortForIP(ip)
	log.Printf("[Provision] Puerto SSH NAT: %d", sshPort)

	// 3. Guardar el zip subido en disco temporal
	file, _, err := r.FormFile("zipfile")
	if err != nil {
		log.Printf("[Provision] Error leyendo zip: %v", err)
		http.Error(w, "Archivo zip requerido", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpZip := filepath.Join(os.TempDir(), hostname+".zip")
	dst, err := os.Create(tmpZip)
	if err != nil {
		log.Printf("[Provision] Error creando archivo temporal: %v", err)
		http.Error(w, "Error guardando zip", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpZip)

	if _, err := dst.ReadFrom(file); err != nil {
		dst.Close()
		log.Printf("[Provision] Error escribiendo zip: %v", err)
		http.Error(w, "Error guardando zip", http.StatusInternalServerError)
		return
	}
	dst.Close()
	log.Printf("[Provision] ZIP guardado en: %s", tmpZip)

	// 4. Crear la VM (NIC1=NAT con port forwarding, NIC2=cloudnet)
	log.Printf("[Provision] Creando VM...")
	if err := vbox.CreateVM(hostname, ip, sshPort); err != nil {
		log.Printf("[Provision] Error creando VM: %v", err)
		http.Error(w, "Error creando VM: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[Provision] VM creada OK")

	// 5. Esperar SSH por NAT (localhost:sshPort) — la VM arranca con IP de plantilla
	log.Printf("[Provision] Esperando SSH via NAT en localhost:%d...", sshPort)
	if err := vbox.WaitForSSHPort("localhost", sshPort, 24); err != nil {
		log.Printf("[Provision] SSH NAT no disponible: %v", err)
		http.Error(w, "VM no respondió a SSH: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[Provision] SSH NAT disponible")

	// 6. Configurar hostname, /etc/hosts e IP en enp0s8 (NIC2 = cloudnet)
	//    enp0s3 = NAT (NIC1), enp0s8 = cloudnet (NIC2)
	configCmd := fmt.Sprintf(
		`sudo hostnamectl set-hostname %s.cloud.local && `+
			`sudo sed -i 's/^127\.0\.1\.1.*/127.0.1.1\t%s.cloud.local\t%s/' /etc/hosts && `+
			`echo -e '# Loopback\nauto lo\niface lo inet loopback\n\n# Red interna cloudnet\nauto enp0s8\niface enp0s8 inet static\n  address %s\n  netmask 255.255.255.0' | sudo tee /etc/network/interfaces && `+
			`sudo systemctl restart networking`,
		hostname, hostname, hostname, ip,
	)
	log.Printf("[Provision] Configurando hostname e IP via NAT...")
	if err := vbox.RunSSHPort("localhost", sshPort, configCmd); err != nil {
		log.Printf("[Provision] Error configurando VM: %v", err)
		http.Error(w, "Error configurando VM: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[Provision] Hostname e IP configurados")

	// 7. Esperar que networking reinicie (la IP ya está configurada)
	log.Printf("[Provision] Esperando que la red se estabilice...")
	time.Sleep(5 * time.Second)
	log.Printf("[Provision] Red estabilizada")

	// 8. Registrar en DNS via NAT
	log.Printf("[Provision] Registrando en DNS...")
	if err := dns.AddRecord(hostname, ip); err != nil {
		log.Printf("[Provision] Error DNS: %v", err)
		http.Error(w, "Error actualizando DNS: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[Provision] DNS OK")

	// 9. Copiar zip y desplegar contenido web via IP real
	log.Printf("[Provision] Copiando ZIP a la VM...")
	if err := vbox.CopyFilePort("localhost", sshPort, tmpZip, "/tmp/site.zip"); err != nil {
		log.Printf("[Provision] Error copiando ZIP: %v", err)
		http.Error(w, "Error copiando contenido: "+err.Error(), http.StatusInternalServerError)
		return
	}

	deployCmd := `sudo apt-get install -y unzip && ` +
		`sudo rm -rf /var/www/html/* && ` +
		`sudo unzip -o /tmp/site.zip -d /tmp/site_extract/ && ` +
		`sudo cp -r /tmp/site_extract/*/* /var/www/html/ && ` +
		`sudo rm -rf /tmp/site_extract && ` +
		`sudo chown -R www-data:www-data /var/www/html/`
	log.Printf("[Provision] Desplegando contenido web...")
	if err := vbox.RunSSHPort("localhost", sshPort, deployCmd); err != nil {
		log.Printf("[Provision] Error desplegando: %v", err)
		http.Error(w, "Error desplegando contenido: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 10. Guardar instancia en el store
	storeMu.Lock()
	store[hostname] = Instance{
		Hostname:  hostname,
		IP:        ip,
		HTTPPort:  sshPort + 1000,
		CreatedAt: time.Now(),
	}
	storeMu.Unlock()

	log.Printf("[Provision] Completado: %s -> %s", hostname, ip)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Delete maneja POST /delete
func Delete(w http.ResponseWriter, r *http.Request) {
	hostname := r.FormValue("hostname")
	log.Printf("[Delete] Eliminando: %s", hostname)

	storeMu.Lock()
	inst, ok := store[hostname]
	if ok {
		delete(store, hostname)
	}
	storeMu.Unlock()

	if ok {
		dns.RemoveRecord(hostname)
		vbox.DeleteVM(inst.Hostname)
		log.Printf("[Delete] Eliminado OK: %s", hostname)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
