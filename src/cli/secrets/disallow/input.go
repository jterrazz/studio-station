package secretsdisallow

type Input struct {
	Names []string
	Held  map[string][]string
}

func bind(in *Input, args []string) error {
	in.Names = args
	return nil
}
