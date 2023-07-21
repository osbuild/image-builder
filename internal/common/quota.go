package common

import (
	"encoding/json"
	"fmt"
	"io"
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
//	{
//	    "000000":{
//	        "quota":2,
//	        "slidingWindow":1209600000000000
//	    },
//	    "000001":{
//	        "quota":0,
//	        "slidingWindow":1209600000000000
//	    },
//	    "default":{
//	        "quota":100,
//	        "slidingWindow":1209600000000000
//	    }
//	}
//
// The unit for the sliding window is the nanosecond.
type Quota struct {
	Quota         int           `json:"quota"`
	SlidingWindow time.Duration `json:"slidingWindow"`
}

// Returns true if the number of requests made by OrgID during a sliding window is below a threshold.
// The duration of the sliding window and the value of the threshold must be set in a file pointed by the QUOTA_FILE
// environment variable.
// If the variable is unset (or an empty string), the check is disabled and always returns true.
func CheckQuota(orgID string, dB db.DB, quotaFile string) (bool, error) {
	if quotaFile == "" {
		return true, nil
	}
	var authorizedRequests int
	var slidingWindow time.Duration

	// read proper values from quotas' file
	var quotas map[string]Quota
	jsonFile, err := os.Open(filepath.Clean(quotaFile))
	if _, ok := err.(*os.PathError); ok {
		return false, fmt.Errorf("No config file for quotas found at %s\n", quotaFile)
	} else {
		rawJsonFile, err := io.ReadAll(jsonFile)
		if err != nil {
			return false, fmt.Errorf("Failed to read quota file %q: %s", quotaFile, err.Error())
		}
		err = json.Unmarshal(rawJsonFile, &quotas)
		if err != nil {
			return false, fmt.Errorf("Failed to unmarshal quota file %q: %s", quotaFile, err.Error())
		}
		if quota, ok := quotas[orgID]; ok {
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
	count, err := dB.CountComposesSince(orgID, slidingWindow)
	if err != nil {
		return false, err
	}
	return count < authorizedRequests, nil
}
