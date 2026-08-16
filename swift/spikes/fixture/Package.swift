// swift-tools-version:5.9
// Capability fixture for the SourceKit-LSP spike (plan 01-03).
// Every hard case from the RESEARCH fixture list lives in this package.
import PackageDescription

let package = Package(
    name: "CapabilityFixture",
    targets: [
        .target(
            name: "CapabilityFixture",
            path: "Sources/CapabilityFixture"
        )
    ]
)
