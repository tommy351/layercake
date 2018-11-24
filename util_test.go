package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunSeries(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		var nums []int
		err := RunSeries(
			func() error {
				nums = append(nums, 1)
				return nil
			},
			func() error {
				nums = append(nums, 2)
				return nil
			},
			func() error {
				nums = append(nums, 3)
				return nil
			},
		)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, nums)
	})

	t.Run("Interrupt", func(t *testing.T) {
		var nums []int
		returnErr := errors.New("test")
		err := RunSeries(
			func() error {
				nums = append(nums, 1)
				return nil
			},
			func() error {
				nums = append(nums, 2)
				return returnErr
			},
			func() error {
				nums = append(nums, 3)
				return nil
			},
		)
		assert.Equal(t, returnErr, err)
		assert.Equal(t, []int{1, 2}, nums)
	})
}
