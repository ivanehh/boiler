package boiler

type Config interface {
	Sources() IOWithAuth
}

type IOWithAuth interface {
	Enabled() bool
	Type() string
	Name() string
	Addr() string
	Auth() Credentials
}

type Credentials interface {
	Username() string
	Password() string
}

type Structurable interface {
	Mapable
	JSONable
}

type Mapable interface {
	AsMap() map[string]any
}

type JSONable interface {
	AsJSON() []byte
}
