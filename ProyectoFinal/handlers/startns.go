package handlers

import (
	"net/http"
	"os/exec"
	"log"
)

func StartNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	log.Printf("[StartNS] Iniciando VM ns...")
	out, err := exec.Command("VBoxManage", "startvm", "ns", "--type", "headless").CombinedOutput()
	if err != nil {
		log.Printf("[StartNS] Error: %v - %s", err, out)
		http.Error(w, "Error iniciando VM ns: "+string(out), http.StatusInternalServerError)
		return
	}
	log.Printf("[StartNS] VM ns iniciada OK")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}