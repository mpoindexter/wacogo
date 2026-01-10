package testutil

import (
	"context"
	"testing"
)

func TestXxx(t *testing.T) {
	_, err := Wat2Wasm(context.Background(), `(component
  (core module
    (func (export "a") (result i32) i32.const 0)
    (func (export "b") (result i64) i64.const 0)
  )
  (core module
    (func (export "c") (result f32) f32.const 0)
    (func (export "d") (result f64) f64.const 0)
  )
)`)

	if err != nil {
		t.Fatal(err)
	}
}
