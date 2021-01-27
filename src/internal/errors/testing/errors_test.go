package testing

import (
	"fmt"
	"testing"

	"github.com/pachyderm/pachyderm/v2/src/internal/errors"
	"github.com/pachyderm/pachyderm/v2/src/internal/require"
)

// FooErr is an error with a normal receiver for fulfilling the error interface
type FooErr struct {
	val int
}

func (err FooErr) Error() string {
	return fmt.Sprintf("foo-%d", err.val)
}

// PtrFooErr is an error with a pointer receiver for fulfilling the error interface
type PtrFooErr struct {
	val int
}

func (err *PtrFooErr) Error() string {
	return fmt.Sprintf("fooptr-%d", err.val)
}

// IntErr is an interface that also fulfills the error interface
type IntErr interface {
	Int() int
	Error() string
}

// ConcreteIntErr is an implementation of the IntErr interface using a normal receiver
type ConcreteIntErr struct {
	val int
}

func (err ConcreteIntErr) Int() int {
	return err.val
}

func (err ConcreteIntErr) Error() string {
	return fmt.Sprintf("int-%d", err.Int())
}

// ConcreteIntErr is an implementation of the IntErr interface using a pointer receiver
type ConcretePtrIntErr struct {
	val int
}

func (err *ConcretePtrIntErr) Int() int {
	return err.val
}

func (err *ConcretePtrIntErr) Error() string {
	return fmt.Sprintf("intptr-%d", err.Int())
}

// OtherErr is a different error type to be used for failed casting
type OtherErr struct{}

func (err OtherErr) Error() string {
	return "other"
}

func TestAsFooErr(t *testing.T) {
	var err error
	fooerr := &FooErr{}
	otherErr := &OtherErr{}

	err = FooErr{1}
	require.True(t, errors.As(err, &FooErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = FooErr{2}
	require.True(t, errors.As(err, fooerr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, fooerr, &FooErr{2})

	err = FooErr{3}
	require.True(t, errors.As(err, &fooerr))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, fooerr, &FooErr{3})

	err = &FooErr{4}
	require.True(t, errors.As(err, &FooErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = &FooErr{5}
	require.True(t, errors.As(err, fooerr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, fooerr, &FooErr{5})

	err = &FooErr{6}
	require.True(t, errors.As(err, &fooerr))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, fooerr, &FooErr{6})
}

func TestAsPtrFooErr(t *testing.T) {
	var err error
	fooptrerr := &PtrFooErr{}
	otherErr := &OtherErr{}

	// these don't compile - fooptrerr can't be used as an error unless it's a pointer
	// err = PtrFooErr{1}
	// require.True(t, errors.As(err, &PtrFooErr{}))
	// require.False(t, errors.As(err, &OtherErr{}))

	// err = PtrFooErr{2}
	// require.True(t, errors.As(err, fooptrerr))
	// require.False(t, errors.As(err, otherErr))
	// require.Equal(t, fooptrerr, &PtrFooErr{2})

	// err = PtrFooErr{3}
	// require.True(t, errors.As(err, &fooptrerr))
	// require.False(t, errors.As(err, &otherErr))
	// require.Equal(t, fooptrerr, &PtrFooErr{3})

	err = &PtrFooErr{4}
	require.True(t, errors.As(err, &PtrFooErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = &PtrFooErr{5}
	require.True(t, errors.As(err, fooptrerr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, fooptrerr, &PtrFooErr{5})

	err = &PtrFooErr{6}
	require.True(t, errors.As(err, &fooptrerr))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, fooptrerr, &PtrFooErr{6})
}

func TestAsConcreteIntErr(t *testing.T) {
	var err error
	var interrFace IntErr
	var interr IntErr = &ConcreteIntErr{}
	otherErr := &OtherErr{}

	err = ConcreteIntErr{1}
	// this doesn't compile - can't construct an IntErr{} as it's an interface
	// require.True(t, errors.As(err, &IntErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = ConcreteIntErr{2}
	require.True(t, errors.As(err, interr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, &ConcreteIntErr{2}, interr)

	err = ConcreteIntErr{3}
	require.True(t, errors.As(err, &interr))
	require.True(t, errors.As(err, &interrFace))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, ConcreteIntErr{3}, interr)
	require.Equal(t, ConcreteIntErr{3}, interrFace)

	err = &ConcreteIntErr{4}
	// this doesn't compile - can't construct an IntErr{} as it's an interface
	// require.True(t, errors.As(err, &IntErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = &ConcreteIntErr{5}
	interr = &ConcreteIntErr{}
	require.True(t, errors.As(err, interr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, &ConcreteIntErr{5}, interr)

	interrFace = nil
	err = &ConcreteIntErr{6}
	require.True(t, errors.As(err, &interr))
	require.True(t, errors.As(err, &interrFace))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, &ConcreteIntErr{6}, interr)
	require.Equal(t, &ConcreteIntErr{6}, interrFace)
}

func TestAsConcretePtrIntErr(t *testing.T) {
	var err error
	var interrFace IntErr
	var interr IntErr = &ConcretePtrIntErr{}
	otherErr := &OtherErr{}

	// these don't compile - ConcretePtrIntErr can't be used as an error unless it's a pointer
	// err = ConcretePtrIntErr{1}
	// require.True(t, errors.As(err, &IntErr{}))
	// require.False(t, errors.As(err, &OtherErr{}))

	// err = ConcretePtrIntErr{2}
	// require.True(t, errors.As(err, interr))
	// require.False(t, errors.As(err, otherErr))
	// require.Equal(t, &ConcretePtrIntErr{2}, interr)

	// err = ConcretePtrIntErr{3}
	// require.True(t, errors.As(err, &interr))
	// require.False(t, errors.As(err, &otherErr))
	// require.Equal(t, &ConcretePtrIntErr{3}, interr)

	err = &ConcretePtrIntErr{4}
	// this doesn't compile - can't construct an IntErr{} as it's an interface
	// require.True(t, errors.As(err, &IntErr{}))
	require.False(t, errors.As(err, &OtherErr{}))

	err = &ConcretePtrIntErr{5}
	require.True(t, errors.As(err, interr))
	require.False(t, errors.As(err, otherErr))
	require.Equal(t, &ConcretePtrIntErr{5}, interr)

	interrFace = nil
	err = &ConcretePtrIntErr{6}
	require.True(t, errors.As(err, &interr))
	require.True(t, errors.As(err, &interrFace))
	require.False(t, errors.As(err, &otherErr))
	require.Equal(t, &ConcretePtrIntErr{6}, interr)
	require.Equal(t, &ConcretePtrIntErr{6}, interrFace)
}
