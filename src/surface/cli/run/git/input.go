package rungit

type Input struct{ Words []string }

func bind(in *Input, args []string) error {
	in.Words = args
	return nil
}
