package main

import (
	"fmt"

	"github.com/venomimonstro/poisk/internal/buildinfo"
)

func requireReleaseBuildIdentity(commit,releaseVersion string)error{
	if err:=buildinfo.ValidateCandidate(commit,releaseVersion);err!=nil{return fmt.Errorf("release binary identity: %w",err)}
	return nil
}
