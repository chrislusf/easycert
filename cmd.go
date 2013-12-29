// Copyright 2013 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"flag"
	"strconv"

	"github.com/kless/flagplus"
)

var (
	errMinSize = errors.New("key size must be at least of 2048")
	errSize    = errors.New("key size must be multiple of 1024")
)

// rsaSizeFlag represents the size in bits of RSA key to generate.
type rsaSizeFlag int

func (s *rsaSizeFlag) String() string {
	return strconv.Itoa(int(*s))
}

func (s *rsaSizeFlag) Set(value string) error {
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
	*s = rsaSizeFlag(i)
	return nil
}

// Flags set by multiple commands.
var (
	RSASize rsaSizeFlag = 2048 // default

	Years = flag.Int("years", 1, "number of years a certificate generated is valid")

	IsRequest = flag.Bool("req", false, "request")
	IsCert    = flag.Bool("cert", false, "certificate")
	IsKey     = flag.Bool("key", false, "private key")
)

func init() {
	flag.Var(&RSASize, "rsa-size", "size in bits for the RSA key")
}

// * * *

// flagsForNewCert adds the common flags to the "ca" and "req" commands.
func flagsForNewCert(cmd *flagplus.Command) {
	_RSASize := flag.Lookup("rsa-size")
	cmd.Flag.Var(&RSASize, _RSASize.Name, _RSASize.Usage)

	_Years := flag.Lookup("years")
	_Years_Value, _ := strconv.Atoi(_Years.Value.String())
	cmd.Flag.IntVar(Years, _Years.Name, _Years_Value, _Years.Usage)
}

// flagsForFileType adds the common flags to the "cat", "chk", and "ls" commands.
func flagsForFileType(cmd *flagplus.Command) {
	_IsRequest := flag.Lookup("req")
	_IsRequest_Value, _ := strconv.ParseBool(_IsRequest.Value.String())
	cmd.Flag.BoolVar(IsRequest, _IsRequest.Name, _IsRequest_Value, _IsRequest.Usage)

	_IsCert := flag.Lookup("cert")
	_IsCert_Value, _ := strconv.ParseBool(_IsCert.Value.String())
	cmd.Flag.BoolVar(IsCert, _IsCert.Name, _IsCert_Value, _IsCert.Usage)

	_IsKey := flag.Lookup("key")
	_IsKey_Value, _ := strconv.ParseBool(_IsKey.Value.String())
	cmd.Flag.BoolVar(IsKey, _IsKey.Name, _IsKey_Value, _IsKey.Usage)
}
