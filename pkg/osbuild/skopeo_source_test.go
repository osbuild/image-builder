package osbuild

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/assertx"
	"github.com/osbuild/image-builder/v73/internal/common"
)

func TestNewSkopeoSource(t *testing.T) {
	testDigest := "sha256:f29b6cd42a94a574583439addcd6694e6224f0e4b32044c9e3aee4c4856c2a50"
	imageID := "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"

	source := NewSkopeoSource()

	source.AddItem("name", testDigest, imageID, common.ToPtr(false))
	assert.Len(t, source.Items, 1)

	item, ok := source.Items[imageID]
	assert.True(t, ok)
	assert.Equal(t, item.Image.Name, "name")
	assert.Equal(t, item.Image.Digest, testDigest)
	assert.Equal(t, item.Image.TLSVerify, common.ToPtr(false), false)

	testDigest = "sha256:d49eebefb6c7ce5505594bef652bd4adc36f413861bd44209d9b9486310b1264"
	imageID = "sha256:d2ab8fea7f08a22f03b30c13c6ea443121f25e87202a7496e93736efa6fe345a"

	source.AddItem("name2", testDigest, imageID, nil)
	assert.Len(t, source.Items, 2)
	item, ok = source.Items[imageID]
	assert.True(t, ok)
	assert.Nil(t, item.Image.TLSVerify)

	// empty name
	expectedErr := regexp.MustCompile(`source item osbuild.SkopeoSourceItem.* has empty name`)
	assertx.PanicsWithErrorRegexp(t, expectedErr, func() {
		source.AddItem("", testDigest, imageID, nil)
	})

	// empty digest
	expectedErr = regexp.MustCompile(`source item osbuild.SkopeoSourceItem.* has invalid digest`)
	assertx.PanicsWithErrorRegexp(t, expectedErr, func() {
		source.AddItem("name", "", imageID, nil)
	})

	// empty image id
	assert.PanicsWithError(t, `item "" has invalid image id`, func() {
		source.AddItem("name", testDigest, "", nil)
	})

	// invalid digest
	expectedErr = regexp.MustCompile(`item osbuild.SkopeoSourceItem.* has invalid digest`)
	assertx.PanicsWithErrorRegexp(t, expectedErr, func() {
		source.AddItem("name", "foo", imageID, nil)
	})

	// invalid image id
	assert.PanicsWithError(t, `item "sha256:foo" has invalid image id`, func() {
		source.AddItem("name", testDigest, "sha256:foo", nil)
	})
}
