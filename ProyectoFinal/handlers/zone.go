package handlers

import (
	"fmt"
	"net/http"

	"httpaas/vbox"
)

const (
	NSHost   = "192.168.10.10"
	ZoneFile = "/etc/bind/db.cloud.local"
)

func Zone(w http.ResponseWriter, r *http.Request) {
	out, err := vbox.RunSSHOutput(NSHost, "cat "+ZoneFile)
	if err != nil {
		http.Error(w, "Error leyendo zona DNS: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, out)
}