# API Coverage — SourceKit-LSP (swiftlang, LSP-over-stdio)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
>
> Context: Phase 1 integrates SourceKit-LSP as a **spike evidence instrument**
> (probe driver `swift/spikes/lsp_probe`), not as a production client — the
> production LSP client is Phase 3 (plan 03-01). Decisions below record what the
> Phase 1 driver exercises; production-integration coverage is re-decided from
> the same full-coverage baseline in Phase 3 planning (per the second-integration
> rule, no opt-outs carry over silently).

| capability | decision | reason |
|---|---|---|
| initialize | INTEGRATE | exercised by driver; serverInfo records toolchain versions |
| initialized | INTEGRATE | part of handshake |
| shutdown | INTEGRATE | clean teardown after harvest |
| exit | INTEGRATE | clean teardown after harvest |
| textDocument/didOpen | INTEGRATE | required before symbolInfo on Xcode 26.3 (deviation finding) |
| textDocument/documentSymbol | INTEGRATE | document-outline evidence |
| textDocument/definition | INTEGRATE | defs evidence |
| textDocument/references | INTEGRATE | refs evidence; perf pass core |
| textDocument/symbolInfo (extension) | INTEGRATE | USR identity backbone — readiness gate + inventory |
| sourcekit/isIndexing (extension) | OPT-OUT | unsupported on Xcode 26.3 build (-32601 recorded); readiness gated on symbolInfo-USR probe instead |
| sourcekit/workspace/symbolNames (extension) | OPT-OUT | optional probe — not needed for Phase 1 evidence |
| sourcekit/workspace/triggerReindex (extension) | OPT-OUT | recovery-only path; cold start exercised directly instead |
| callHierarchy/prepare | INTEGRATE | hierarchy capability evidence |
| callHierarchy/incomingCalls | INTEGRATE | hierarchy capability evidence (returns real ranges) |
| callHierarchy/outgoingCalls | INTEGRATE | hierarchy capability evidence |
| typeHierarchy/prepare | INTEGRATE | hierarchy capability evidence |
| typeHierarchy/supertypes | INTEGRATE | hierarchy capability evidence |
| typeHierarchy/subtypes | INTEGRATE | hierarchy capability evidence (includes extensions) |
