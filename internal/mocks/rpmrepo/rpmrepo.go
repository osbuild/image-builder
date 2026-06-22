package rpmrepo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/rpmmd"
)

type testRepoServer struct {
	Server     *httptest.Server
	RepoConfig rpmmd.RepoConfig
}

func NewTestServer() *testRepoServer {
	server := httptest.NewServer(http.FileServer(http.Dir("../../test/data/testrepo/")))
	testrepo := rpmmd.RepoConfig{
		Name:      "cs9-baseos",
		BaseURLs:  []string{server.URL},
		CheckGPG:  common.ToPtr(false),
		IgnoreSSL: common.ToPtr(true),
		RHSM:      false,
	}
	return &testRepoServer{Server: server, RepoConfig: testrepo}
}

func (trs *testRepoServer) Close() {
	trs.Server.Close()
}

// WriteConfig writes the repository config to the specified path in .repo
// format. Assumes the location already exists.
func (trs *testRepoServer) WriteConfig(path string) {
	cfgtmpl := `[%[1]s]
name=%[1]s
baseurl=%[2]s
gpgcheck=%[3]s
sslverify=%[4]s
`

	checkGPG := "0"
	if trs.RepoConfig.CheckGPG != nil && *trs.RepoConfig.CheckGPG {
		checkGPG = "1"
	}
	sslverify := "1"
	if trs.RepoConfig.IgnoreSSL != nil && *trs.RepoConfig.IgnoreSSL {
		sslverify = "0"
	}

	config := fmt.Sprintf(cfgtmpl, trs.RepoConfig.Name, trs.RepoConfig.BaseURLs[0], checkGPG, sslverify)

	fp, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if _, err := fp.Write([]byte(config)); err != nil {
		panic(err)
	}
}
