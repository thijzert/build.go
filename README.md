`build.go`
==========
`build.go` is a build system for compiling software packages.

History
-------
After getting thouroughly fed up with common build systems (gnu make, maven, grunt, gulp), I used a simple shell script as a build pipeline for my next project. This quickly evolved into the first Go-based version of `build.go`, which included the ability to watch a source directory, and re-run certain build steps if changes were detected.

This approach worked so well that I copied and pasted the build script in another new project. And then another. When I was about to copy the build script to a fifth new project, I knew it was time to move the features common to each variant to a library. This way, project authors could write only the configuration specific to their project, and use this library for the heavy lifting.

It was then that I realised I had become what I set out to destroy.

Usage
-----
Don't. If you _must_, create a file `build.go` somewhere in your project, like so:

```go
package main

import (
	"context"

	build "github.com/thijzert/build.go"
)

func main() {
	wl := build.WatchList{
		Paths: []string{
			"cmd/fooServer",
			"pkg/fooPackage",
		},
		FileFilter: []string{"*.go"},
	}

	build.Main("fooServer", compile, wl)
}

func compile(ctx context.Context, conf build.CompileConfig) error {
	args := []string{"go", "build", "-o", "fooServer", "cmd/fooServer/main.go"}
	return build.Passthru(ctx, args...)
}
```

And then run `go run build.go` to build your project.

`build.go` defines several command-line flags, most of which set options in the `CompileConfig`:

* `--development`: sets `Development` to `true`.
* `--quick`: sets `Quick` to `true`.
* `--version`: sets the version in the config struct. If empty, `build.go` will attempt to determine the version number from the git repository from which it's running.
* `--GOOS` / `--GOARCH`: values are passed as environment variables `GOOS` and `GOARCH` to any build steps.
* `--watch`: watch the source tree, and re-run build steps if changes are detected.
* `--run`: if compilation is successful, start the resulting executable automatically.

Calling `build.Main()` will also call `flag.Parse()`.

License
-------
Copyright (c) 2023 Thijs van Dijk, all rights reserved. Redistribution and use is permitted under the terms of the BSD 3-clause (revised) license. See the file `LICENSE` or [this url](https://tldrlegal.com/license/bsd-3-clause-license-%28revised%29) for details.
