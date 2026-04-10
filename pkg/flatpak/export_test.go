package flatpak

func RegistryTypeFromURI(uri string) (RegistryType, error) {
	return registryTypeFromURI(uri)
}
