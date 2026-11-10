package game

import "strings"

// AppBuildAllowed says whether an app build may use the server, and the minimum that applies to
// its platform. A minimum of 0 locks nothing. build <= 0 means the caller sent no build number
// (a script or curl), which is allowed: the lock is for app builds handed out to players.
func AppBuildAllowed(s Settings, platform string, build int) (minBuild int, allowed bool) {
	minBuild = s.MinAppBuild
	if strings.EqualFold(platform, "ios") {
		minBuild = s.MinAppBuildIOS
	}
	if minBuild <= 0 || build <= 0 {
		return minBuild, true
	}
	return minBuild, build >= minBuild
}
