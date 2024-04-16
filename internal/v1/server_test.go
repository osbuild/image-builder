package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/osbuild/image-builder/internal/clients/recommendations"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/clients/composer"
	"github.com/osbuild/image-builder/internal/clients/content_sources"
	"github.com/osbuild/image-builder/internal/clients/provisioning"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/distribution"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/tutils"
)

var dbc *tutils.PSQLContainer

func TestMain(m *testing.M) {
	code := runTests(m)
	os.Exit(code)
}

func runTests(m *testing.M) int {
	d, err := tutils.NewPSQLContainer()
	if err != nil {
		panic(err)
	}

	dbc = d
	code := m.Run()
	defer func() {
		err = dbc.Stop()
		if err != nil {
			logrus.Errorf("Error stopping postgres container: %v", err)
		}
	}()
	return code
}

// Create a temporary file containing quotas, returns the file name as a string
func initQuotaFile(t *testing.T) (string, error) {
	// create quotas with only the default values
	quotas := map[string]common.Quota{
		"default": {Quota: common.DefaultQuota, SlidingWindow: common.DefaultSlidingWindow},
	}
	jsonQuotas, err := json.Marshal(quotas)
	if err != nil {
		return "", err
	}

	// get a temp file to store the quotas
	file, err := os.CreateTemp(t.TempDir(), "account_quotas.*.json")
	if err != nil {
		return "", err
	}

	// write to disk
	jsonFile, err := os.Create(file.Name())
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	_, err = jsonFile.Write(jsonQuotas)
	if err != nil {
		return "", err
	}
	err = jsonFile.Close()
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}

func makeUploadOptions(t *testing.T, uploadOptions interface{}) *composer.UploadOptions {
	data, err := json.Marshal(uploadOptions)
	require.NoError(t, err)

	var result composer.UploadOptions
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	return &result
}

type testServerClientsConf struct {
	ComposerURL  string
	ProvURL      string
	CSURL        string
	RecommendURL string
}

func startServer(t *testing.T, tscc *testServerClientsConf, conf *ServerConfig) (*echo.Echo, *httptest.Server) {
	var log = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}

	err := logger.ConfigLogger(log, "DEBUG")
	require.NoError(t, err)

	tokenServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, "offlinetoken", r.FormValue("refresh_token"))
		require.Equal(t, "refresh_token", r.FormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
		}{
			AccessToken: "accesstoken",
		})
		require.NoError(t, err)
	}))

	tokenServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, "client_credentials", r.FormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
		}{
			AccessToken: "accesstoken",
		})
		require.NoError(t, err)
	}))

	compClient, err := composer.NewClient(composer.ComposerClientConfig{
		ComposerURL:  tscc.ComposerURL,
		TokenURL:     tokenServer1.URL,
		ClientId:     "rhsm-api",
		OfflineToken: "offlinetoken",
	})
	require.NoError(t, err)

	provClient, err := provisioning.NewClient(provisioning.ProvisioningClientConfig{
		URL: tscc.ProvURL,
	})
	require.NoError(t, err)

	csClient, err := content_sources.NewClient(content_sources.ContentSourcesClientConfig{
		URL: tscc.CSURL,
	})
	require.NoError(t, err)

	recommendClient, err := recommendations.NewClient(recommendations.RecommendationsClientConfig{
		URL:          tscc.RecommendURL,
		TokenURL:     tokenServer2.URL,
		ClientId:     "rhsm-api",
		ClientSecret: "grant_type",
	})
	require.NoError(t, err)

	var tokenServer *httptest.Server
	if compClient != nil {
		tokenServer = tokenServer1
	} else if recommendClient != nil {
		tokenServer = tokenServer2
	} else {
		// Handle case when neither compClient nor recommendClient is initialized
		log.Fatalf("Neither compClient nor recommendClient is initialized")
	}

	//store the quotas in a temporary file
	quotaFile, err := initQuotaFile(t)
	require.NoError(t, err)

	echoServer := echo.New()
	echoServer.HideBanner = true
	serverConfig := conf
	if serverConfig == nil {
		serverConfig = &ServerConfig{}
	}

	if serverConfig.DBase == nil {
		dbase, err := dbc.NewDB()
		require.NoError(t, err)
		serverConfig.DBase = dbase
	}
	serverConfig.EchoServer = echoServer
	serverConfig.CompClient = compClient
	serverConfig.ProvClient = provClient
	serverConfig.CSClient = csClient
	serverConfig.RecommendClient = recommendClient
	if serverConfig.QuotaFile == "" {
		serverConfig.QuotaFile = quotaFile
	}
	if serverConfig.DistributionsDir == "" {
		serverConfig.DistributionsDir = "../../distributions"
	}
	if serverConfig.AllDistros == nil {
		adr, err := distribution.LoadDistroRegistry(serverConfig.DistributionsDir)
		require.NoError(t, err)
		serverConfig.AllDistros = adr
	}

	err = Attach(serverConfig)
	require.NoError(t, err)
	// execute in parallel b/c .Run() will block execution
	go func() {
		_ = echoServer.Start("localhost:8086")
	}()

	// wait until server is ready
	tries := 0
	for tries < 5 {
		resp, err := tutils.GetResponseError("http://localhost:8086/status")
		if err == nil {
			defer resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		} else if tries == 4 {
			require.NoError(t, err)
		}
		time.Sleep(time.Second)
		tries += 1
	}

	return echoServer, tokenServer
}
