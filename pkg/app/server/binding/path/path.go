package path

// PathParam parameter acquisition interface on the URL path
type PathParam interface {
	Get(name string) (string, bool)
}
