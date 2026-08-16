// Cross-file same-module extension + retroactive extension probes.

/// Same module, different file than Core.swift: members must attribute to Shape.
extension Shape {
    public func area2() -> Double { origin.0 * origin.1 }

    public var perimeter2: Double { 2 * (origin.0 + origin.1) }
}

/// Retroactive extension of a type owned by another module (Swift stdlib).
extension String {
    public func spikeFlag() -> String { "spike:\(self)" }
}

extension Vec {
    public func norm() -> Double { (x * x + y * y).squareRoot() }
}

public func useExtensions() -> String {
    let s = Shape(origin: (2, 3))
    return "\(s.area2()) \(s.perimeter2) \(String(describing: "x".spikeFlag()))"
        + String(Vec(x: 3, y: 4).norm())
}
