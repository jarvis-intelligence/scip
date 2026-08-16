// Core declarations referenced by every other fixture file.

import Foundation

/// Known-symbol probe target: the driver's readiness gate resolves
/// textDocument/symbolInfo at the `Shape` declaration in this file.
public struct Shape {
    public var origin: (Double, Double)

    /// Computed property with explicit get/set accessors (accessor probe).
    public var area: Double {
        get { origin.0 * origin.1 }
        set { origin = (newValue, 1) }
    }

    public init(origin: (Double, Double)) {
        self.origin = origin
    }

    public func describe() -> String { "shape at \(origin.0),\(origin.1)" }
}

public final class Vec {
    public var x = 0.0
    public var y = 0.0

    public init() {}

    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
    }

    /// Static operator declaration (operator probe).
    public static func + (lhs: Vec, rhs: Vec) -> Vec {
        Vec(x: lhs.x + rhs.x, y: lhs.y + rhs.y)
    }

    /// Escaped-in-SCIP operator: `=` is not an identifier character.
    public static func == (lhs: Vec, rhs: Vec) -> Bool {
        lhs.x == rhs.x && lhs.y == rhs.y
    }
}

/// Three overloads of `f` (overload/USR-distinctness probe).
public func f(_ i: Int) -> Int { i }

public func f(_ s: String) -> String { s }

public func f(_ i: Int, _ j: Int) -> Int { i + j }

/// Protocol requirement (witness probe).
public protocol Drawable {
    func draw() -> String
}

/// Conformance witness.
public struct Circle: Drawable {
    public var radius: Double

    public init(radius: Double) {
        self.radius = radius
    }

    public func draw() -> String { "circle(\(radius))" }
}

/// Class inheritance pair (override probe).
public class Animal {
    public func speak() -> String { "..." }
}

public final class Dog: Animal {
    public override func speak() -> String { "woof" }
}

/// Call through the protocol existential (isDynamic/receiverUsrs probe).
public func callDraw(_ d: Drawable) -> String {
    d.draw()
}

public func useEverything() -> String {
    let v1 = Vec(x: 1, y: 2)
    let v2 = Vec(x: 3, y: 4)
    let v3 = v1 + v2
    var s = Shape(origin: (2, 3))
    s.area = 6
    let c = Circle(radius: 1)
    let dog = Dog()
    return "\(v3.x) \(s.area) \(callDraw(c)) \(dog.speak()) \(s.describe())"
        + f("x") + String(f(1, 2))
}
