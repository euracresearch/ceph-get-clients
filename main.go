// Copyright 2020 Eurac Research. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// WARNING: We will not guarantee that the output of this tool is correct. Use
// it on your own risk.

// Ceph-get-clients returns the current connected clients
//
// Usage:
//
//  ceph-get-clients -user cephssh [-port 22 -feature 0x200000] mon1 mon2 mon3
//
// Ceph-get-clients will connect to the given Ceph monitor servers using SSH and
// retrieve all currently connected clients using `ceph daemon mon.<hostname>
// sessions`. It will parse the output and merge it for all the given monitors,
// duplicated clients will be removed. For each client a reverse DNS lookup will
// be done. The output will be printed to Stdout using CSV format. It is
// possible to check if a client supports a give feature by passing the feature
// hex value as a parameter using the -feature flag.
//
// Prerequisite:
//
//  - SSH connection is using the local ssh agent
//  - SSH_AUTH_SOCK should be set and point to the running ssh agent socket
//  - SSH user should have sudo rights without password
//
// Example:
//
//  ceph-get-client -user cephadm -feature 0x200000 mon1 mon2 mon3
//  IP,feature,release,fqdn,0x200000
//  10.7.3.67,0x3ffddff8eea4fffb,luminous,clienta.fqdn.tld.,true
//  10.7.3.65,0x3ffddff8eea4fffb,luminous,webserver.fqdn.tld.,true
//  10.7.3.64,0x7010fb86aa42ada,jewel,,true
//  10.7.3.70,0x1ffddff8eea4fffb,luminous,usera.fqdn.tld.,true
//
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	var (
		user    = flag.String("user", "", "SSH username.")
		port    = flag.Int("port", 22, "SSH server port.")
		feature = flag.String("feature", "", "Check if the clients have the features. (e.g. '0x200000' will check if the client supports the upmap feature)")
	)
	flag.Parse()

	if *user == "" {
		log.Fatal("error missing -user")
	}

	if flag.NArg() < 1 {
		log.Fatal("missing host")
	}

	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		log.Fatalf("could not find ssh agent: %v", err)
	}

	agentClient := agent.NewClient(sshAgent)
	config := &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			// Use a callback rather than PublicKeys so we only consult the
			// agent once the remote server wants it.
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		// TODO: quick & dirty
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	var clients []*Client
	for _, h := range flag.Args() {
		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", h, *port), config)
		if err != nil {
			log.Printf("unable to connect: %v\n", err)
			continue
		}

		sess, err := client.NewSession()
		if err != nil {
			log.Printf("unable to create session: %v\n", err)
			continue
		}

		out, err := sess.Output(fmt.Sprintf("sudo ceph daemon mon.%s sessions", h))
		if err != nil {
			log.Printf("unable to execute 'ceph daemon mon.%s sessions: %v", h, err)
			continue
		}

		var c []*Client
		if err := json.Unmarshal([]byte(out), &c); err != nil {
			log.Printf("unable to unmarshal sessions: %v\n", err)
			continue
		}

		for _, add := range c {
			clients = unique(clients, add)
		}

		sess.Close()
		client.Close()
	}

	w := csv.NewWriter(os.Stdout)

	header := []string{"IP", "feature", "release", "fqdn"}
	if *feature != "" {
		header = append(header, *feature)
	}
	w.Write(header)

	for _, s := range clients {
		line := []string{s.IP, s.Feature, s.Release}

		names, _ := net.LookupAddr(s.IP)
		line = append(line, strings.Join(names, " "))

		if *feature != "" {
			line = append(line, fmt.Sprint(checkForFeatures(s, "200000")))
		}

		w.Write(line)
	}
	w.Flush()

	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
}

func unique(clients []*Client, add *Client) []*Client {
	for _, c := range clients {
		if c.Equal(add) {
			return clients
		}
	}

	return append(clients, add)
}

func checkForFeatures(c *Client, feature string) bool {
	i, err := strconv.ParseInt(trimHexPrefix(c.Feature), 16, 0)
	if err != nil {
		log.Fatal(err)
	}

	b, err := strconv.ParseInt(trimHexPrefix(feature), 16, 0)
	if err != nil {
		log.Fatal(err)
	}

	return (i & b) != 0
}

func trimHexPrefix(s string) string {
	return strings.TrimPrefix(s, "0x")
}

// Client represents a connected client.
type Client struct {
	IP      string
	Feature string
	Release string
}

func (c *Client) Equal(client *Client) bool {
	return c.IP == client.IP
}

func (c *Client) String() string {
	return c.IP + c.Feature + c.Release
}

func (c *Client) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}

	// A session string has the following format:
	// "MonSession(mon.0 10.7.3.65:6789/0 is open allow *, features 0x3ffddff8eea4fffb (luminous))"
	fields := strings.Split(str, " ")
	if len(fields) < 9 {
		return errors.New("unable to parse session string. wrong number of fields")
	}

	host, _, err := net.SplitHostPort(fields[1])
	if err != nil {
		return err
	}

	c.IP = host
	c.Feature = fields[len(fields)-2]
	c.Release = strings.TrimSuffix(strings.TrimPrefix(fields[len(fields)-1], "("), "))")

	return nil
}
