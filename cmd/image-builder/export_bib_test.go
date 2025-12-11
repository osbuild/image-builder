package main

var (
	CanChownInPath    = canChownInPath
	BuildCobraCmdline = bibBuildCobraCmdline
	HandleAWSFlags    = handleAWSFlags
)

func MockOsGetuid(new func() int) (restore func()) {
	saved := osGetuid
	osGetuid = new
	return func() {
		osGetuid = saved
	}
}

func MockOsReadFile(new func(string) ([]byte, error)) (restore func()) {
	saved := osReadFile
	osReadFile = new
	return func() {
		osReadFile = saved
	}
}
