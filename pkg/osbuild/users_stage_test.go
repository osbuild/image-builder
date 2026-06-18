package osbuild

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/customizations/users"
)

func TestNewUsersStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.users",
		Options: &UsersStageOptions{},
	}
	actualStage := NewUsersStage(&UsersStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewUsersStageOptionsPassword(t *testing.T) {
	Pass := "testpass"
	EmptyPass := ""
	CryptPass := "$6$RWdHzrPfoM6BMuIP$gKYlBXQuJgP.G2j2twbOyxYjFDPUQw8Jp.gWe1WD/obX0RMyfgw5vt.Mn/tLLX4mQjaklSiIzoAW3HrVQRg4Q." // #nosec G101

	users := []users.User{
		{
			Name:     "bart",
			Password: &Pass,
		},
		{
			Name:     "lisa",
			Password: &CryptPass,
		},
		{
			Name:     "maggie",
			Password: &EmptyPass,
		},
		{
			Name: "homer",
		},
	}

	options, err := NewUsersStageOptions(users, false)
	require.Nil(t, err)
	require.NotNil(t, options)

	// bart's password should now be a hash
	assert.True(t, strings.HasPrefix(*options.Users["bart"].Password, "$6$"))

	// lisa's password should be left alone (already hashed)
	assert.Equal(t, CryptPass, *options.Users["lisa"].Password)

	// maggie's password should now be nil (locked account)
	assert.Nil(t, options.Users["maggie"].Password)

	// homer's password should still be nil (locked account)
	assert.Nil(t, options.Users["homer"].Password)
}

func TestGenUsersStageSameAsNewUsersStageOptions(t *testing.T) {
	users := []users.User{
		{
			Name: "user1", UID: common.ToPtr(1000), GID: common.ToPtr(1000),
			Groups:      []string{"grp1"},
			Description: common.ToPtr("some-descr"),
			Home:        common.ToPtr("/home/user1"),
			Shell:       common.ToPtr("/bin/zsh"),
			Key:         common.ToPtr("some-key"),
		},
	}
	expected := &UsersStageOptions{
		Users: map[string]UsersStageOptionsUser{
			"user1": {
				UID:         common.ToPtr(1000),
				GID:         common.ToPtr(1000),
				Groups:      []string{"grp1"},
				Description: common.ToPtr("some-descr"),
				Home:        common.ToPtr("/home/user1"),
				Shell:       common.ToPtr("/bin/zsh"),
				Key:         common.ToPtr("some-key")},
		},
	}

	// check that NewUsersStageOptions creates the expected user options
	opts, err := NewUsersStageOptions(users, false)
	require.Nil(t, err)
	assert.Equal(t, opts, expected)

	// check that GenUsersStage creates the expected user options
	st, err := GenUsersStage(users, false)
	require.Nil(t, err)
	usrStageOptions := st.Options.(*UsersStageOptions)
	assert.Equal(t, usrStageOptions, expected)

	// and (for good measure, not really needed) check that both gen
	// the same
	assert.Equal(t, usrStageOptions, opts)
}

func TestNewUsersStageOptionsForLockedRoot(t *testing.T) {
	options := NewUsersStageOptionsForLockedRoot()
	require.NotNil(t, options)

	assert.Equal(t, "!locked", *options.Users["root"].Password)
}
