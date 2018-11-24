package main

type StringSet map[string]bool

func NewStringSet() StringSet {
	return make(StringSet)
}

func (s StringSet) Contains(value string) bool {
	_, ok := s[value]
	return ok
}

func (s StringSet) Insert(values ...string) {
	for _, v := range values {
		s[v] = true
	}
}

func (s StringSet) Delete(value string) {
	delete(s, value)
}

func (s StringSet) Range(fn func(value string) bool) {
	for k := range s {
		if !fn(k) {
			break
		}
	}
}

func (s StringSet) Len() int {
	return len(s)
}

func (s StringSet) Slice() (result []string) {
	for key := range s {
		result = append(result, key)
	}

	return
}

type OrderedStringSet struct {
	arr []string
	m   map[string]int
}

func NewOrderedStringSet() *OrderedStringSet {
	return &OrderedStringSet{
		arr: []string{},
		m:   map[string]int{},
	}
}

func (o *OrderedStringSet) Contains(value string) bool {
	_, ok := o.m[value]
	return ok
}

func (o *OrderedStringSet) Insert(values ...string) {
	for _, v := range values {
		o.insert(v)
	}
}

func (o *OrderedStringSet) insert(value string) {
	if _, ok := o.m[value]; !ok {
		o.m[value] = len(o.arr)
		o.arr = append(o.arr, value)
	}
}

func (o *OrderedStringSet) Delete(value string) {
	idx, ok := o.m[value]

	if ok {
		left := o.arr[0:idx]
		right := o.arr[idx+1:]
		o.arr = append(left, right...)

		// Delete the value from the map
		delete(o.m, value)

		// Update indices of values on right hand side
		for i, v := range right {
			o.m[v] = idx + i
		}
	}
}

func (o *OrderedStringSet) Range(fn func(value string, idx int) bool) {
	for i, v := range o.arr {
		if !fn(v, i) {
			break
		}
	}
}

func (o *OrderedStringSet) IndexOf(value string) int {
	idx, ok := o.m[value]

	if ok {
		return idx
	}

	return -1
}

func (o *OrderedStringSet) Len() int {
	return len(o.arr)
}

func (o *OrderedStringSet) Slice() []string {
	return o.arr
}
