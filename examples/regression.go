package examples

// @immutable
type im struct {
	x int
}

func single() im {
	return im{}
}

func m() (im, int) {
	return im{}, 0
}

func Test() (int, int, int) {
	var x, y int
	t := im{}
	_ = t
	t = single() // CATCH - reassignment
	_ = t
	t, x = im{}, 1 // CATCH - reassignment
	_ = t
	t, y = m() // CATCH - multiple reassignment
	return t.x, x, y
}
