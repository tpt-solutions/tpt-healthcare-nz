package mock

// TokenFunc returns a fixed or rotating token function for auth tests.
func TokenFunc(token string, err error) func() (string, error) {
	return func() (string, error) {
		return token, err
	}
}

// RotatingTokenFunc returns a token function that returns different tokens on each call.
func RotatingTokenFunc(tokens []string) func() (string, error) {
	i := 0
	return func() (string, error) {
		if i >= len(tokens) {
			return tokens[len(tokens)-1], nil
		}
		t := tokens[i]
		i++
		return t, nil
	}
}
