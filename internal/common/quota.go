package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/osbuild/image-builder/internal/db"
)

const (
	day                  time.Duration = 24 * time.Hour
	week                 time.Duration = 7 * day
	DefaultSlidingWindow time.Duration = 2 * week
	DefaultQuota         int           = 100
)

// the QUOTA_FILE needs to contain data arranged as such:
//
// {
//     "000000":{
//         "quota":2,
//         "slidingWindow":1209600000000000
//     },
//     "000001":{
//         "quota":0,
//         "slidingWindow":1209600000000000
//     },
//     "default":{
//         "quota":100,
//         "slidingWindow":1209600000000000
//     }
// }
//
// The unit for the sliding window is the nanosecond.
type Quota struct {
	Quota         int           `json:"quota"`
	SlidingWindow time.Duration `json:"slidingWindow"`
}

// Returns true if the number of requests made by accountNumber during a sliding window is below a threshold.
// The duration of the sliding window and the value of the threshold must be set in a file pointed by the QUOTA_FILE
// environment variable.
func CheckQuota(accountNumber string, dB db.DB, quotaFile string) (bool, error) {
	var authorizedRequests int
	var slidingWindow time.Duration

	// read proper values from quotas' file
	var quotas map[string]Quota
	jsonFile, err := os.Open(filepath.Clean(quotaFile))
	if _, ok := err.(*os.PathError); ok {
		return false, fmt.Errorf("No config file for quotas found at %s\n", quotaFile)
	} else {
		rawJsonFile, _ := ioutil.ReadAll(jsonFile)
		err = json.Unmarshal(rawJsonFile, &quotas)
		if err != nil {
			return false, err
		}
		if quota, ok := quotas[accountNumber]; ok {
			authorizedRequests = quota.Quota
			slidingWindow = quota.SlidingWindow
		} else if quota, ok := quotas["default"]; ok {
			authorizedRequests = quota.Quota
			slidingWindow = quota.SlidingWindow
		} else {
			return false, fmt.Errorf("No default values in the quotas' file %s\n", quotaFile)
		}
	}

	// read user created requests
	count, err := dB.CountComposesSince(accountNumber, slidingWindow)
	if err != nil {
		return false, err
	}
	return count < authorizedRequests, nil
}
