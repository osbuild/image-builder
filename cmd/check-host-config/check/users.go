package check

import (
	"log"

	"github.com/osbuild/images/internal/buildconfig"
)

func init() {
	RegisterCheck(Metadata{
		Name:                   "users",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, usersCheck)
}

func usersCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	users := config.Blueprint.Customizations.User
	if len(users) == 0 {
		return Skip("no users to check")
	}

	for _, user := range users {
		stdout, _, _, err := ExecString("id", user.Name)
		if err != nil {
			return Fail("user does not exist:", user.Name)
		}
		log.Printf("User %s exists: %s\n", user.Name, stdout)
	}

	return Pass()
}
