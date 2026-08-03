package main

import (
	"buf.build/go/bufplugin/check"
	"github.com/labset/protobuf-toolchain/internal/rules"
)

func main() {
	check.Main(&check.Spec{
		Rules: rules.All,
	})
}
