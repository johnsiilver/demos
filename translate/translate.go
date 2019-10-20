package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	ipv4RE = regexp.MustCompile(`((?:[0-9]{1,3}\.){3}(?:[0-9]{1,3}))`)
)

func replaceIP(line string) string {
	matches := ipv4RE.FindAllStringSubmatch(line, -1)
	if len(matches) > 0 {
		for _, submatch := range matches {
			if net.ParseIP(submatch[1]) != nil {
				names, err := net.LookupAddr(submatch[1])
				if err != nil || len(names) == 0 {
					continue
				}
				line = strings.ReplaceAll(line, submatch[1], names[0])
			}
		}
	}
	return line
}

func main() {
	defer fmt.Println("exited")
	if len(os.Args) != 2 {
		fmt.Println("must pass user@server")
		os.Exit(1)
	}

	arg := strings.Split(os.Args[1], "@")
	if len(arg) != 2 {
		fmt.Println("must pass user@server")
		os.Exit(1)
	}

	fmt.Printf("Password: ")
	state, err := terminal.MakeRaw(0)
	if err != nil {
		log.Fatal(err)
	}

	pass, err := terminal.ReadPassword(0)
	if err != nil {
		log.Fatal(err)
	}
	if err := terminal.Restore(0, state); err != nil {
		log.Fatal(err)
	}
	fmt.Println("")

	config := &ssh.ClientConfig{
		User: arg[0],
		Auth: []ssh.AuthMethod{
			ssh.Password(string(pass)),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", arg[1], config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}

	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}

	buff := &bytes.Buffer{}
	r := bufio.NewReader(buff)

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Fatalf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, os.Stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Fatalf("Unable to setup stdout for session: %v", err)
	}
	go io.Copy(buff, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		log.Fatalf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(os.Stderr, stderr)

	session.Shell()

	done := make(chan struct{})
	go func() {
		session.Wait()
		log.Println("session ended?")
		close(done)
	}()

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				select {
				case <-done:
					os.Exit(0)
				default:
					time.Sleep(200 * time.Millisecond)
					continue
				}
			}
			fmt.Println("received error: %s", err)
			os.Exit(1)
		}
		os.Stdout.WriteString(replaceIP(line))
	}
}
