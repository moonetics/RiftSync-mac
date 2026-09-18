package obfuscator

import (
	"riftsync/internal/obfuscator/ast"
	"riftsync/internal/obfuscator/compiler"
	"riftsync/internal/obfuscator/luau"
	"riftsync/internal/obfuscator/vm"
)

// Obfuscate compiles Luau source code into a protected, single-line Luau VM script.
func Obfuscate(source string) (string, error) {
	jsonAST, err := luau.Parse(source)
	if err != nil {
		return "", err
	}

	root, err := ast.FromJSON([]byte(jsonAST))
	if err != nil {
		return "", err
	}

	c := compiler.NewCompiler()
	chunk, err := c.Compile(root)
	if err != nil {
		return "", err
	}

	return vm.Generate(chunk), nil
}
