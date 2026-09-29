package buildinfo

import (
	"errors"
	"regexp"
	"strings"
)

var (
	GitCommit = "dev"
	ReleaseVersion = "dev"
)

var (
	gitCommitPattern=regexp.MustCompile(`^[0-9a-f]{40}$`)
	releaseVersionPattern=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

func ValidateCandidate(commit,release string)error{
	commit=strings.ToLower(strings.TrimSpace(commit));release=strings.TrimSpace(release)
	builtCommit:=strings.ToLower(strings.TrimSpace(GitCommit));builtRelease:=strings.TrimSpace(ReleaseVersion)
	if !gitCommitPattern.MatchString(builtCommit){return errors.New("backend binary has no immutable release git commit")}
	if !releaseVersionPattern.MatchString(builtRelease)||builtRelease=="dev"{return errors.New("backend binary has no valid immutable release version")}
	if !releaseVersionPattern.MatchString(release)||release=="dev"{return errors.New("candidate release version is invalid")}
	if commit!=builtCommit{return errors.New("READINESS_GIT_SHA does not match backend binary git commit")}
	if release!=builtRelease{return errors.New("RELEASE_VERSION does not match backend binary release version")}
	return nil
}
