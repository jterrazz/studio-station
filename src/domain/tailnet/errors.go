package tailnet

type ServeError struct{ Detail string }

func (e *ServeError) Error() string { return "tailscale serve refused: " + e.Detail }
