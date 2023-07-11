package xds

import (
	"time"
)

var (
	defaultTZ = timezoneJST()
)

func timezoneJST() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	return loc
}
