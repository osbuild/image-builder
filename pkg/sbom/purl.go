package sbom

import (
	"strconv"
	"strings"

	"github.com/package-url/packageurl-go"
)

var vendorNamespaces = map[string]string{
	"red hat, inc.":                        "redhat",
	"fedora project":                       "fedora",
	"centos":                               "centos",
	"almalinux":                            "almalinux",
	"rocky enterprise software foundation": "rockylinux",
	"oracle america":                       "oraclelinux",
}

func purlNamespace(vendor string) string {
	v := strings.ToLower(vendor)
	if ns, ok := vendorNamespaces[v]; ok {
		return ns
	}
	return v
}

func rpmPURL(p Package) string {
	qualifiers := map[string]string{"arch": p.Arch}
	if p.Epoch != 0 {
		qualifiers["epoch"] = strconv.FormatUint(uint64(p.Epoch), 10)
	}
	return packageurl.NewPackageURL(
		packageurl.TypeRPM,
		purlNamespace(p.Vendor),
		p.Name,
		p.Version+"-"+p.Release,
		packageurl.QualifiersFromMap(qualifiers),
		"",
	).ToString()
}
