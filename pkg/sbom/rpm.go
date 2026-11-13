package sbom

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/osbuild/image-builder/pkg/rpmmd"
	pb "github.com/protobom/protobom/pkg/sbom"
)

func rpmNodes(packages []Package) ([]*pb.Node, error) {
	nodes := make([]*pb.Node, 0, len(packages))
	seen := make(map[string]rpmmd.Checksum, len(packages))
	for _, p := range packages {
		node, err := rpmNode(p)
		if err != nil {
			return nil, err
		}
		purl := node.Identifiers[int32(pb.SoftwareIdentifierType_PURL)]
		if recorded, ok := seen[purl]; ok {
			if recorded != p.Checksum {
				return nil, fmt.Errorf("duplicate package %q has checksum %s and %s", purl, recorded, p.Checksum)
			}
			continue
		}
		seen[purl] = p.Checksum
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(a, b *pb.Node) int {
		return strings.Compare(
			a.Identifiers[int32(pb.SoftwareIdentifierType_PURL)],
			b.Identifiers[int32(pb.SoftwareIdentifierType_PURL)],
		)
	})
	return nodes, nil
}

func rpmNode(p Package) (*pb.Node, error) {
	for _, field := range []struct{ name, value string }{
		{"name", p.Name}, {"version", p.Version}, {"release", p.Release}, {"arch", p.Arch},
	} {
		if strings.TrimSpace(field.value) == "" {
			return nil, fmt.Errorf("package %q: missing %s", p.Name, field.name)
		}
	}
	purl := rpmPURL(p)
	hashes, err := packageHashes(p)
	if err != nil {
		return nil, err
	}
	node := &pb.Node{
		Id:       nodeID(purl),
		Type:     pb.Node_PACKAGE,
		Name:     p.Name,
		Version:  p.Version + "-" + p.Release,
		FileName: rpmFileName(p),
		UrlHome:  p.URL,
		Hashes:   hashes,
		Identifiers: map[int32]string{
			int32(pb.SoftwareIdentifierType_PURL): purl,
		},
	}
	if purlNamespace(p.Vendor) == "redhat" {
		node.Suppliers = []*pb.Person{{Name: "Red Hat", IsOrg: true}}
	}
	return node, nil
}

func packageHashes(p Package) (map[int32]string, error) {
	if p.Checksum.Value == "" {
		return nil, fmt.Errorf("package %s: missing checksum", p.Name)
	}
	if p.Checksum.Type == "" {
		return nil, fmt.Errorf("package %s: missing checksum type", p.Name)
	}
	algo, err := hashAlgorithm(p.Checksum.Type)
	if err != nil {
		return nil, fmt.Errorf("package %s: %w", p.Name, err)
	}
	size := map[pb.HashAlgorithm]int{
		pb.HashAlgorithm_MD5: 16, pb.HashAlgorithm_SHA1: 20,
		pb.HashAlgorithm_SHA256: 32, pb.HashAlgorithm_SHA384: 48, pb.HashAlgorithm_SHA512: 64,
	}[algo]
	digest, err := hex.DecodeString(p.Checksum.Value)
	if err != nil || len(digest) != size || p.Checksum.Value != strings.ToLower(p.Checksum.Value) {
		return nil, fmt.Errorf("package %s: invalid %s checksum %q", p.Name, p.Checksum.Type, p.Checksum.Value)
	}
	return map[int32]string{int32(algo): p.Checksum.Value}, nil
}

// hashAlgorithm maps an RPM checksum type to the protobom enum.
// The set matches the digest curl will fetch: md5, sha1, sha256, sha384, sha512.
func hashAlgorithm(typ string) (pb.HashAlgorithm, error) {
	switch strings.ToLower(typ) {
	case "md5":
		return pb.HashAlgorithm_MD5, nil
	case "sha1":
		return pb.HashAlgorithm_SHA1, nil
	case "sha256":
		return pb.HashAlgorithm_SHA256, nil
	case "sha384":
		return pb.HashAlgorithm_SHA384, nil
	case "sha512":
		return pb.HashAlgorithm_SHA512, nil
	default:
		return pb.HashAlgorithm_UNKNOWN, fmt.Errorf("unsupported checksum type %q", typ)
	}
}

// rpmFileName is the on-disk RPM filename. Epoch is not part of it.
func rpmFileName(p Package) string {
	return fmt.Sprintf("%s-%s-%s.%s.rpm", p.Name, p.Version, p.Release, p.Arch)
}
