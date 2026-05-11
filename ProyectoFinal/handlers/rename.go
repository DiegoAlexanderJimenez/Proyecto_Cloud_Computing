package handlers

import (
	"fmt"
	"httpaas/dns"
	"httpaas/vbox"
	"net/http"
	"time"
)

func Rename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	oldHostname := r.FormValue("old_hostname")
	newHostname := r.FormValue("new_hostname")

	if oldHostname == "" || newHostname == "" {
		http.Error(w, "Hostnames requeridos", http.StatusBadRequest)
		return
	}

	logAndBroadcast("[Rename] Renombrando %s -> %s", oldHostname, newHostname)

	storeMu.Lock()
	inst, ok := store[oldHostname]
	storeMu.Unlock()

	if !ok {
		http.Error(w, "Instancia no encontrada", http.StatusNotFound)
		return
	}

	sshPort := 2200
	{
		var a, b, c, d int
		fmt.Sscanf(inst.IP, "%d.%d.%d.%d", &a, &b, &c, &d)
		sshPort = 2200 + d
	}

	// 1. Cambiar hostname en la VM
	renameCmd := fmt.Sprintf(
		`sudo hostnamectl set-hostname %s.cloud.local && `+
			`sudo sed -i 's/^127\.0\.1\.1.*/127.0.1.1\t%s.cloud.local\t%s/' /etc/hosts`,
		newHostname, newHostname, newHostname,
	)
	logAndBroadcast("[Rename] Cambiando hostname en VM...")
	if err := vbox.RunSSHPort("localhost", sshPort, renameCmd); err != nil {
		logAndBroadcast("[Rename] Error cambiando hostname: %v", err)
		http.Error(w, "Error cambiando hostname: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Actualizar DNS
	logAndBroadcast("[Rename] Actualizando DNS...")
	if err := dns.RemoveRecord(oldHostname); err != nil {
		logAndBroadcast("[Rename] Error eliminando DNS viejo: %v", err)
		http.Error(w, "Error eliminando DNS: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := dns.AddRecord(newHostname, inst.IP); err != nil {
		logAndBroadcast("[Rename] Error agregando DNS nuevo: %v", err)
		http.Error(w, "Error agregando DNS: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Actualizar store
	storeMu.Lock()
	delete(store, oldHostname)
	store[newHostname] = Instance{
		Hostname:  newHostname,
		VMName:    inst.VMName,
		IP:        inst.IP,
		CreatedAt: time.Now(),
	}
	storeMu.Unlock()

	logAndBroadcast("[Rename] Completado: %s -> %s", oldHostname, newHostname)
	w.WriteHeader(http.StatusOK)
}