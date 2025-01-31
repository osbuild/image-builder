#!/bin/bash

set -e

# inspired by from https://github.com/osbuild/osbuild-composer/blob/bdd2014c4467e9325b29624439c98584addc681f/test/cases/api.sh#L230-L262
# thanks Thozza!

# make a dummy rpm for our tests
DUMMYRPMDIR=$(mktemp -d)
DUMMYSPECFILE="$DUMMYRPMDIR/dummy.spec"
pushd "$DUMMYRPMDIR"

cat <<EOF > "$DUMMYSPECFILE"
#----------- spec file starts ---------------
Name:                   dummy
Version:                1.0.0
Release:                0
BuildArch:              noarch
Vendor:                 dummy
Summary:                Provides %{name}
License:                BSD
Provides:               dummy

%description
%{summary}

%files
EOF

mkdir -p "DUMMYRPMDIR/rpmbuild"
rpmbuild --quiet --define "_topdir $DUMMYRPMDIR/rpmbuild" -bb "$DUMMYSPECFILE"
popd

echo "Done building dummy rpm in $DUMMYRPMDIR"
cp "$DUMMYRPMDIR"/rpmbuild/RPMS/noarch/dummy-*.noarch.rpm .
