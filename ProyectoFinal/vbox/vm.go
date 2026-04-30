package vbox

import (
    "fmt"
    "os/exec"
)

const (
    BaseDisk = "C:\\Users\\DIEGO\\VirtualBox VMs\\ApacheTemplate.vdi"
    SSHKey   = "/home/diego/.ssh/cloud_key"
    SSHUser  = "diego"
)

// Crea y arranca una nueva VM usando el disco multiconexión
func CreateVM(name, ip string) error {
    cmds := [][]string{
        {"VBoxManage", "createvm", "--name", name, "--ostype", "Debian_64", "--register"},
        {"VBoxManage", "storagectl", name, "--name", "SATA", "--add", "sata", "--controller", "IntelAhci"},
        {"VBoxManage", "storageattach", name,
            "--storagectl", "SATA", "--port", "0", "--device", "0",
            "--type", "hdd", "--medium", BaseDisk, "--mtype", "multiattach"},
        {"VBoxManage", "modifyvm", name,
            "--memory", "512", "--cpus", "1",
            "--nic1", "intnet", "--intnet1", "cloudnet"},
        {"VBoxManage", "startvm", name, "--type", "headless"},
    }

    for _, args := range cmds {
        fmt.Println("====================================")
        fmt.Println("Ejecutando comando:", args)

        cmd := exec.Command(args[0], args[1:]...)
        out, err := cmd.CombinedOutput()

        fmt.Println("Salida del comando:")
        fmt.Println(string(out))

        if err != nil {
            fmt.Println("ERROR detectado:", err)
            return fmt.Errorf("error en %v: %s", args, string(out))
        }
    }

    fmt.Println("VM creada correctamente:", name, "IP:", ip)
    return nil
}

// Apaga y elimina una VM
func DeleteVM(name string) error {
    cmds := [][]string{
        {"VBoxManage", "controlvm", name, "poweroff"},
        {"VBoxManage", "unregistervm", name, "--delete"},
    }
    for _, args := range cmds {
        exec.Command(args[0], args[1:]...).Run()
    }
    return nil
}