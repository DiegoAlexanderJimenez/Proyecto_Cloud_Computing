package vbox

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sshClient abre una conexión SSH a host:port
func sshClient(host string, port int) (*ssh.Client, error) {
	key, err := os.ReadFile(SSHKey)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la llave SSH (%s): %w", SSHKey, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("llave SSH inválida: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	return ssh.Dial("tcp", addr, config)
}

// RunSSH ejecuta un comando en host:22
func RunSSH(host, cmd string) error {
	return RunSSHPort(host, 22, cmd)
}

// RunSSHPort ejecuta un comando en host:port (útil para NAT con port forwarding)
func RunSSHPort(host string, port int, cmd string) error {
	client, err := sshClient(host, port)
	if err != nil {
		return fmt.Errorf("conexión SSH a %s:%d fallida: %w", host, port, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	wrappedCmd := fmt.Sprintf("echo '%s' | sudo -S bash -c %q", SSHPass, cmd)
out, err := session.CombinedOutput(wrappedCmd)
if err != nil {
    return fmt.Errorf("comando fallido en %s:%d: %s — %w", host, port, out, err)
}
	log.Printf("[SSH %s:%d] %s", host, port, out)
	return nil
}

// CopyFile copia un archivo local a un host remoto vía SCP en puerto 22
func CopyFile(host, localPath, remotePath string) error {
	return CopyFilePort(host, 22, localPath, remotePath)
}

// CopyFilePort copia un archivo a host:port
func CopyFilePort(host string, port int, localPath, remotePath string) error {
	client, err := sshClient(host, port)
	if err != nil {
		return fmt.Errorf("SFTP: conexión a %s:%d fallida: %w", host, port, err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP: no se pudo crear cliente: %w", err)
	}
	defer sftpClient.Close()

	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("SFTP: no se pudo abrir archivo local: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("SFTP: no se pudo crear archivo remoto: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// WaitForSSH espera hasta que SSH responda en host:22
func WaitForSSH(host string, maxAttempts int) error {
	return WaitForSSHPort(host, 22, maxAttempts)
}

// WaitForSSHPort espera hasta que SSH responda en host:port
func WaitForSSHPort(host string, port int, maxAttempts int) error {
	for i := 0; i < maxAttempts; i++ {
		log.Printf("[WaitSSH] Intento %d/%d en %s:%d...", i+1, maxAttempts, host, port)
		client, err := sshClient(host, port)
		if err == nil {
			client.Close()
			log.Printf("[WaitSSH] SSH disponible en %s:%d", host, port)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("SSH no disponible en %s:%d tras %d intentos", host, port, maxAttempts)
}

func RunSSHOutput(host, cmd string) (string, error) {
	client, err := sshClient(host, 22)
	if err != nil {
		return "", fmt.Errorf("conexión SSH a %s fallida: %w", host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	wrappedCmd := fmt.Sprintf("echo '%s' | sudo -S bash -c %q", SSHPass, cmd)
	out, err := session.CombinedOutput(wrappedCmd)
	return string(out), err
}
