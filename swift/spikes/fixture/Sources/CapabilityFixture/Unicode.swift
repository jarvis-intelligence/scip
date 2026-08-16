// Unicode declarations and emoji string literals (position-encoding probe).

public func 🚀() -> String { "liftoff 🚀🎉" }

public var π: Double = 3.14159

public struct F⃗ {
    public var magnitude: Double

    public init(magnitude: Double) {
        self.magnitude = magnitude
    }
}

public let emojiLiteral = "rocket: 🚀 flags: 🇺🇳🇻🇳 family: 👨‍👩‍👧‍👦"

public func useUnicode() -> String {
    🚀() + " " + String(π) + " " + emojiLiteral
}
