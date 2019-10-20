package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var prompt = "prompt>"

// next returns the next rune in the input.
func translator(r *bufio.Reader, f *os.File) error {
	lr := &lineRunner{}

	for {
		if r.Buffered() == 0 {
			f.WriteString(prompt)
		}

		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("received error: %s", err)
		}
		lr.reset(line)
		run(lr.start)

		f.WriteString(lr.outBuffer())
	}

}

type StateFn func() (StateFn, error)

const (
	// ItemEOL indicates that the end of input is reached. No further tokens will be sent.
	ItemEOL = '\n'
)

func run(sf StateFn) error {
	var err error
	for {
		sf, err = sf()
		if err != nil {
			return err
		}
		if sf == nil {
			return nil
		}
	}
}

type lineRunner struct {
	input    string
	pos      int
	strBuff  strings.Builder
	possible []rune
}

func (l *lineRunner) reset(line string) {
	l.input = line
	l.pos = 0
	l.strBuff.Reset()
	l.possible = l.possible[0:0]
}

func (l *lineRunner) outBuffer() string {
	defer l.strBuff.Reset()
	str := l.strBuff.String()
	if len(str) == 0 {
		return "\n"
	}
	if str[len(str)-1] != '\n' {
		return str + "\n"
	}
	return str
}

// Backup steps back one rune. Can be called only once per call of next.
func (l *lineRunner) backup() {
	l.pos -= 1
}

func (l *lineRunner) peek() rune {
	if l.pos >= len(l.input) {
		return rune(ItemEOL)
	}
	r := l.next()
	l.backup()
	return r
}

func (l *lineRunner) next() rune {
	if l.pos >= len(l.input) {
		return rune(ItemEOL)
	}
	l.pos++
	return rune(l.input[l.pos-1])
}

func (l *lineRunner) start() (StateFn, error) {
	l.possible = l.possible[0:0]
	for char := l.next(); char != ItemEOL; char = l.next() {
		if l.isNum(char) {
			pChar := l.peek()
			switch {
			case l.isNum(pChar) || l.isDot(pChar):
				l.backup()
				return l.firstOctet, nil
			}
		}
		l.strBuff.WriteRune(char)
	}
	return nil, nil
}

func (l *lineRunner) firstOctet() (StateFn, error) {
	for i := 0; i < 4; i++ {
		char := l.next()
		l.add(char)
		switch {
		case i == 3:
			if l.isDot(char) {
				return l.secondOctet, nil
			}
			return l.publish()
		case l.isNum(char):
		case l.isDot(char):
			return l.secondOctet, nil
		case char == ItemEOL:
			return l.eof()
		default:
			return l.publish()
		}
	}
	panic("can't get here")
}

func (l *lineRunner) secondOctet() (StateFn, error) {
	for i := 0; i < 4; i++ {
		char := l.next()
		l.add(char)
		switch {
		case i == 3:
			if l.isDot(char) {
				return l.thirdOctet, nil
			}
			return l.publish()
		case l.isNum(char):
		case l.isDot(char):
			return l.thirdOctet, nil
		case char == ItemEOL:
			return l.eof()
		default:
			return l.publish()
		}
	}
	panic("can't get here")
}

func (l *lineRunner) thirdOctet() (StateFn, error) {
	for i := 0; i < 4; i++ {
		char := l.next()
		l.add(char)
		switch {
		case i == 3:
			if l.isDot(char) {
				return l.fourthOctet, nil
			}
			return l.publish()
		case l.isNum(char):
		case l.isDot(char):
			return l.fourthOctet, nil
		case char == ItemEOL:
			return l.eof()
		default:
			return l.publish()
		}
	}
	panic("can't reach")
}

func (l *lineRunner) fourthOctet() (StateFn, error) {
	for i := 0; i < 3; i++ {
		char := l.next()
		l.add(char)
		switch {
		case char == ' ' || char == ItemEOL:
			if i == 0 {
				return l.publish()
			}
			return l.validateIP, nil
		case i == 2:
			pChar := l.peek()
			if pChar == ' ' || pChar == ItemEOL {
				return l.validateIP, nil
			}
			return l.publish()
		case l.isNum(char):
		case char == ItemEOL:
			return l.eof()
		default:
			return l.publish()
		}
	}
	panic("can't reach")
}

func (l *lineRunner) validateIP() (StateFn, error) {
	ipStr := string(l.possible)
	addChars := []rune{}

	switch last := l.possible[len(l.possible)-1]; last {
	case ' ', '\n':
		addChars = append(addChars, last)
		ipStr = strings.TrimSpace(ipStr)
	}

	if net.ParseIP(ipStr) != nil {
		names, err := net.LookupAddr(ipStr)
		if err != nil || len(names) == 0 {
			return l.publish()
		}
		l.strBuff.WriteString(names[0])
		for _, r := range addChars {
			l.strBuff.WriteRune(r)
		}
		return l.start, nil
	}
	return l.publish()
}

func (l *lineRunner) eof() (StateFn, error) {
	l.strBuff.WriteString(string(l.possible))
	l.possible = l.possible[0:0]
	return nil, nil
}

// add adds a character to our .possible field, as it might be part of an ipv4 address.
func (l *lineRunner) add(char rune) {
	l.possible = append(l.possible, char)
}

// publish takes the .posssible and writes it to our string.Builder, returns us to starting state.
func (l *lineRunner) publish() (StateFn, error) {
	l.strBuff.WriteString(string(l.possible))
	l.possible = l.possible[0:0]
	return l.start, nil
}

// isNum determines if the charater is a number.
func (l *lineRunner) isNum(char rune) bool {
	switch string(char) {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return true
	}
	return false
}

// isDot determines if the character is a ".".
func (l *lineRunner) isDot(char rune) bool {
	if string(char) == "." {
		return true
	}
	return false
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

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Fatalf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, os.Stdin)

	stderr, err := session.StderrPipe()
	if err != nil {
		log.Fatalf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(os.Stderr, stderr)

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Fatalf("Unable to setup stdout for session: %v", err)
	}
	r := bufio.NewReader(stdout)

	session.Shell()

	translator(r, os.Stdout)
}
