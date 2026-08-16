// Command generate emits the perf-fixture SwiftPM package for plan 01-03:
// ~500 files, ~15 definitions and ~10 references each, deterministic given
// the fixed seed. CAUTION: throwaway spike tooling; do not promote.
//
// Usage (from swift/spikes/perf):
//
//	GOWORK=off go run ./generate [-out gen] [-files 500] [-seed 42]
//
// The generated tree (and any .build/) is git-ignored; only the generator
// and the Package.swift template it embeds are committed.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

const packageTemplate = `// swift-tools-version:5.9
// Generated perf fixture package (plan 01-03). Do not edit.
import PackageDescription

let package = Package(
    name: "PerfFixture",
    targets: [
        .target(
            name: "PerfFixture",
            path: "Sources/PerfFixture"
        )
    ]
)
`

// fileTemplate: ~15 definitions per file (struct + members + init, class +
// members + init, protocol, witness struct, enum with three cases, free
// funcs, extension with two methods) and ~10 references (calls into three
// other files' helpers plus own-module uses). @I@ is the file's own index;
// @R1@/@R2@/@R3@ are deterministic other-file indices.
const fileTemplate = `// Generated perf fixture file Gen@I@ (seed @SEED@). Do not edit.

import Foundation

public struct Gen@I@Widget {
    public let id: Int
    public var label: String

    public init(id: Int, label: String) {
        self.id = id
        self.label = label
    }

    public func compute() -> Int {
        gen@R1@Helper(id) &+ gen@R2@Helper(id &+ 1)
    }

    public func describe() -> String {
        "Gen@I@Widget(\(id), \(label))"
    }
}

public final class Gen@I@Engine {
    public private(set) var ticks = 0

    public init() {}

    public func tick() -> Int {
        ticks &+= 1
        return gen@R1@Helper(ticks)
    }

    public func reset() {
        ticks = 0
    }
}

public protocol Gen@I@Service {
    func run(_ input: Int) -> Int
}

public struct Gen@I@DefaultService: Gen@I@Service {
    public init() {}

    public func run(_ input: Int) -> Int {
        gen@R2@Helper(input) &+ gen@R3@Helper(input &+ 1)
    }
}

public enum Gen@I@Kind {
    case alpha
    case beta
    case gamma

    public var label: String {
        switch self {
        case .alpha: return "alpha"
        case .beta: return "beta"
        case .gamma: return "gamma"
        }
    }
}

public func gen@I@Helper(_ x: Int) -> Int {
    (x &+ @I@) % 97
}

extension Gen@I@Widget {
    public func count() -> Int {
        label.count &+ id
    }

    public func upgraded() -> Gen@I@Widget {
        Gen@I@Widget(id: id &+ 1, label: label + "+")
    }
}

public func gen@I@Combine(_ a: Int, _ b: Int) -> Int {
    let engine = Gen@I@Engine()
    let widget = Gen@I@Widget(id: a, label: "a")
    let service = Gen@I@DefaultService()
    let kind = Gen@I@Kind.alpha
    return engine.tick() &+ widget.compute() &+ service.run(b)
        &+ widget.count() &+ kind.label.count
        &+ gen@R1@Helper(a) &+ gen@R2@Helper(b)
}
`

var (
	out   = flag.String("out", "gen", "output directory for the generated package")
	files = flag.Int("files", 500, "number of Swift files to generate")
	seed  = flag.Int("seed", 42, "deterministic seed")
)

// ref picks a deterministic other-file index for cross-file references.
func ref(r *rand.Rand, n, self int) int {
	i := (self*7 + 3 + r.Intn(11)) % n
	if i == self {
		i = (self + 1) % n
	}
	return i
}

func main() {
	flag.Parse()
	if *files < 2 {
		fmt.Fprintln(os.Stderr, "generate: -files must be >= 2")
		os.Exit(2)
	}
	srcDir := filepath.Join(*out, "Sources", "PerfFixture")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "Package.swift"), []byte(packageTemplate), 0o644); err != nil {
		fail(err)
	}
	// rand.New(rand.NewSource) is deterministic given the seed; all content
	// derives from it plus the file index, so re-generation is byte-stable.
	r := rand.New(rand.NewSource(int64(*seed)))
	pad := len(fmt.Sprintf("%d", *files-1))
	num := func(i int) string { return fmt.Sprintf("%0*d", pad, i) }
	for i := 0; i < *files; i++ {
		body := strings.NewReplacer(
			"@SEED@", fmt.Sprint(*seed),
			"@I@", num(i),
			"@R1@", num(ref(r, *files, i)),
			"@R2@", num(ref(r, *files, i)),
			"@R3@", num(ref(r, *files, i)),
		).Replace(fileTemplate)
		name := filepath.Join(srcDir, fmt.Sprintf("Gen%0*d.swift", pad, i))
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			fail(err)
		}
	}
	fmt.Printf("generate: wrote %d files + Package.swift under %s (seed %d)\n", *files, *out, *seed)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "generate: %v\n", err)
	os.Exit(1)
}
