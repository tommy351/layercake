package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertStringSet(t *testing.T, expected []string, set StringSet) {
	assert.ElementsMatch(t, expected, set.Slice())
}

func TestStringSet_Contains(t *testing.T) {
	set := NewStringSet()
	set.Insert("a", "b")
	assert.True(t, set.Contains("a"))
	assert.True(t, set.Contains("b"))
	assert.False(t, set.Contains("c"))
}

func TestStringSet_Insert(t *testing.T) {
	set := NewStringSet()
	set.Insert("a", "b", "a", "c")
	assertStringSet(t, []string{"a", "b", "c"}, set)
}

func TestStringSet_Delete(t *testing.T) {
	set := NewStringSet()
	set.Insert("a", "b", "c")

	// Exist
	set.Delete("b")
	assertStringSet(t, []string{"a", "c"}, set)

	// Not exist
	set.Delete("d")
	assertStringSet(t, []string{"a", "c"}, set)
}

func TestStringSet_Range(t *testing.T) {
	set := NewStringSet()
	values := []string{"a", "b", "c"}
	set.Insert(values...)

	t.Run("Normal", func(t *testing.T) {
		var actual []string

		set.Range(func(value string) bool {
			actual = append(actual, value)
			return true
		})

		assert.ElementsMatch(t, values, actual)
	})

	t.Run("Interrupt", func(t *testing.T) {
		var actual []string

		set.Range(func(value string) bool {
			actual = append(actual, value)
			return false
		})

		assert.Len(t, actual, 1)
	})
}

func TestStringSet_Len(t *testing.T) {
	set := NewStringSet()
	set.Insert("a", "b", "a", "c")
	assert.Equal(t, 3, set.Len())
}

func TestStringSet_Slice(t *testing.T) {
	set := NewStringSet()
	values := []string{"a", "b", "c"}
	set.Insert(values...)
	assert.ElementsMatch(t, values, set.Slice())
}

func assertOrderedStringSet(t *testing.T, expected []string, set *OrderedStringSet) {
	assert.Equal(t, expected, set.Slice())

	for i, v := range expected {
		assert.Equal(t, i, set.IndexOf(v))
	}
}

func TestOrderedStringSet_Contains(t *testing.T) {
	set := NewOrderedStringSet()
	set.Insert("a", "b")
	assert.True(t, set.Contains("a"))
	assert.True(t, set.Contains("b"))
	assert.False(t, set.Contains("c"))
}

func TestOrderedStringSet_Insert(t *testing.T) {
	set := NewOrderedStringSet()
	set.Insert("c", "a", "b", "a", "c")
	assertOrderedStringSet(t, []string{"c", "a", "b"}, set)
}

func TestOrderedStringSet_Delete(t *testing.T) {
	set := NewOrderedStringSet()
	set.Insert("a", "b", "c")

	set.Delete("b")
	assertOrderedStringSet(t, []string{"a", "c"}, set)

	set.Delete("d")
	assertOrderedStringSet(t, []string{"a", "c"}, set)
}

func TestOrderedStringSet_Range(t *testing.T) {
	set := NewOrderedStringSet()
	values := []string{"a", "b", "c"}
	set.Insert(values...)

	type setItem struct {
		Value string
		Index int
	}

	t.Run("Normal", func(t *testing.T) {
		var actual []setItem

		set.Range(func(value string, idx int) bool {
			actual = append(actual, setItem{
				Value: value,
				Index: idx,
			})

			return true
		})

		assert.Equal(t, []setItem{
			{Value: "a", Index: 0},
			{Value: "b", Index: 1},
			{Value: "c", Index: 2},
		}, actual)
	})

	t.Run("Interrupt", func(t *testing.T) {
		var actual []setItem

		set.Range(func(value string, idx int) bool {
			actual = append(actual, setItem{
				Value: value,
				Index: idx,
			})

			return false
		})

		assert.Equal(t, []setItem{
			{Value: "a", Index: 0},
		}, actual)
	})
}

func TestOrderedStringSet_IndexOf(t *testing.T) {
	set := NewOrderedStringSet()
	set.Insert("a", "b")
	assert.Equal(t, 0, set.IndexOf("a"))
	assert.Equal(t, 1, set.IndexOf("b"))
	assert.Equal(t, -1, set.IndexOf("c"))
}

func TestOrderedStringSet_Len(t *testing.T) {
	set := NewOrderedStringSet()
	set.Insert("a", "b", "a", "c")
	assert.Equal(t, 3, set.Len())
}

func TestOrderedStringSet_Slice(t *testing.T) {
	set := NewOrderedStringSet()
	values := []string{"a", "b", "c"}
	set.Insert(values...)
	assert.Equal(t, values, set.Slice())
}
