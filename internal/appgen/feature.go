package appgen

type Feature interface {
	Name() string
	Enabled(Context) bool
	Generate(Context) error
}
