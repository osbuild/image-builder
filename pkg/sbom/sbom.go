package sbom

import (
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/google/uuid"
	"github.com/protobom/protobom/pkg/formats"
	pb "github.com/protobom/protobom/pkg/sbom"
	"github.com/protobom/protobom/pkg/writer"
)

// Document is a single software bill of materials document. Create one with
// Generate and serialize it with Write.
type Document struct{ doc *pb.Document }

// ToolInfo identifies the program that created an SBOM.
type ToolInfo struct {
	Name    string
	Version string
}

// Creator identifies who and what created an SBOM.
type Creator struct {
	Tool ToolInfo

	// Organization is the organization that created the SBOM, e.g. "Red Hat".
	// Empty means no Organization creator is recorded.
	Organization string
}

// Image identifies the built image the SBOM describes.
type Image struct {
	// Name is the image package name, supplied by the caller, e.g. "centos-9-qcow2-x86_64". It becomes the SPDX document name.
	Name string

	// FileName is the image file name as saved to disk, e.g.
	// "centos-9-qcow2-x86_64.qcow2".
	FileName string

	// SHA256 is the lowercase hex SHA-256 digest of the image file.
	SHA256 string
}

// Inputs is everything Generate needs to build an image-rooted SBOM.
type Inputs struct {
	Creator Creator
	Image   Image

	// Packages is the caller-supplied RPM inventory.
	Packages []Package
}

// Format identifies a serialization supported by Write.
type Format string

const (
	FormatSPDX23JSON Format = "spdx-2.3-json"
)

// Generate builds an image-rooted SBOM for a built image and its
// packages.
func Generate(in Inputs) (*Document, error) {
	if in.Image.Name == "" {
		return nil, errors.New("image name is missing")
	}
	if err := validateSHA256(in.Image.SHA256); err != nil {
		return nil, err
	}
	if in.Creator.Tool.Name == "" {
		return nil, errors.New("tool name is missing")
	}
	if in.Creator.Tool.Version == "" {
		return nil, errors.New("tool version is missing")
	}

	doc := pb.NewDocument()
	doc.Metadata.Tools = append(doc.Metadata.Tools, &pb.Tool{Name: in.Creator.Tool.Name, Version: in.Creator.Tool.Version})
	if in.Creator.Organization != "" {
		doc.Metadata.Authors = append(doc.Metadata.Authors, &pb.Person{Name: in.Creator.Organization, IsOrg: true})
	}

	doc.Metadata.Name = in.Image.Name
	ns, err := documentNamespace(in.Image.Name)
	if err != nil {
		return nil, err
	}
	doc.Metadata.Id = ns

	image := imageNode(in.Image, in.Creator.Organization)
	doc.NodeList.AddRootNode(image)

	rpms, err := rpmNodes(in.Packages)
	if err != nil {
		return nil, err
	}
	for _, rpm := range rpms {
		doc.NodeList.AddNode(rpm)
		doc.NodeList.AddEdge(&pb.Edge{
			Type: pb.Edge_contains,
			From: image.Id,
			To:   []string{rpm.Id},
		})
	}

	return &Document{doc: doc}, nil
}

// Write serializes the SBOM to w in the given format.
func (s *Document) Write(w io.Writer, f Format) error {
	if f != FormatSPDX23JSON {
		return fmt.Errorf("unsupported SBOM format %q", f)
	}
	return writer.New().WriteStreamWithOptions(s.doc, w, &writer.Options{Format: formats.SPDX23JSON})
}

// documentNamespace appends a random UUID so each document gets a unique
// SPDX namespace, as SPDX 2.3 requires:
// https://spdx.github.io/spdx-spec/v2.3/document-creation-information/#65-spdx-document-namespace-field
func documentNamespace(name string) (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generating document namespace: %w", err)
	}
	return "https://osbuild.org/spdxdocs/" + url.PathEscape(name) + "-" + id.String(), nil
}

// imageNode builds the package node for the built image.
func imageNode(image Image, organization string) *pb.Node {
	node := &pb.Node{
		Id:       nodeID("image\n" + image.SHA256),
		Type:     pb.Node_PACKAGE,
		Name:     image.Name,
		FileName: image.FileName,
		Hashes: map[int32]string{
			int32(pb.HashAlgorithm_SHA256): image.SHA256,
		},
	}
	if organization != "" {
		node.Suppliers = []*pb.Person{{Name: organization, IsOrg: true}}
	}
	return node
}

// nodeID gives nodes stable identities within a document.
func nodeID(v string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(v)).String()
}

func validateSHA256(s string) error {
	if s == "" {
		return errors.New("image sha256 is missing")
	}
	if len(s) != 64 {
		return fmt.Errorf("invalid image sha256 %q", s)
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("invalid image sha256 %q", s)
		}
	}
	return nil
}
