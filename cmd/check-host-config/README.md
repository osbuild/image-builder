## check-host-config

A command used to test the environment against build configuration JSON. It
takes a configuration file and validates each customization. Before validation
starts, it waits until systemd reports the system as fully booted.

The command is safe to run on development machines; it does not change
configuration or leave resources behind.

### Implementing new checks

Each check is a function that takes the metadata and configuration struct. It
returns an error, or nil when the check succeeds.

Metadata information must be registered:

```go
func init() {
	RegisterCheck(Metadata{
		Name:                   "users",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
        TempDisabled:           "",
        RunOn:                  []string{"centos", "!rhel"},
	}, usersCheck)
}
```

Metadata fields have the following semantics:

* **Name**: name of the check.
* **RequiresBlueprint**: when set to true and the blueprint is empty, the check is automatically skipped.
* **RequiresCustomizations**: when set to true and config, blueprint, or customizations is nil, the check is skipped.
* **TempDisabled**: the check is temporarily disabled (skipped) when this is not an empty string (e.g. issue URL).
* **RunOn**: when set, run on specific distro IDs (or use bang to exclude specific OS).

Checks can return:

* pass — `Pass()` function (returns `nil`)
* warning — `Warning(reason)` function
* error — `Fail(reason)` function
* skip — `Skip(reason)` function

### Unit testing

For complex checks (e.g. OpenSCAP, DNF, RPM), unit tests help identify problems
early. Functions such as `Exec`, `ExecString`, `Exists`, `Grep`, and `ReadFile`
are available in the package and can be mocked using helpers in unit tests. Most
tests except OpenSCAP checks are clearer in tabular form; prefer that style. For
readability, the package provides helper types and functions in
`mock_helpers_test.go` that allow reusing the following testing pattern:

	tests := []struct {
		name         string
		config       *blueprint.KernelCustomization
		mockExec     map[string]ExecResult
		mockReadFile map[string]ReadFileResult
		wantErr      error
	}

Each test case has a name and configuration, and returns either nil or an
error. It can also define one or more mocks, represented as maps from input
(typically a string, or a struct for functions with multiple arguments) to
output (a result struct). The mock maps can be formatted like this:

    {
        name: "fail when append does not match",
        config: &blueprint.KernelCustomization{
            Append: "debug",
        },
        mockReadFile: map[string]ReadFileResult{
            "/proc/cmdline": {Data: []byte("root=UUID=1234-5678 ro")},
        },
        wantErr: check.ErrCheckFailed,
    },

The helper functions install these mocks:

    installMockExec(t, tt.mockExec)
    installMockReadFile(t, tt.mockReadFile)

### Smoke (end-to-end) tests

In addition to unit tests, `main_test.go` contains a set of "smoke" tests that
run in a Fedora container, which is set up so the tests can pass. To build the
container locally and run the tests, run `make host-check-test`.
