// Copyright 2013 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"
)

var cmdReq = &Command{
	Run:       runReq,
	UsageLine: "req [-sign] [-rsa-size bits] [-years number] [-host name1,name2,...] NAME",
	Short:     "create certificate request",
	Long: `
"req" creates a certificate signing request (CSR) to be signed by a CA.
`,
}

var errHost = errors.New("must be an IP or DNS")

// hostFlag represents the hostname with IP addresses and/or domain names.
type hostFlag struct {
	ip  []string
	dns []string
}

func (h *hostFlag) String() string {
	ip := strings.Join(h.ip, ", ")
	dns := strings.Join(h.dns, ", ")

	if len(ip) != 0 && len(dns) != 0 {
		return ip + ", " + dns
	}
	return ip + dns
}

func (h *hostFlag) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		v = strings.TrimSpace(v)

		if ip := net.ParseIP(v); ip != nil {
			h.ip = append(h.ip, "IP:"+ip.String())
		} else if strings.ContainsRune(v, '.') {
			h.dns = append(h.dns, "DNS:"+v)
		} else {
			return errHost
		}
	}
	return nil
}

var (
	Host hostFlag

	IsSignReq = flag.Bool("sign", false, "sign a certificate request")
)

func init() {
	flagsForNewCert(cmdReq)

	flag.Var(&Host, "host", "comma-separated hostnames and IPs to generate a certificate for a server")

	sign := flag.Lookup("sign")
	signValue, _ := strconv.ParseBool(sign.Value.String())
	cmdReq.Flag.BoolVar(IsSignReq, sign.Name, signValue, sign.Usage)
}

func runReq(cmd *Command, args []string) {
	if _, err := os.Stat(File.Request); !os.IsNotExist(err) {
		log.Fatalf("Certificate request already exists: %q", File.Request)
	}

	configFile := ""

	if Host.String() != "" {
		if err := serverConfig(); err != nil {
			log.Fatal(err)
		}
		configFile = File.SrvConfig
	} else {
		configFile = File.Config
	}

	opensslArgs := []string{"req", "-new", "-nodes",
		"-config", configFile, "-keyout", File.Key, "-out", File.Request,
		"-newkey", "rsa:" + RSASize.String(),
	}
	fmt.Printf("%s", openssl(opensslArgs...))

	if err := os.Chmod(File.Key, 0400); err != nil {
		log.Print(err)
	}

	fmt.Printf("\n== Generated\n- Request:\t%q\n- Private key:\t%q\n", File.Request, File.Key)
}

// serverConfig generates the configuration according for a server.
func serverConfig() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Could not get hostname: %s\n\n"+
			"You may want to fix your '/etc/hosts' and/or DNS setup",
			err)
	}

	tmpl, err := template.ParseFiles(File.Config + ".tmpl")
	if err != nil {
		return fmt.Errorf("Parsing error in configuration: %s", err)
	}

	configFile, err := os.Create(File.SrvConfig)
	if err != nil {
		return err
	}

	data := struct {
		HostName       string
		SubjectAltName string
	}{
		hostname,
		"subjectAltName = " + Host.String(),
	}
	err = tmpl.Execute(configFile, data)
	configFile.Close()
	if err != nil {
		return err
	}

	return nil
}
