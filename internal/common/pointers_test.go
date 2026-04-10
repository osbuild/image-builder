package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPtr(t *testing.T) {
	valueInt := 42
	gotInt := ToPtr(valueInt)
	assert.Equal(t, valueInt, *gotInt)

	valueBool := true
	gotBool := ToPtr(valueBool)
	assert.Equal(t, valueBool, *gotBool)

	valueUint64 := uint64(1)
	gotUint64 := ToPtr(valueUint64)
	assert.Equal(t, valueUint64, *gotUint64)

	valueStr := "the-greatest-test-value"
	gotStr := ToPtr(valueStr)
	assert.Equal(t, valueStr, *gotStr)

}

func TestClonePtr(t *testing.T) {
	t.Run("nil pointer returns nil", func(t *testing.T) {
		var p *int
		result := ClonePtr(p)
		assert.Nil(t, result)
	})

	t.Run("non-nil pointer returns independent copy", func(t *testing.T) {
		original := 42
		p := &original
		result := ClonePtr(p)

		// Should have same value
		assert.Equal(t, 42, *result)

		// Should be different pointer
		assert.NotSame(t, p, result)

		// Modifying original should not affect clone
		original = 100
		assert.Equal(t, 42, *result)
	})
}

func TestValueOrEmpty(t *testing.T) {
	var ptrInt *int
	valueInt := ValueOrEmpty(ptrInt)
	assert.Equal(t, 0, valueInt)
	helperInt := 20
	ptrInt = &helperInt
	valueInt = ValueOrEmpty(ptrInt)
	assert.Equal(t, 20, valueInt)

	var ptrBool *bool
	valueBool := ValueOrEmpty(ptrBool)
	assert.Equal(t, false, valueBool)
	helperBool := true
	ptrBool = &helperBool
	valueBool = ValueOrEmpty(ptrBool)
	assert.Equal(t, true, valueBool)

	var ptrUint64 *uint64
	valueUint64 := ValueOrEmpty(ptrUint64)
	assert.Equal(t, uint64(0), valueUint64)
	helperUint64 := uint64(20)
	ptrUint64 = &helperUint64
	valueUint64 = ValueOrEmpty(ptrUint64)
	assert.Equal(t, uint64(20), valueUint64)

	var ptrString *string
	valueString := ValueOrEmpty(ptrString)
	assert.Equal(t, "", valueString)
	helperString := "the-greatest-test-value"
	ptrString = &helperString
	valueString = ValueOrEmpty(ptrString)
	assert.Equal(t, "the-greatest-test-value", valueString)
}
