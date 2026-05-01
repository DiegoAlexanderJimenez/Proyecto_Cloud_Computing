package vbox

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

const (
    BaseDisk = `C:\Users\DIEGO\VirtualBox VMs\ApacheTemplate.vdi`
    SSHKey   = `C:\Users\DIEGO\.ssh\cloud_key`
    SSHUser  = "diego"
    SSHPass  = "123"
	NSPort   = 2210 
)

// Crea y arranca una nueva VM con NAT (para SSH inicial) + cloudnet (red interna)
// El puerto SSH de la VM se mapea al puerto 2200+offset del host para evitar colisiones
func CreateVM(name, ip string, sshPort int) error {
	disk := filepath.FromSlash(BaseDisk)

	cmds := [][]string{
		// 1. Registrar la VM
		{"VBoxManage", "createvm", "--name", name, "--ostype", "Debian_64", "--register"},

		// 2. Agregar controlador de almacenamiento
		{"VBoxManage", "storagectl", name, "--name", "SATA", "--add", "sata", "--controller", "IntelAhci"},

		// 3. Conectar el disco multiattach
		{"VBoxManage", "storageattach", name,
			"--storagectl", "SATA", "--port", "0", "--device", "0",
			"--type", "hdd", "--medium", disk, "--mtype", "multiattach"},

		// 4. Configurar hardware: 512MB RAM, 1 CPU
		//    NIC1 = NAT (para SSH inicial)
		//    NIC2 = red interna cloudnet (para comunicación final)
		{"VBoxManage", "modifyvm", name,
		"--memory", "512", "--cpus", "1",
		"--nic1", "nat",
		"--nic2", "hostonly", "--hostonlyadapter2", "VirtualBox Host-Only Ethernet Adapter #2"},

		// 5. Port forwarding: host:sshPort -> VM:22 y host:httpPort -> VM:80
		{"VBoxManage", "modifyvm", name,
			"--natpf1", fmt.Sprintf("ssh,tcp,,%d,,22", sshPort)},

		// 6. Arrancar headless
		{"VBoxManage", "startvm", name, "--type", "headless"},
	}

	for _, args := range cmds {
		log.Printf("[CreateVM] Ejecutando: %v", args)
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("error en %v: %s", args, out)
		}
		log.Printf("[CreateVM] OK: %s", out)
	}
	return nil
}

// RemoveNATAdapter quita el adaptador NAT una vez configurada la VM
func RemoveNATAdapter(name string) error {
	args := []string{"VBoxManage", "modifyvm", name, "--nic1", "none"}
	log.Printf("[RemoveNAT] Quitando adaptador NAT de %s", name)
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error quitando NAT: %s", out)
	}
	return nil
}

// Apaga y elimina una VM
func DeleteVM(name string) error {
	cmds := [][]string{
		{"VBoxManage", "controlvm", name, "poweroff"},
		{"VBoxManage", "unregistervm", name, "--delete"},
	}
	for _, args := range cmds {
		log.Printf("[DeleteVM] Ejecutando: %v", args)
		out, _ := exec.Command(args[0], args[1:]...).CombinedOutput()
		log.Printf("[DeleteVM] Resultado: %s", out)
	}
	return nil
}
