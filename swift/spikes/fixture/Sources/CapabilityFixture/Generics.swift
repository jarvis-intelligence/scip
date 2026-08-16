// Generic functions, types, methods, and a protocol with associated types.

public func id<T>(_ t: T) -> T { t }

public struct Box<T> {
    public var value: T

    public init(value: T) {
        self.value = value
    }

    /// Generic method over the struct's own parameter plus a fresh one.
    public func map<U>(_ transform: (T) -> U) -> Box<U> {
        Box<U>(value: transform(value))
    }
}

public protocol Repository {
    associatedtype Entity

    func find(_ id: Int) -> Entity?
}

public struct UserRepository: Repository {
    public typealias Entity = Int

    public init() {}

    public func find(_ id: Int) -> Int? { id }
}

public func useGenerics() -> String {
    let n = id(42)
    let boxed = Box<String>(value: "x").map { $0 + "!" }
    let repo = UserRepository()
    return "\(n) \(boxed.value) \(String(describing: repo.find(1)))"
}
