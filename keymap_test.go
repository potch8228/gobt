package main

import "testing"

func TestKeymapConvert(t *testing.T) {
	if k, mk := Convert("KEY_A"); mk == MOD_KEY {
		t.Error("KEY_A is not a modifier key: got ", mk, k)
	}

	if k, mk := Convert("KEY_RIGHTMETA"); mk == FUNC_KEY {
		t.Error("KEY_RIGHTMETA is not a function key: got ", mk, k)
	}
}
