package images

type BuildRequest struct {
	Images []string
	Binary string
}

type Step struct{ Label string }
