package main

import (
	"buf.build/go/bufplugin/check"
	"github.com/labset/go-protobuf-toolchain-template/internal/rules"
)

func main() {
	check.Main(&check.Spec{
		Rules: rules.All,
	})
}
