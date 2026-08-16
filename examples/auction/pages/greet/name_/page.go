package name_

// Greet runs between the request and the render.
//
// It is an external the template declares and binds with {val}: the component
// names the route input it needs and what it wants made of it, rather than a
// positional contract between two files.
func Greet(name string) string {
	return "Hello, " + name
}
