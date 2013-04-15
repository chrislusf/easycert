// Copyright 2013 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command easycert handle certificates to be used in TLS conections.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	// Where the configuration template is installed through "go get".
	_DIR_CONFIG = "github.com/kless/easycert/data"

	_DIR_CA  = ".RootCA" // Directory for the Root Certification Authority
	_NAME_CA = "ca"      // Name for files related to the CA.

	_FILE_CONFIG = "openssl.cfg"
	_FILE_GO     = "cert.go"
)

// File extensions.
const (
	EXT_CERT  = ".crt" // Certificate (can be publicly distributed)
	EXT_KEY   = ".key" // Private key (with restrictive permissions)
	EXT_REVOK = ".crl" // Certificate Revokation List (can be publicly distributed)

	// Certificate Request (it will be signed by the CA in order to create the
	// server certificate. Afterwards it is not needed and can be deleted).
	EXT_REQUEST = ".csr"

	// For files that contain both the Key and the server Certificate since some
	// servers need this. Permissions should be restrictive on these files.
	EXT_CERT_AND_KEY = ".pem"
)

// DirPath represents the directory structure.
type DirPath struct {
	Root  string // Certificate Authority’s directory.
	Cert  string // Where the server certificates are placed.
	Key   string // Where the private keys are placed.
	Revok string // Where the certificate revokation list is placed.

	// Where OpenSSL puts the created certificates in PEM (unencrypted) format
	// and in the form 'cert_serial_number.pem' (e.g. '07.pem')
	NewCert string
}

// FilePath represents the files structure.
type FilePath struct {
	Cmd    string // OpenSSL' path
	Config string // OpenSSL configuration file.
	Index  string // Serves as a database for OpenSSL.
	Serial string // Contains the next certificate’s serial number.

	Cert    string // Certificate.
	Key     string // Private key.
	Request string // Certificate request.
}

var (
	Dir  *DirPath
	File *FilePath
)

// Set the Certificate Authority’s structure.
func init() {
	log.SetFlags(0)
	log.SetPrefix("FAIL! ")

	cmdPath, err := exec.LookPath("openssl")
	if err != nil {
		log.Fatal("OpenSSL is not installed")
	}

	user, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	root := filepath.Join(user.HomeDir, _DIR_CA)

	Dir = &DirPath{
		Root:    root,
		Cert:    filepath.Join(root, "certs"),
		NewCert: filepath.Join(root, "newcerts"),
		Key:     filepath.Join(root, "private"),
		Revok:   filepath.Join(root, "crl"),
	}

	File = &FilePath{
		Cmd:    cmdPath,
		Config: filepath.Join(Dir.Root, _FILE_CONFIG),
		Index:  filepath.Join(Dir.Root, "index.txt"),
		Serial: filepath.Join(Dir.Root, "serial"),
	}
}

// == Flags

var (
	errMinSize = errors.New("key size must be at least of 2048")
	errSize    = errors.New("key size must be multiple of 1024")
)

// keySize represents the size in bits of the RSA key to generate.
type keySize int

func (s *keySize) Set(value string) error {
	i, err := strconv.Atoi(value)
	if err != nil {
		return err
	}

	if i < 2048 {
		return errMinSize
	}
	if i%1024 != 0 {
		return errSize
	}
	*s = keySize(i)
	return nil
}

func (s *keySize) String() string {
	return strconv.Itoa(int(*s))
}

var (
	_IsRootCA  = flag.Bool("root-ca", false, "create the Certification Authority's structure")
	_IsNewReq  = flag.Bool("new-req", false, "create a certificate request")
	_IsSignReq = flag.Bool("sign", false, "sign a certificate request")
	_IsLangGo  = flag.Bool("lang-go", false, "generate file for Go language with certificate in binary")

	_KeySize keySize = 2048 // default
	_Years           = flag.Int("years", 1,
		"number of years a certificate generated is valid;\n\twith flag 'root-ca', the default is 3 years")

	_IsCheck = flag.Bool("chk", false, "checking")

	_IsCert = flag.Bool("cert", false, "the file is a certificate")
	_IsKey  = flag.Bool("key", false, "the file is a private key")

	_IsPrint       = flag.Bool("p", false, "print out information of the certificate")
	_IsPrintHash   = flag.Bool("hash", false, "print the hash value")
	_IsPrintInfo   = flag.Bool("info", false, "print extensive information")
	_IsPrintIssuer = flag.Bool("issuer", false, "print the issuer")
	_IsPrintName   = flag.Bool("name", false, "print the subject")
)

func init() {
	flag.Var(&_KeySize, "size", "size in bits for the RSA key")
}

func usage() {
	fmt.Fprintf(os.Stderr, `Tool to generate and handle certificates.

Usage: easycert [options]

- Create directory structure for the Certification Authority:
	-root-ca [-size -years]

- Generate certificate request:
	-new-req [-size -years -sign] certificate_name
- Sign certificate request:
	-sign certificate_name
- Convert certificate to binary to be used from some language:
	-lang-go certificate_name

- ChecK:
	-chk [-cert|-key] file

- Information:
	-p [-cert|-key] file
	-cert [-hash -issuer -name] file
	-cert -info file...

`)

	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if !*_IsRootCA {
		if len(flag.Args()) == 0 {
			usage()
		}
	}

	if *_IsCheck {
		if *_IsCert {
			CheckCert()
		} else if *_IsKey {
			CheckKey()
		}
		os.Exit(0)
	}

	if *_IsPrint {
		if *_IsCert {
			PrintCert()
		} else if *_IsKey {
			PrintKey()
		}
		os.Exit(0)
	}
	if *_IsCert {
		if *_IsPrintHash {
			PrintHash()
		}
		if *_IsPrintInfo {
			PrintInfo()
		}
		if *_IsPrintIssuer {
			PrintIssuer()
		}
		if *_IsPrintName {
			PrintName()
		}
		os.Exit(0)
	}

	isExit := false
	name := ""

	if !*_IsRootCA {
		name = flag.Args()[0]
	} else {
		name = _NAME_CA
	}
	File.Cert = filepath.Join(Dir.Cert, name+EXT_CERT)
	File.Key = filepath.Join(Dir.Key, name+EXT_KEY)
	File.Request = filepath.Join(Dir.Root, name+EXT_REQUEST)

	if *_IsNewReq {
		if _, err := os.Stat(File.Request); !os.IsNotExist(err) {
			log.Fatalf("Certificate request already exists: %q", File.Request)
		}
		NewRequest()
		isExit = true
	}
	if *_IsSignReq {
		if _, err := os.Stat(File.Cert); !os.IsNotExist(err) {
			log.Fatalf("Certificate already exists: %q", File.Cert)
		}
		if isExit {
			fmt.Print("\n== Sign\n\n")
		}
		SignReq()
		os.Exit(0)
	}
	if isExit {
		os.Exit(0)
	}

	if *_IsLangGo {
		if _, err := os.Stat(_FILE_GO); !os.IsNotExist(err) {
			log.Fatalf("File already exists: %q", _FILE_GO)
		}
		Cert2Go()
		os.Exit(0)
	}

	if *_IsRootCA {
		if _, err := os.Stat(Dir.Root); !os.IsNotExist(err) {
			log.Fatalf("The Certification Authority's structure exists: %q", Dir.Root)
		}
		SetupDir()
		*_Years = 3
		RootCA()
		os.Exit(0)
	}

	usage()
}

// SetupDir creates the directory structure.
func SetupDir() {
	var err error

	for _, v := range []string{Dir.Root, Dir.Cert, Dir.NewCert, Dir.Key, Dir.Revok} {
		if err = os.Mkdir(v, 0755); err != nil {
			log.Fatal(err)
		}
	}
	if err = os.Chmod(Dir.Key, 0710); err != nil {
		log.Fatal(err)
	}

	file, err := os.Create(File.Index)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	file, err = os.Create(File.Serial)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.Write([]byte{'0', '1', '\n'})
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	// Configuration template

	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not get hostname: %s\n\n"+
			"You may want to fix your '/etc/hosts' and/or DNS setup",
			err)
	}

	pkg, err := build.Import(_DIR_CONFIG, build.Default.GOPATH, build.FindOnly)
	if err != nil {
		log.Fatal("Data directory not found\n", err)
	}

	configTemplate := filepath.Join(pkg.Dir, _FILE_CONFIG+".tmpl")
	if _, err = os.Stat(configTemplate); os.IsNotExist(err) {
		log.Fatalf("Configuration template not found: %q", configTemplate)
	}

	tmpl, err := template.ParseFiles(configTemplate)
	if err != nil {
		log.Fatal("Parsing error in configuration: ", err)
	}

	tmpConfigFile, err := os.Create(File.Config)
	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		RootDir  string
		HostName string
		AltNames string
	}{
		Dir.Root,
		host,
		"IP.1 = 127.0.0.1",
	}
	err = tmpl.Execute(tmpConfigFile, data)
	tmpConfigFile.Close()
	if err != nil {
		log.Fatal(err)
	}

	if err = os.Chmod(File.Config, 0600); err != nil {
		log.Print(err)
	}

	fmt.Printf("* Directory structure created in %q\n", Dir.Root)
}

// Cert2Go creates the certificate in binary for Go.
func Cert2Go() {
	CACertBlock, err := ioutil.ReadFile(filepath.Join(Dir.Cert, _NAME_CA+EXT_CERT))
	if err != nil {
		log.Fatal(err)
	}
	certBlock, err := ioutil.ReadFile(File.Cert)
	if err != nil {
		log.Fatal(err)
	}
	keyBlock, err := ioutil.ReadFile(File.Key)
	if err != nil {
		log.Fatal(err)
	}
	// Remove certificate
	/*for _, v := range []string{File.Cert, File.Key} {
		if err = os.Remove(v); err != nil {
			log.Print(err)
		}
	}*/

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	version, err := exec.Command(File.Cmd, "version").Output()
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.OpenFile(_FILE_GO, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	tmpl := template.Must(template.New("").Parse(TEMPLATE_GO))
	data := struct {
		Command   string
		System    string
		Arch      string
		Version   string
		Date      string
		Package   string
		CACert    string
		Cert, Key string
	}{
		strings.Join(os.Args, " "),
		runtime.GOOS,
		runtime.GOARCH,
		string(bytes.TrimRight(version, "\n")),
		time.Now().Format(time.RFC822),
		filepath.Base(wd),
		GoBlock(CACertBlock).String(),
		GoBlock(certBlock).String(),
		GoBlock(keyBlock).String(),
	}

	err = tmpl.Execute(file, data)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}
}