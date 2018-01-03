package grafana

// Taken from https://github.com/matrix-org/gomatrixserverlib/blob/7d789f4fb6fa1624901abf391426c5560d76793f/redactevent.go#L39-L51
type rawJSON []byte

// MarshalJSON implements the json.Marshaller interface using a value receiver.
// This means that rawJSON used as an embedded value will still encode correctly.
func (r rawJSON) MarshalJSON() ([]byte, error) {
	return []byte(r), nil
}

// UnmarshalJSON implements the json.Unmarshaller interface using a pointer receiver.
func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = rawJSON(data)
	return nil
}
