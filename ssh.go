package main

import (
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

var sshConn *ssh.Client

func sshConnect() error {
	key, err := ioutil.ReadFile(config.RepoPrivateKeyPath)
	if err != nil {
		return err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return err
	}
	sshConfig := &ssh.ClientConfig{
		User:            config.RepoUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	address := fmt.Sprintf("%s:%d", config.RepoAddress, config.RepoPort)
	sshConn, err = ssh.Dial("tcp", address, sshConfig)
	return err
}

func sshRunCommand(cmd string) ([]byte, error) {
	session, err := sshConn.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}
