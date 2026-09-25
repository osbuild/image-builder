### Useful cmds

The following utilities, defined in the `cmd/` directory, are useful for
development and testing. They **should not** be relied on for production
purposes. In particular, command line options and default behavior can change
at any time.

The following are high level descriptions of what some of the utilities can do
and how they can be used during development. For specific flags and options,
refer to each command's help output and doc strings.

Each utility can be compiled using `go build -o <outputfile> ./cmd/<utility>`
or run directly using `go run ./cmd/<utility>`. Use `go run ./cmd/<utility>
-help` for option descriptions (e.g. `go run ./cmd/gen-manifests -help`).

#### Manifest generation

The `gen-manifests` tool can be used to generate all or a subset of the
manifests for the images defined in the repository. This is useful for quickly
seeing the effects of changes in image definitions on the manifest and the
image build itself. While manifests are meant to be machine readable, it is
often much faster to inspect the difference between two manifests (before and
after a change in code) to evaluate if a change is having the desired effect.

Manifests can be generated with or without content resolution (e.g. package
depsolving, containers, ostree commits). If you are working on changes in image
definitions that do not rely on content (e.g. an image type's partition table),
manifests without resolved content can be generated almost instantly. Note that
even though content is not resolved and packages are not depsolved, the
selected packages without their dependencies are still added to generated
manifests, so disabling package depsolving can also be used to inspect package
selection without dependencies.

Manifests should be generated with all content enabled if they are going to be
built. A common workflow when working on changing image definitions, or adding
a new image type, might be:
1. Generate the manifests for the image types that you will be working on.
2. Make changes in an existing image definition or add a new image type.
3. Add appropriate configuration changes:
    - If a new image type is added, add it to the [config
      list](test/config-list.json) under an appropriate configuration file or
      write a new one.
    - If an existing image type is being modified, and the change depends on an
      image customization, make sure the modification is covered by an existing
      [test config](test/configs).
4. Generate the relevant manifests without content (`-packages=false
   -containers=false -commits=false`).
    - If the change depends on a customization, it might be more useful to
      generate multiple manifests with different configuration options set and
      inspect the differences between them.
5. Inspect the differences between manifests generated in steps 1 and 4.
6. Generate manifest with all content enabled for the relevant image types.
7. Build at least one of the manifests using `osuild` and inspect the output
   (boot the image or mount it to look for the desired changes).

_NOTE: By default, manifests created with the `gen-manifest` tool contain extra
metadata. The manifest itself is stored under the key "manifest". You can
extract the actual manifest using `jq .manifest
<manifestfile>.json`. Alternatively, you can generate manifests without
metadata using the `-metadata=false` option._

#### Diffing manifests

When working on image definitions it is often useful to see the effect
this has on existing manifests. The tool `gen-manifests-diff` will generate
a diff against the osbuild manifests produced by the current upstream "main"
branch of the "images" library. If a first argument is passed the diff
is generated against the given revision or git tag of upstream.

Note that no content is resolved, just like in step 4 of [Manifest
generation](#manifest-generation) described above. This is done so that the
diff is small and fast. Because of this, the following should be noted:
- Repository changes are not caught. As long as the repository URLs and
  configurations are the same between runs, the same exact manifest is
  generated.
- Upstream packaging dependency changes will not be caught. Similar to the
  previous note, no changes in package versions or dependencies will occur if
  the repository URLs and configurations don't change.
- Functionality that depends on package inclusion or package versions might not
  behave as expected. This can have several side effects:
    - If a stage or stage option depends on a specific package being in the
      image and that package is only added as a dependency, the manifest will
      always be generated as if the package is not included. The alternative
      functionality will never be visible in a manifest, unless the package is
      added to an image type's package list explicitly or is selected in the
      blueprint.
    - If a stage or stage option depends on a specific package version, the
      manifest will always be generated with the same behaviour (depending on
      the mock package version generated for the unresolved manifest), so a
      change in this behaviour between manifests or between commits will never
      be visible.
- The same notes apply for containers and ostree commits. Remote changes in a
  container registry or ostree repository will never be visible in manifests
  with unresolved content unless the URLs and refs change.

#### Building images

You can build an image by generating its manifest and then running
osbuild. Alternatively, the `cmd/build` tool can perform both steps in one
call. It will generate a manifest, build the image, and store both the image
and its manifest in the output directory.

The build tool must be run as root because image building with osbuild requires
superuser privileges. It is **not recommended** to run `sudo go run
./cmd/build` however. The `go run` command can make changes to the go build
cache and if these changes are made as root, it can cause issues when running
other go commands in the future as a regular user. Instead, it is recommended
to first build the binary and then run it as root:
```
go build -o bin/build ./cmd/build
sudo ./bin/build ...
```

#### Booting images

You can boot an image in its target environment by using the appropriate
command from `cmd/`. _Currently, only AWS is supported._

For example, to boot an AMI or EC2 image, you can use the `./cmd/boot-aws`
command with the `setup` subcommand:
```bash
go run ./cmd/boot-aws setup \
     --region "${AWS_REGION}" \
     --ami "${AMI_ID}" \
     --username "${USERNAME}" \
     --arch "${IMAGE_ARCHITECTURE}" \
     --ssh-pubkey "${PATH_TO_SSH_PUBLIC_KEY}" \
     --vpc-id "${VPC_ID}" \
     --subnet-id "${SUBNET_ID}" \
     --repo-file "${PATH_TO_REPOSITORY_FILE}" \
     --resourcefile ./aws-test-resources.json
```
where:
- `${AWS_REGION}` is the AWS region to use,
- `${AMI_ID}` is the ID of an existing AMI,
- `${USERNAME}` is the username to set up on the instance,
- `${IMAGE_ARCHITECTURE}` is the hardware architecture of the image being
  uploaded and booted,
- `${PATH_TO_SSH_PUBLIC_KEY}` points to a public SSH key,
- `${VPC_ID}` and `${SUBNET_ID}` optionally select the VPC and subnet to use;
  if either option is specified, both are required,
- `${PATH_TO_REPOSITORY_FILE}` optionally points to a `.repo` file to install
  in `/etc/yum.repos.d`. The `--repo-file` option can be repeated to install
  multiple repository files.

AWS credentials are loaded from the default AWS credential chain. They can
instead be supplied explicitly with `--access-key-id`, `--secret-access-key`,
and optionally `--session-token`.

The command creates a security group configured to allow SSH access and
launches an instance from the AMI. It waits until the instance is ready and
prints its public IP address, falling back to its private IP address when no
public address is assigned. It also uses the public SSH key and provided
username to configure cloud-init to create a user and set the SSH key on first
boot. When using a private address, the host running the command must have a
route to the selected subnet so it can connect to the instance over SSH.

The IDs of all created resources are stored in the file specified by the
`--resourcefile` flag. This can be used to tear down all the resources created
by the `setup` subcommand:
```bash
go run ./cmd/boot-aws teardown \
     --resourcefile ./aws-test-resources.json
```

Alternatively, a setup-test-teardown procedure can be run in a single command using the `run` subcommand:
```bash
go run ./cmd/boot-aws run \
     --region "${AWS_REGION}" \
     --ami "${AMI_ID}" \
     --username "${USERNAME}" \
     --arch "${IMAGE_ARCHITECTURE}" \
     --ssh-pubkey "${PATH_TO_SSH_PUBLIC_KEY}" \
     --ssh-privkey "${PATH_TO_SSH_PRIVATE_KEY}" \
     --vpc-id "${VPC_ID}" \
     --subnet-id "${SUBNET_ID}" \
     --repo-file "${PATH_TO_REPOSITORY_FILE}" \
     ${PATH_TO_EXECUTABLE}
```

This performs the same steps as the `setup` subcommand, uploads the specified
executable to the instance, runs it, and then performs the same actions as the
`teardown` subcommand.

#### Listing available image type configurations

The `cmd/list-images` utility simply lists all available combinations of
distribution, architecture, and image type. It also supports filtering one or
more of those three variables.
