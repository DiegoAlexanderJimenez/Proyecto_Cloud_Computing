package vbox

import (
    "fmt"
    "io"
    "os"
    "time"

    "golang.org/x/crypto/ssh"
)

func sshClient(host string) (*ssh.Client, error) {
    key, err := os.ReadFile(SSHKey)
    if err != nil {
        return nil, err
    }
    signer, err := ssh.ParsePrivateKey(key)
    if err != nil {
        return nil, err
    }
    config := &ssh.ClientConfig{
        User:            SSHUser,
        Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         10 * time.Second,
    }
    return ssh.Dial("tcp", host+":22", config)
}

// Ejecuta un comando en un host remoto
func RunSSH(host, cmd string) error {
    client, err := sshClient(host)
    if err != nil {
        return fmt.Errorf("conexión SSH a %s fallida: %w", host, err)
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        return err
    }
    defer session.Close()

    out, err := session.CombinedOutput(cmd)
    if err != nil {
        return fmt.Errorf("comando fallido: %s — %w", out, err)
    }
    return nil
}

// Copia un archivo local a un host remoto vía SCP (protocolo SSH)
func CopyFile(host, localPath, remotePath string) error {
    client, err := sshClient(host)
    if err != nil {
        return err
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        return err
    }
    defer session.Close()

    f, err := os.Open(localPath)
    if err != nil {
        return err
    }
    defer f.Close()

    stat, _ := f.Stat()
    stdin, _ := session.StdinPipe()

    go func() {
        defer stdin.Close()
        fmt.Fprintf(stdin, "C0644 %d %s\n", stat.Size(), remotePath)
        io.Copy(stdin, f)
        fmt.Fprint(stdin, "\x00")
    }()

    return session.Run(fmt.Sprintf("scp -t %s", remotePath))
}

// Espera a que la VM tenga SSH disponible (con reintentos)
func WaitForSSH(host string, maxAttempts int) error {
    for i := 0; i < maxAttempts; i++ {
        client, err := sshClient(host)
        if err == nil {
            client.Close()
            return nil
        }
        time.Sleep(5 * time.Second)
    }
    return fmt.Errorf("SSH no disponible en %s tras %d intentos", host, maxAttempts)
}