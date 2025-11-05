package examples

// @immutable
type im struct {
	x int
}

func single() im {
	return im{}
}

func m0() (im, int) {
	return im{}, 0
}

func m1() (*im, int) {
	return nil, 0
}

func T0(imm *im, i int)  {
    if imm.x == 0 && i == 0 {
        imm, i = m1() // OK - pointer reassignment
		_, _ = imm, i
    }
}

func T1() (int, int, int) {
	var x, y int
	t := im{}
	_ = t
	t = single() // CATCH - reassignment
	_ = t
	t, x = im{}, 1 // CATCH - reassignment
	_ = t
	t, y = m0() // CATCH - multiple reassignment
	return t.x, x, y
}
