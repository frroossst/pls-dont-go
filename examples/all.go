package examples

import (
	"reflect"
	"unsafe"
)

// @immutable
type ImmutableString string

// @immutable
type ImmutalbeMap map[string]int

// @immutable
type ImmutableFunctionPointer func(int) int

func succ(x int) int {
	return x + 1
}

func prev(x int) int {
	return x - 1
}

func TestTypedefs() {
	// @immutable
	type MyInt int
	// @immutable
	type MyString string

	var mi MyInt = 10
	_ = mi
	mi = 20 // CATCH

	var ms MyString = "immutable"
	_ = ms
	ms = "mutable" // CATCH
}

func TestTypeAliases() {
	var imStr ImmutableString = "hello"
	_ = imStr
	imStr = "world" // CATCH

	// cast to string and mutate
	var normalStr string = string(imStr)
	_ = normalStr
	normalStr = "mutable string" // this is fine cause upto user to cast

	var imMap ImmutalbeMap = ImmutalbeMap{"a": 1, "b": 2, "c": 3}
	_ = imMap
	imMap["d"] = 4 // CATCH
	// cast to normal map and mutate
	var normalMap map[string]int = map[string]int(imMap)
	normalMap["e"] = 5 // this is fine cause upto user to cast

	// reinit the map
	imMap = ImmutalbeMap{"x": 10} // CATCH

	var fnPtr ImmutableFunctionPointer = succ
	_ = fnPtr

	fnPtr = func(x int) int { // CATCH
		return x * 2
	}

	var fnPtr2 ImmutableFunctionPointer = prev
	_ = fnPtr2
	fnPtr2 = succ // CATCH
}

// create multiple threads to test concurrency (some mutation attempts may be missed otherwise)
func TestRaceCondition() {
	im := Immtbl{}

	go func() {
		im.Num = 2001 // CATCH
	}()

	go func() {
		im.Str = "race condition" // CATCH
	}()

	go func() {
		im.Map["race"] = 42 // CATCH
	}()

	select {}
}

type cell struct {
	Value int
}

type ocell struct {
	Cell cell
}

// @immutable
type InnerImmutable struct {
	Data string
}

type OuterMutable struct {
	Inner InnerImmutable
	Value string
}

// Interface-based accessors for examples below
type HasImmutable interface {
	GetImmutable() *Immtbl
}

type Holder struct{ i *Immtbl }

func (h *Holder) GetImmutable() *Immtbl { return h.i }

// @immutable
type Immtbl struct {
	Num int
	Str string
	Arr []int
	Two [][]int
	Map map[string]int
	Cll cell
}

func New() *Immtbl {
	return &Immtbl{
		Num: 42,
		Str: "immutable",
		Arr: []int{1, 2, 3},
		Two: [][]int{{1, 2}, {3, 4}},
		Map: map[string]int{"a": 1, "b": 2},
		Cll: cell{Value: 100},
	}
}

func ReadNum(s *Immtbl) int {
	return s.Num
}

func ReadStr(s *Immtbl) string {
	return s.Str
}

func (s *Immtbl) RecvReadNum() int {
	return s.Num
}

func (s *Immtbl) RecvReadStr() string {
	return s.Str
}

func MutateNum(s *Immtbl) {
	s.Num = 123456789 // CATCH
}

func MutateMap(s *Immtbl) {
	s.Map["catch"] = -7 // CATCH
}

func (s *Immtbl) RecvMutateNum() {
	s.Num = 987654321 // CATCH
}

func (s *Immtbl) RecvMutateMap() {
	s.Map["recv_catch"] = -77 // CATCH
}

func TestAll() {
	im := Immtbl{Num: 123, Str: "hello world!", Two: [][]int{{1, 2, 3}, {4, 5, 6}}}

	/*
	 * Direct field assignments
	 */
	im.Str = "heyo" // CATCH
	im.Num = 99     // CATCH

	/*
	 * Pointer auto-deref assignments
	 */
	imPtr := &im
	imPtr.Num = -99                  // CATCH
	imPtr.Str = "ptr heyo"           // CATCH
	imPtr.Str = (string)("ptr heyo") // CATCH

	/*
	 * python like assignments
	 */
	im.Num += 5  // CATCH
	im.Num -= 1  // CATCH
	im.Num *= 2  // CATCH
	im.Num /= 3  // CATCH
	im.Num %= 2  // CATCH
	im.Num <<= 1 // CATCH
	im.Num >>= 1 // CATCH
	im.Num &= 1  // CATCH
	im.Num |= 2  // CATCH
	im.Num ^= 3  // CATCH

	/*
	 * C-like like assignments
	 */
	im.Num++ // CATCH
	im.Num-- // CATCH

	/*
	 * nested mutations
	 */
	im.Arr = append(im.Arr, 0) // CATCH
	im.Map["x"] = 42           // CATCH
	im.Cll.Value = 456         // CATCH
	im.Two[0][3] = 7           // CATCH

	/*
	 * nested mutable structs
	 */
	oc := ocell{}
	oc.Cell = cell{}
	oc.Cell.Value = 321 // this is fine, ocell and cell are mutable

	/*
	 * mutations via functions
	 * N.B. not caught at call site but inside the function
	 */
	MutateNum(&im)
	MutateMap(&im)
	_ = ReadNum(&im)
	_ = ReadStr(&im)

	/*
	 * mutations via methods
	 * N.B. not caught at call site but inside the function
	 */
	im.RecvMutateNum()
	im.RecvMutateMap()
	_ = im.RecvReadNum()
	_ = im.RecvReadStr()

	/*
	 * mutations via closures
	 */
	func() {
		im.Str = "closure heyo" // CATCH
	}()

	/*
	 * mutations via defer
	 */
	defer func() {
		im.Str = "defer heyo" // CATCH
	}()

	/*
	 * mutations via unsafe and reflect
	 */
	p := (*int)(unsafe.Pointer(&im.Num))
	*p = 100 // CATCH - use of unsafe to mutate immutable

	pP := (&im.Num)
	*pP = 200 // CATCH

	v := reflect.ValueOf(&im).Elem()
	v.FieldByName("Num").SetInt(42) // CATCH - use of reflect to mutate immutable

	/*
	 * reassignments
	 * not sure if these should be caught!
	 */
	im = Immtbl{} // CATCH

	/*
	 * mutations of immutable type inside mutable type
	 */
	outer := OuterMutable{
		Inner: InnerImmutable{
			Data: "initial",
		},
	}
	outer.Value = "this is fine!"
	outer.Inner.Data = "mutated" // CATCH

	/*
	 * mutations through pointer indirection
	 */
	imPtr2 := &im
	ptrPtr := &imPtr2
	(**ptrPtr).Num = 777 // CATCH

	/*
		ptr
		 * mutations through type conversion
	*/
	type Alias = Immtbl
	var aliased Alias = im
	aliased.Num = 888 // CATCH - mutation through type alias

	/*
	 * mutations in composite literals (probably OK to allow?)
	 */
	_ = Immtbl{Num: 1} // this should be fine
	arr := []Immtbl{{Num: 1}}
	arr[0].Num = 999 // CATCH

	/*
	 * mutations through interface{}
	 */
	var iface interface{} = &im
	iface.(*Immtbl).Num = 111 // CATCH

	var iface_any any = &im
	iface_any.(*Immtbl).Num = 112 // CATCH

	/*
	 * mutations through channels
	 */
	ch := make(chan *Immtbl, 1)
	ch <- &im
	received := <-ch
	received.Num = 222 // CATCH

	/*
	 * mutations in range loops
	 */
	immutables := []Immtbl{{Num: 1}, {Num: 2}}
	for i := range immutables {
		immutables[i].Num = 333 // CATCH
	}

	/*
	 * mutations through embedded fields
	 */
	type Wrapper struct {
		Immtbl // embedded
		Other  string
	}
	wrapped := Wrapper{Immtbl: im}
	wrapped.Num = 444        // CATCH
	wrapped.Immtbl.Num = 555 // CATCH

	/*
	 * mutations in switch statements
	 */
	switch true {
	case true:
		im.Num = 666 // CATCH
	}

	/*
	 * mutations in if statements
	 */
	if true {
		im.Num = 777 // CATCH
	}

	/*
	 * mutations through variadic functions
	 */
	func(ptrs ...*Immtbl) {
		ptrs[0].Num = 888 // CATCH
	}(&im)

	/*
	 * mutations through return values (assigning to returned pointer)
	 */
	getImmutable := func() *Immtbl {
		return &im
	}
	getImmutable().Num = 999 // CATCH

	/*
	 * mutations through map values (tricky!)
	 */
	mapOfImmutables := map[string]Immtbl{"key": im}
	val := mapOfImmutables["key"]
	val.Num = 1010 // probably OK - this is a copy
	// but this should be caught:
	mapOfImmutablePtrs := map[string]*Immtbl{"key": &im}
	mapOfImmutablePtrs["key"].Num = 1111 // CATCH

	/*
	 * mutations through defer with named returns (sneaky!)
	 */
	func() (result *Immtbl) {
		result = &im
		defer func() {
			result.Num = 1212 // CATCH
		}()
		return result
	}()

	/*
	 * mutations through method chaining
	 */
	type Container struct {
		immutable *Immtbl
	}
	getContainer := func() *Container {
		return &Container{immutable: &im}
	}
	getContainer().immutable.Num = 1313 // CATCH

	/*
	 * mutations in goroutines
	 */
	go func() {
		im.Num = 1414 // CATCH
	}()

	/*
	 * mutations through select statements
	 */
	select {
	default:
		im.Num = 1515 // CATCH
	}

	/*
	 * mutations by taking address and dereferencing in one line
	 */
	*(&im.Num) = 1616 // CATCH

	// do a more complex expression with address and dereference
	// like multiple pointers and derefs that don't get optimized away
	ptr1 := &im
	ptr2 := &ptr1
	ptr3 := &ptr2
	ptr4 := &ptr3
	ptr5 := &ptr4
	*****ptr5 = Immtbl{Num: 1717} // CATCH

	/*
	 * mutations through slices of pointers
	 */
	sliceOfPtrs := []*Immtbl{&im}
	sliceOfPtrs[0].Num = 1717 // CATCH

	/*
	 * mutations after constructors
	 */
	newIm := New()
	newIm.Num = 1818 // CATCH

	/*
	 * mutations using interfaces (type assertions, type switches, and interface methods)
	 */
	var i1 any = &im
	i1.(*Immtbl).Num = 1919 // CATCH

	var i2 any = &im
	switch v := i2.(type) {
	case *Immtbl:
		v.Num = 2020 // CATCH
	}

	var h HasImmutable = &Holder{i: &im}
	h.GetImmutable().Num = 2121 // CATCH

	mutateViaIface := func(h HasImmutable) {
		h.GetImmutable().Num = 2222 // CATCH
	}
	mutateViaIface(h)

	type NumMutator interface{ RecvMutateNum() }
	var nm NumMutator = &im
	nm.RecvMutateNum()

}
