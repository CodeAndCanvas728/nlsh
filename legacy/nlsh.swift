//
//  nlsh — natural-language shell for your terminal.
//
//  You describe what you want in plain words; a model translates it into
//  shell commands, the plan is validated locally (command/flag existence,
//  quote balance, path consistency) and regenerated if broken, then shown
//  for approval and run. Optionally, the command is only *inserted into
//  your editable shell buffer* instead of executed.
//
//  Backends (pluggable):
//    1. Apple Foundation Models — default on macOS 26+ (Apple Intelligence),
//       on-device, schema-constrained via guided generation.
//    2. Any OpenAI-compatible server — for remote models or a shared setup.
//       Point it at OpenAI, Ollama, LM Studio, or Apfel (which serves Apple
//       Foundation Models as an OpenAI API): https://github.com/Arthur-Ficial/apfel
//       No manual hosting: when the endpoint is localhost:11434 and nothing
//       is listening, nlsh starts `apfel --serve` itself and waits until it
//       answers (apfel must be installed: brew install apfel). The server
//       keeps running afterwards, so later calls are instant.
//
//  Build (macOS 26+/Apple Silicon, Xcode 26 or CLT 26):
//    swiftc -O -o nlsh nlsh.swift
//  Build (Linux, Swift 5.9+; OpenAI-compatible backend only):
//    swiftc -O -o nlsh nlsh.swift
//
//  Use:
//    ./nlsh symlink folder ./Designs to ./dist
//    ./nlsh -n find the three biggest files here        # dry-run
//    ./nlsh --print create a sha256 token               # commands only
//    eval "$(nlsh --zsh-init)"                           # Ctrl+O buffer mode
//

#if canImport(Darwin)
import Darwin
#elseif canImport(Glibc)
import Glibc
#endif
import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(FoundationModels)
import FoundationModels
#endif

// MARK: - Constants

enum Cfg {
    static let version = "0.2.0"
    static let program = "nlsh"
    static let defaultOpenAIBase = "https://api.openai.com/v1"
}

// MARK: - ANSI styling

enum Style {
    private static let tty = isatty(STDOUT_FILENO) == 1
    private static let noColor = ProcessInfo.processInfo.environment["NO_COLOR"] != nil
    static var supportsANSI: Bool { tty && !noColor }

    private static func wrap(_ code: String, _ s: String) -> String {
        supportsANSI ? "\u{001B}[\(code)m\(s)\u{001B}[0m" : s
    }
    static func dim(_ s: String) -> String { wrap("2", s) }
    static func bold(_ s: String) -> String { wrap("1", s) }
    static func cyan(_ s: String) -> String { wrap("36", s) }
    static func green(_ s: String) -> String { wrap("32", s) }
    static func yellow(_ s: String) -> String { wrap("33", s) }
    static func red(_ s: String) -> String { wrap("31", s) }
}

// MARK: - Plan model (backend-neutral)

struct PlannedCommandDTO: Codable {
    var command: String
    var risk: String
}

struct PlanDTO: Codable {
    var commands: [PlannedCommandDTO]
    var explanation: String
    var caveat: String
}

#if canImport(FoundationModels)
// Guided-generation schema: the on-device model is constrained to this
// structure (constrained decoding), so even a small model returns a
// well-formed plan.
@Generable
struct GenerableCommand {
    @Guide(description: "One complete shell command line, ready to run unattended on the user's machine. No sudo, no interactive programs, no comments.")
    var command: String
    @Guide(description: #""safe" = read-only. "careful" = creates or changes files or state. "destructive" = deletes, overwrites, resets, or force-pushes data."#)
    var risk: String
}

@Generable
struct GenerablePlan {
    @Guide(description: "Ordered shell commands that fulfill the request. Empty array if the request cannot be fulfilled unattended or lacks required information.")
    var commands: [GenerableCommand]
    @Guide(description: "One short sentence, written to the user, describing what the commands will do.")
    var explanation: String
    @Guide(description: "Brief note on anything assumed, uncertain, or potentially surprising — or an empty string.")
    var caveat: String

    var dto: PlanDTO {
        PlanDTO(commands: commands.map { PlannedCommandDTO(command: $0.command, risk: $0.risk) },
                explanation: explanation,
                caveat: caveat)
    }
}
#endif

// MARK: - Risk model

enum Risk: Int, Comparable {
    case safe = 0, careful = 1, destructive = 2
    static func < (lhs: Risk, rhs: Risk) -> Bool { lhs.rawValue < rhs.rawValue }

    var badge: String {
        switch self {
        case .safe: return Style.green("[safe]")
        case .careful: return Style.yellow("[careful]")
        case .destructive: return Style.red("[DESTRUCTIVE]")
        }
    }
}

func risk(fromTag tag: String) -> Risk {
    switch tag.lowercased() {
    case "destructive", "dangerous": return .destructive
    case "careful", "cautious": return .careful
    default: return .safe
    }
}

// Local, regex-based guard: never trust the model's self-reported risk alone.
enum Guard {
    static let destructive: [NSRegularExpression] = [
        try! NSRegularExpression(pattern: #"(?i)\brm\s+(-[a-z]*r[a-z]*f|-[a-z]*f[a-z]*r)\b"#),
        try! NSRegularExpression(pattern: #"(?i)\bmkfs"#),
        try! NSRegularExpression(pattern: #"(?i)\bdd\b[^|;&]*\bof="#),
        try! NSRegularExpression(pattern: #"(?i)\bdiskutil\s+(erase|apfs\s+delete|zeroDisk)"#),
        try! NSRegularExpression(pattern: #"(?i)\bgit\s+(reset\s+--hard|clean\s+-[a-z]*f|push\s+(.+\s)?--force)"#),
        try! NSRegularExpression(pattern: #"(?i)\bchmod\s+-R\s+777"#),
        try! NSRegularExpression(pattern: #"(?i)\bshred\b"#),
    ]
    static let sudo = try! NSRegularExpression(pattern: #"(?i)(^|[\s;|&])sudo($|\s)"#)

    static func matches(_ pattern: NSRegularExpression, in text: String) -> Bool {
        pattern.firstMatch(in: text, range: NSRange(text.startIndex..., in: text)) != nil
    }

    static func localRisk(_ command: String) -> Risk {
        destructive.contains { matches($0, in: command) } ? .destructive : .safe
    }

    static func classify(_ command: PlannedCommandDTO) -> Risk {
        max(risk(fromTag: command.risk), localRisk(command.command))
    }
}

// MARK: - Validator (fmc-style post-generation checks)
//
//  The model's plan is checked locally before it is shown. If issues are
//  found, they are fed back to the model for one regeneration. Checks are
//  heuristic — they gate obvious breakage (unknown binaries, invented flags,
//  unbalanced quotes, ignored paths), not correctness.

enum Validator {
    private static let builtins: Set<String> = [
        "cd", "echo", "type", "source", "export", "alias", "unalias", "true",
        "false", "test", "[", "umask", "pushd", "popd", "dirs", "jobs", "fg",
        "bg", "unset", "set", "shift", "read", "wait", "eval", "local",
        "hash", "command", "builtin", "printf", "which", "time", "kill",
    ]

    private static let lock = NSLock()
    private static var helpCache: [String: String] = [:]

    static func issues(in plan: PlanDTO, request: String) -> [String] {
        guard !plan.commands.isEmpty else { return [] }
        var issues: [String] = []

        for (i, c) in plan.commands.enumerated() {
            let cmd = c.command.trimmingCharacters(in: .whitespaces)
            if cmd.isEmpty {
                issues.append("command \(i + 1): empty command in plan")
                continue
            }
            if !quotesBalanced(cmd) {
                issues.append("command \(i + 1): unbalanced quotes")
            }
            guard let bin = firstBinary(in: cmd) else {
                issues.append("command \(i + 1): could not identify a command to run")
                continue
            }
            if !commandExists(bin) {
                issues.append("command \(i + 1): '\(bin)' not found in PATH and not a shell builtin")
                continue
            }
            for flag in longFlags(in: cmd) where !helpKnows(bin, flag: flag) {
                issues.append("command \(i + 1): flag \(flag) does not appear in '\(bin) --help'")
            }
        }

        for path in pathsMentioned(in: request)
        where !plan.commands.contains(where: { $0.command.contains(path) }) {
            issues.append("the request mentions '\(path)' but no command uses it")
        }
        return issues
    }

    private static func quotesBalanced(_ s: String) -> Bool {
        var single = false, double = false, escaped = false
        for ch in s {
            if escaped { escaped = false; continue }
            switch ch {
            case "\\": if !single { escaped = true }
            case "'": if !double { single.toggle() }
            case "\"": if !single { double.toggle() }
            default: break
            }
        }
        return !single && !double && !escaped
    }

    private static func firstBinary(in cmd: String) -> String? {
        let skipWords: Set<String> = ["env", "command", "sudo", "builtin", "nice", "nohup", "time"]
        for raw in cmd.split(separator: " ") {
            let token = raw.trimmingCharacters(in: .whitespaces)
            if token.isEmpty { continue }
            if token.contains("=") && !token.contains(where: { $0 == " " }) { continue } // env assignment
            if token.hasPrefix("-") { continue }                                            // flag
            if token.hasPrefix("(") || token.hasPrefix("{") { return nil }                  // subshell/block
            let bare = token.trimmingCharacters(in: CharacterSet(charactersIn: "'\""))
            if skipWords.contains(bare) { continue }
            return bare
        }
        return nil
    }

    private static func longFlags(in cmd: String) -> [String] {
        let re = try! NSRegularExpression(pattern: #"(^|\s)(--[A-Za-z0-9][A-Za-z0-9_-]*)"#)
        let ns = cmd as NSString
        return re.matches(in: cmd, range: NSRange(cmd.startIndex..., in: cmd)).compactMap { m in
            m.numberOfRanges > 2 ? ns.substring(with: m.range(at: 2)) : nil
        }
    }

    private static func commandExists(_ bin: String) -> Bool {
        if bin.hasPrefix("/") || bin.hasPrefix("~") {
            return FileManager.default.fileExists(atPath: bin)
        }
        if builtins.contains(bin) { return true }
        let path = ProcessInfo.processInfo.environment["PATH"] ?? ""
        for dir in path.split(separator: ":") {
            let candidate = "\(dir)/\(bin)"
            if FileManager.default.fileExists(atPath: candidate) { return true }
        }
        return false
    }

    // Cached `<bin> --help` text; flags must appear there. If help can't be
    // obtained (binary hangs, exits silently), verification is skipped.
    private static func helpKnows(_ bin: String, flag: String) -> Bool {
        let help: String
        lock.lock()
        if let cached = helpCache[bin] { help = cached }
        else {
            let fetched = runCapture("'\(bin)' --help", timeout: 3) ?? ""
            helpCache[bin] = fetched
            help = fetched
        }
        lock.unlock()
        if help.count < 20 { return true } // no usable help output — can't verify
        return help.contains(flag)
    }

    private static let pathRe = try! NSRegularExpression(pattern: #"(?:~/|\./|\.\./|/)[A-Za-z0-9._/+=-]+"#)
    private static func pathsMentioned(in request: String) -> [String] {
        let ns = request as NSString
        return pathRe.matches(in: request, range: NSRange(request.startIndex..., in: request))
            .map { ns.substring(with: $0.range) }
    }
}

// MARK: - Options

struct Options {
    var autoYes = false
    var force = false
    var dryRun = false
    var printMode = false
    var showVersion = false
    var backend = ""            // "", "foundation", "openai"
    var baseURL: String?
    var apiKey: String?
    var model: String?
    var request: [String] = []

    // Returns nil when help was requested.
    static func parse(_ args: [String]) -> Options? {
        var o = Options()
        var i = 0
        var onlyRequest = false
        func value(_ name: String) -> String? {
            i += 1
            guard i < args.count else {
                printErr("\(Cfg.program): \(name) expects a value")
                return nil
            }
            return args[i]
        }
        while i < args.count {
            let arg = args[i]
            if onlyRequest || !arg.hasPrefix("-") {
                onlyRequest = true // first request word: everything after is request
                o.request.append(arg)
                i += 1
                continue
            }
            switch arg {
            case "--": onlyRequest = true
            case "-h", "--help": return nil
            case "-v", "--version": o.showVersion = true
            case "-y", "--yes": o.autoYes = true
            case "-f", "--force": o.force = true
            case "-n", "--dry-run": o.dryRun = true
            case "-p", "--print": o.printMode = true
            case "--backend": o.backend = value("--backend") ?? ""
            case "--base-url": o.baseURL = value("--base-url")
            case "--api-key": o.apiKey = value("--api-key")
            case "--model": o.model = value("--model")
            default:
                onlyRequest = true // unknown dash-word belongs to the request
                o.request.append(arg)
            }
            i += 1
        }
        return o
    }
}

let usage = """
\(Cfg.program) \(Cfg.version) — say what you want; it runs the commands.

Usage
  nlsh [options] <request in plain words>      quote wildcards: "clean *.tmp"
  nlsh                                         interactive mode (keeps context)

Backends
  auto (default)   Apple Foundation Models on macOS with Apple Intelligence;
                   otherwise the OpenAI-compatible endpoint from NLSH_BASE_URL.
  --backend foundation     force Apple Foundation Models (macOS only)
  --backend openai         force the OpenAI-compatible backend
  OpenAI-compatible configuration (flags or env):
    --base-url / NLSH_BASE_URL   e.g. http://localhost:11434/v1
    --api-key  / NLSH_API_KEY (or OPENAI_API_KEY)
    --model    / NLSH_MODEL      e.g. gpt-5-mini, qwen2.5-coder:7b
  Apple Foundation Models via OpenAI API (shared setup):
    NLSH_BASE_URL=http://localhost:11434/v1
    If apfel is installed (brew install apfel), nlsh starts `apfel --serve`
    itself whenever nothing is listening on that port — no manual hosting,
    no boot service; the server stays up for later calls.

Options
  -y, --yes      approve automatically (destructive commands still ask)
  -f, --force    never ask, even for destructive commands (careful!)
  -n, --dry-run  show the plan, never execute it
  -p, --print    print commands only, no prompts (exit 2 if validation fails)
  --zsh-init     print the zsh integration script (buffer insertion, Ctrl+O)
  -v, --version  print version
  -h, --help     show this help

Shell integration (zsh)
  eval "$(nlsh --zsh-init)"
  then: type a request on your prompt and press Ctrl+O — the generated command
  is placed in your editable buffer; you review, edit, and press Enter.

Examples
  nlsh symlink folder ./Designs to ./dist
  nlsh create a new sha256 token and copy it to the clipboard
  nlsh -n find the three biggest files in this folder
"""

// MARK: - Model instructions

#if os(macOS)
let platformRules = """
- Target macOS with BSD core utilities; zsh is the shell. Assume Homebrew is at /opt/homebrew.
- Prefer BSD-flavored invocations (sed -i '', stat -f, date -v) over GNU-only forms.
"""
#else
let platformRules = """
- Target Linux with GNU core utilities; the shell is zsh or bash.
"""
#endif

let plannerRules = """
You translate natural-language requests into a minimal, ordered list of shell commands for the user's machine, to be run unattended.

Rules:
\(platformRules)
- Use as few commands as possible; keep each one readable and quote file paths.
- When the user names a file or folder without a path, treat it as relative to the current working directory.
- Commands must run without interaction: no sudo, no interactive programs (vim, top, less, man pagers), nothing that waits for input. Use non-interactive flags (-y, -f, -q) where tools support them.
- Do only what was asked: no extra installs, no verbose commentary, no echo banners.
- risk: "safe" for read-only commands (ls, cat, grep, find, du, git status, ...); "careful" for anything that creates, moves, or modifies files or state; "destructive" for anything that deletes, overwrites, resets, or force-pushes.
- Self-check before answering: every command must exist on the target system, flags must be valid, and quotes must balance. If unsure about a flag, omit it.
- If the request is impossible unattended, needs elevation, or lacks required information you cannot reasonably assume, return an empty commands array and explain why in caveat.
- If you must assume something (a path, a filename, an interpretation), take the most literal reading of the user's words and state the assumption in caveat.
- explanation: one short sentence, written to the user, saying what will happen.
"""

let openAISchemaAddendum = """

Respond with a single JSON object and nothing else — no markdown fences, no prose. Exact shape:
{"commands": [{"command": "<shell command>", "risk": "safe" | "careful" | "destructive"}], "explanation": "<one sentence>", "caveat": "<assumptions or empty string>"}
Use an empty "commands" array for requests that cannot be fulfilled.
"""

func makePrompt(request: String) -> String {
    let cwd = FileManager.default.currentDirectoryPath
    let os = ProcessInfo.processInfo.operatingSystemVersionString
    let user = NSUserName()
    let shell = ProcessInfo.processInfo.environment["SHELL"] ?? shellPath
    return """
    Current directory: \(cwd)
    System: \(os)
    User: \(user), shell: \(shell)

    Request: \(request)
    """
}

// MARK: - Backend plumbing

enum BackendError: LocalizedError {
    case http(Int, String)
    case badResponse
    case noModel

    var errorDescription: String? {
        switch self {
        case .http(let code, let body): return "backend HTTP \(code): \(body.prefix(300))"
        case .badResponse: return "backend returned an unparseable response"
        case .noModel: return "no model configured (set NLSH_MODEL or pass --model)"
        }
    }
}

protocol Planner {
    func plan(_ prompt: String) async throws -> PlanDTO
    func regenerate(_ feedback: String) async throws -> PlanDTO
}

#if canImport(FoundationModels)
final class FoundationModelsPlanner: Planner {
    private let session: LanguageModelSession

    init(instructions: String) {
        session = LanguageModelSession(instructions: instructions)
    }

    private func ask(_ prompt: String) async throws -> PlanDTO {
        let response = try await session.respond(to: prompt, generating: GenerablePlan.self)
        return response.content.dto
    }

    func plan(_ prompt: String) async throws -> PlanDTO {
        try await ask(prompt)
    }

    func regenerate(_ feedback: String) async throws -> PlanDTO {
        try await ask("""
        Your previous plan had these problems:
        \(feedback)

        Regenerate the complete corrected plan.
        """)
    }
}
#endif

final class OpenAIPlanner: Planner {
    private let base: URL
    private let apiKey: String?
    private let model: String
    private var history: [[String: String]]

    init(baseURL: String, apiKey: String?, model: String, instructions: String) {
        self.base = URL(string: baseURL) ?? URL(string: Cfg.defaultOpenAIBase)!
        self.apiKey = apiKey
        self.model = model
        self.history = [["role": "system", "content": instructions + openAISchemaAddendum]]
    }

    private struct Body: Codable {
        struct Format: Codable { let type: String }
        let model: String
        let messages: [[String: String]]
        let temperature: Double
        let response_format: Format?
    }

    private struct Response: Codable {
        struct Choice: Codable {
            struct Message: Codable { let content: String? }
            let message: Message
        }
        let choices: [Choice]
    }

    private func chat(_ messages: [[String: String]], jsonMode: Bool) async throws -> String {
        var request = URLRequest(url: base.appendingPathComponent("chat/completions"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let apiKey { request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization") }
        request.timeoutInterval = 180
        request.httpBody = try JSONEncoder().encode(
            Body(model: model, messages: messages, temperature: 0.2,
                 response_format: jsonMode ? Body.Format(type: "json_object") : nil))

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw BackendError.badResponse }
        // Some OpenAI-compatible servers reject response_format; retry without it.
        if http.statusCode == 400 && jsonMode {
            return try await chat(messages, jsonMode: false)
        }
        guard (200..<300).contains(http.statusCode) else {
            throw BackendError.http(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
        let decoded = try JSONDecoder().decode(Response.self, from: data)
        guard let content = decoded.choices.first?.message.content else { throw BackendError.badResponse }
        return content
    }

    private func decodePlan(_ raw: String) throws -> PlanDTO {
        guard let f = raw.firstIndex(of: "{"), let l = raw.lastIndex(of: "}"), f < l else {
            throw BackendError.badResponse
        }
        let json = String(raw[f...l])
        guard let data = json.data(using: .utf8) else { throw BackendError.badResponse }
        return try JSONDecoder().decode(PlanDTO.self, from: data)
    }

    private func chatForPlan(_ user: String) async throws -> PlanDTO {
        let messages = history + [["role": "user", "content": user]]
        var raw = try await chat(messages, jsonMode: true)
        do {
            return try decodePlan(raw)
        } catch {
            // One self-repair for weaker local models that emit prose around JSON.
            let repair = """
            Your previous reply was not a valid JSON object of the required shape. \
            Respond again with ONLY the JSON object: {"commands":[{"command":"...","risk":"..."}],"explanation":"...","caveat":"..."}
            """
            raw = try await chat(messages + [
                ["role": "assistant", "content": raw],
                ["role": "user", "content": repair],
            ], jsonMode: true)
            return try decodePlan(raw)
        }
    }

    func plan(_ prompt: String) async throws -> PlanDTO {
        let plan = try await chatForPlan(prompt)
        history.append(["role": "user", "content": prompt])
        return plan
    }

    func regenerate(_ feedback: String) async throws -> PlanDTO {
        let prompt = """
        Your previous plan had these problems:
        \(feedback)

        Regenerate the complete corrected plan, as JSON in the required shape.
        """
        let plan = try await chatForPlan(prompt)
        history.append(["role": "user", "content": prompt])
        return plan
    }
}

// MARK: - On-demand apfel hosting
//
//  When the OpenAI backend targets localhost:11434 and nothing is listening,
//  nlsh starts `apfel --serve` itself and waits until it answers. This gives
//  "hosted from first use" without a boot-time LaunchAgent and without
//  bundling a second binary. The spawned server survives nlsh and keeps
//  serving later calls. On macOS with Apple Intelligence none of this is
//  needed — the direct Foundation Models backend is the default.

enum AutoServe {
    static let apfelCandidates = ["/opt/homebrew/bin/apfel", "/usr/local/bin/apfel", "/usr/bin/apfel"]
    static let localHosts: Set<String> = ["localhost", "127.0.0.1", "[::1]", "::1"]

    static func isLocalhost(_ baseURL: String) -> Bool {
        guard let host = URL(string: baseURL)?.host?.lowercased() else { return false }
        return localHosts.contains(host)
    }

    static func findApfel() -> String? {
        if let installed = apfelCandidates.first(where: { FileManager.default.fileExists(atPath: $0) }) {
            return installed
        }
        if let found = runCapture("command -v apfel", timeout: 2)?
            .trimmingCharacters(in: .whitespacesAndNewlines), !found.isEmpty {
            return found
        }
        return nil
    }

    // Any HTTP response (even 404) means a server is listening.
    static func isAlive(_ baseURL: String, timeout: Double = 2) async -> Bool {
        guard let url = URL(string: baseURL) else { return false }
        var request = URLRequest(url: url)
        request.timeoutInterval = timeout
        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            return (response as? HTTPURLResponse) != nil
        } catch { return false }
    }

    @discardableResult
    static func ensure(_ baseURL: String) async -> Bool {
        guard isLocalhost(baseURL) else { return true } // remote endpoint: not ours to start
        if await isAlive(baseURL) { return true }       // already hosted (apfel, Ollama, ...)

        guard URL(string: baseURL)?.port == 11434 else { // apfel's documented drop-in port
            printErr("\(Cfg.program): no server is listening on \(baseURL). Start one (e.g. `apfel --serve`) and retry.")
            return false
        }
        guard let apfel = findApfel() else {
            printErr("""
            \(Cfg.program): no server on \(baseURL) and apfel is not installed.
            To host Apple Foundation Models locally:  brew install apfel
            Or point NLSH_BASE_URL at another OpenAI-compatible server.
            """)
            return false
        }

        print(Style.dim("  ⋯ starting \(apfel) --serve"))
        do {
            let process = Process()
            process.executableURL = URL(fileURLWithPath: apfel)
            process.arguments = ["--serve"]
            process.standardOutput = FileHandle.nullDevice
            process.standardError = FileHandle.nullDevice
            try process.run() // detached: keeps running after nlsh exits
        } catch {
            printErr("\(Cfg.program): could not start apfel: \(error.localizedDescription)")
            return false
        }

        for _ in 0..<40 { // up to ~20 s: first boot may warm up the model
            if await isAlive(baseURL, timeout: 1) { return true }
            try? await Task.sleep(nanoseconds: 500_000_000)
        }
        printErr("\(Cfg.program): apfel did not come up within 20 s — try starting it manually: `apfel --serve`")
        return false
    }
}

struct BackendConfig {
    enum Kind { case auto, foundation, openai }
    var kind: Kind
    var baseURL: String?
    var apiKey: String?
    var model: String?

    static func resolve(_ opts: Options) -> BackendConfig {
        let env = ProcessInfo.processInfo.environment
        let kind: Kind
        switch (opts.backend.lowercased(), (env["NLSH_BACKEND"] ?? "").lowercased()) {
        case ("foundation", _), (_, "foundation"): kind = .foundation
        case ("openai", _), (_, "openai"): kind = .openai
        default: kind = .auto
        }
        return BackendConfig(
            kind: kind,
            baseURL: opts.baseURL ?? env["NLSH_BASE_URL"],
            apiKey: opts.apiKey ?? env["NLSH_API_KEY"] ?? env["OPENAI_API_KEY"],
            model: opts.model ?? env["NLSH_MODEL"])
    }

    var hasOpenAIEndpoint: Bool { baseURL != nil || apiKey != nil }
}

#if canImport(FoundationModels)
// Returns availability plus the reason when unavailable. The reason is
// returned rather than stashed in a global so that no mutable global has to
// cross the actor boundary into makePlanner.
private func foundationAvailability() -> (available: Bool, message: String?) {
    switch SystemLanguageModel.default.availability {
    case .available: return (true, nil)
    case .unavailable(let reason): return (false, unavailableMessage(reason))
    @unknown default:
        return (false, "\(Cfg.program): cannot determine Foundation Models availability.")
    }
}
#endif

func makePlanner(_ config: BackendConfig) async -> Planner? {
    #if canImport(FoundationModels)
    let (fmAvailable, fmMessage) = foundationAvailability()
    if config.kind == .foundation {
        guard fmAvailable else {
            printErr(fmMessage ?? "\(Cfg.program): Apple Foundation Models are unavailable.")
            return nil
        }
        return FoundationModelsPlanner(instructions: plannerRules)
    }
    if config.kind == .auto && fmAvailable {
        return FoundationModelsPlanner(instructions: plannerRules)
    }
    #else
    if config.kind == .foundation {
        printErr("\(Cfg.program): the 'foundation' backend requires macOS 26+ with Apple Intelligence. Use --backend openai.")
        return nil
    }
    #endif

    guard config.kind == .openai || config.hasOpenAIEndpoint else {
        printErr("""
        \(Cfg.program): no model backend available.
        On macOS: enable Apple Intelligence (System Settings › Apple Intelligence & Siri).
        Any platform, any model — point nlsh at an OpenAI-compatible server:
          export NLSH_BASE_URL=http://localhost:11434/v1    # Ollama, LM Studio, ...
          export NLSH_MODEL=qwen2.5-coder:7b
        Apple Foundation Models via OpenAI API (nlsh auto-starts `apfel --serve`
        if apfel is installed — brew install apfel):
          export NLSH_BASE_URL=http://localhost:11434/v1
        """)
        return nil
    }
    let base = config.baseURL ?? Cfg.defaultOpenAIBase
    if base != Cfg.defaultOpenAIBase, !(await AutoServe.ensure(base)) { return nil }
    guard let model = config.model, !model.isEmpty else {
        printErr(BackendError.noModel.localizedDescription)
        return nil
    }
    return OpenAIPlanner(
        baseURL: base,
        apiKey: config.apiKey,
        model: model,
        instructions: plannerRules)
}

#if canImport(FoundationModels)
func unavailableMessage(_ reason: SystemLanguageModel.Availability.UnavailableReason) -> String {
    switch reason {
    case .appleIntelligenceNotEnabled:
        return "\(Cfg.program): Apple Intelligence is turned off. Enable it in System Settings › Apple Intelligence & Siri, then try again."
    case .deviceNotEligible:
        return "\(Cfg.program): this Mac isn't eligible for Apple Intelligence (requires Apple Silicon)."
    case .modelNotReady:
        return "\(Cfg.program): the on-device model is still downloading or warming up. Try again in a bit."
    @unknown default:
        return "\(Cfg.program): the on-device model is unavailable."
    }
}
#endif

// MARK: - Shell execution

let shellPath: String = {
    let candidates = ["/bin/zsh", "/usr/bin/zsh", "/bin/bash", "/usr/bin/bash"]
    return candidates.first { FileManager.default.fileExists(atPath: $0) } ?? "/bin/sh"
}()

func printErr(_ message: String) {
    FileHandle.standardError.write(Data((message + "\n").utf8))
}

func prompt(_ text: String) -> String? {
    print(text, terminator: "")
    fflush(stdout)
    return readLine()
}

@discardableResult
func runShell(_ command: String) -> Int32 {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: shellPath)
    process.arguments = ["-c", command]

    let stdoutPipe = Pipe()
    let stderrPipe = Pipe()
    process.standardOutput = stdoutPipe
    process.standardError = stderrPipe

    let done = DispatchSemaphore(value: 0)

    stdoutPipe.fileHandleForReading.readabilityHandler = { handle in
        let chunk = handle.availableData
        if chunk.isEmpty { handle.readabilityHandler = nil }
        else { FileHandle.standardOutput.write(chunk) }
    }
    stderrPipe.fileHandleForReading.readabilityHandler = { handle in
        let chunk = handle.availableData
        if chunk.isEmpty { handle.readabilityHandler = nil }
        else { FileHandle.standardError.write(chunk) }
    }
    process.terminationHandler = { _ in done.signal() }

    do { try process.run() } catch {
        printErr("\(Cfg.program): could not launch \(shellPath): \(error.localizedDescription)")
        return 127
    }
    done.wait()

    if process.terminationReason == .exit { return process.terminationStatus }
    return 128 + process.terminationStatus // killed by signal
}

// Capture a command's stdout with a hard timeout; nil on timeout/failure.
func runCapture(_ command: String, timeout: TimeInterval) -> String? {
    final class Box {
        let lock = NSLock()
        var data = Data()
    }
    let box = Box()
    let process = Process()
    process.executableURL = URL(fileURLWithPath: shellPath)
    process.arguments = ["-c", command]
    var env = ProcessInfo.processInfo.environment
    env["PAGER"] = "cat"
    env["MANPAGER"] = "cat"
    process.environment = env

    let out = Pipe()
    process.standardOutput = out
    process.standardError = Pipe()
    let done = DispatchSemaphore(value: 0)

    out.fileHandleForReading.readabilityHandler = { handle in
        let chunk = handle.availableData
        if chunk.isEmpty { handle.readabilityHandler = nil } else {
            box.lock.lock(); box.data.append(chunk); box.lock.unlock()
        }
    }
    process.terminationHandler = { _ in done.signal() }

    do { try process.run() } catch { return nil }
    if done.wait(timeout: .now() + timeout) == .timedOut {
        process.terminate()
        _ = done.wait(timeout: .now() + 2)
        return nil
    }
    return String(data: box.data, encoding: .utf8)
}

// MARK: - Plan display and approval

func showCommands(_ commands: [String], risks: [Risk]) {
    print()
    if commands.isEmpty {
        print(Style.dim("  (no commands — nothing to run)"))
        return
    }
    for (i, command) in commands.enumerated() {
        print("  \(i + 1). \(Style.cyan("$")) \(command)  \(risks[i].badge)")
    }
}

func showPlan(_ plan: PlanDTO) {
    print()
    print(Style.bold("  " + plan.explanation))
    if !plan.caveat.isEmpty { print(Style.yellow("  ⚠ " + plan.caveat)) }
    showCommands(plan.commands.map(\.command), risks: plan.commands.map(Guard.classify))
}

func editCommands(_ commands: [String]) -> [String]? {
    let url = FileManager.default.temporaryDirectory
        .appendingPathComponent("nlsh-edit-\(UUID().uuidString).sh")
    do {
        try commands.joined(separator: "\n\n").write(to: url, atomically: true, encoding: .utf8)
    } catch { return nil }

    let editor = ProcessInfo.processInfo.environment["EDITOR"] ?? "vim"
    let process = Process()
    process.executableURL = URL(fileURLWithPath: shellPath)
    process.arguments = ["-c", "\(editor) '\(url.path)'"]
    do { try process.run(); process.waitUntilExit() } catch { return nil }
    defer { try? FileManager.default.removeItem(at: url) }

    guard process.terminationStatus == 0,
          let text = try? String(contentsOf: url, encoding: .utf8) else { return nil }
    let lines = text.split(whereSeparator: \.isNewline)
        .map { $0.trimmingCharacters(in: .whitespaces) }
        .filter { !$0.isEmpty && !$0.hasPrefix("#") }
    return lines.isEmpty ? nil : lines
}

@discardableResult
func execute(_ commands: [String]) -> Int32 {
    for (i, command) in commands.enumerated() {
        print("\n  \(Style.cyan("$")) \(command)")
        fflush(stdout)
        let status = runShell(command)
        if status != 0 {
            printErr("\(Cfg.program): command \(i + 1) exited with status \(status); stopping.")
            return status
        }
    }
    return 0
}

/// Returns the exit code for this request (0 = fine).
func handle(request: String, planner: Planner, opts: Options) async -> Int32 {
    print(Style.dim("  ⋯ thinking"), terminator: "")
    fflush(stdout)

    var plan: PlanDTO
    do {
        plan = try await planner.plan(makePrompt(request: request))
    } catch {
        printErr("\n\(Cfg.program): the model failed to produce a plan: \(error.localizedDescription)")
        return 1
    }

    if Style.supportsANSI { print("\r\u{001B}[K", terminator: "") } else { print() }
    fflush(stdout)

    // Validation-and-regeneration loop (stolen from fmc).
    var issues = Validator.issues(in: plan, request: request)
    if !issues.isEmpty {
        print(Style.dim("  ⚠ plan validation failed:"))
        issues.forEach { print(Style.dim("    · \($0)")) }
        print(Style.dim("  ⋯ regenerating"), terminator: "")
        fflush(stdout)
        if let fixed = try? await planner.regenerate(issues.joined(separator: "\n- ")) {
            plan = fixed
            if Style.supportsANSI { print("\r\u{001B}[K", terminator: "") } else { print() }
            fflush(stdout)
            let remaining = Validator.issues(in: plan, request: request)
            if remaining.isEmpty {
                print(Style.dim("  ✓ regenerated plan passes validation"))
            } else {
                print(Style.dim("  ⚠ remaining issues — review carefully:"))
                remaining.forEach { print(Style.dim("    · \($0)")) }
                issues = remaining
            }
        } else {
            if Style.supportsANSI { print("\r\u{001B}[K", terminator: "") } else { print() }
            fflush(stdout)
        }
    }

    // Machine mode for buffer insertion: commands on stdout, nothing else.
    if opts.printMode {
        for command in plan.commands { print(command.command) }
        if !issues.isEmpty {
            printErr("\(Cfg.program): validation issues remain; commands may be broken.")
            return 2
        }
        return 0
    }

    showPlan(plan)
    guard !plan.commands.isEmpty else { return 0 }

    var pending = plan.commands.map(\.command)
    var risks = plan.commands.map(Guard.classify)

    if opts.dryRun {
        print(Style.dim("\n  dry run — nothing executed."))
        return 0
    }

    if pending.contains(where: { Guard.matches(Guard.sudo, in: $0) }) {
        print(Style.red("\n  ✋ refusing to run sudo — it needs an interactive password prompt."))
        print(Style.dim("  Run it yourself:"))
        for command in pending { print("  \(Style.cyan("$")) \(command)") }
        return 2
    }

    approval: while true {
        let maxRisk = risks.max() ?? .safe
        if opts.force { break approval }
        if opts.autoYes && maxRisk != .destructive { break approval }

        if maxRisk == .destructive {
            print(Style.red("\n  ⚠ one or more commands are destructive."))
            let answer = prompt("  type 'yes' to run them, anything else to cancel: ") ?? "n"
            if answer.trimmingCharacters(in: .whitespaces).lowercased() == "yes" { break approval }
            print(Style.dim("  cancelled."))
            return 0
        }

        let answer = (prompt("\n  Run? [Y=run · n=cancel · e=edit] ") ?? "n")
            .trimmingCharacters(in: .whitespaces).lowercased()
        switch answer {
        case "", "y", "yes":
            break approval
        case "n", "no":
            print(Style.dim("  cancelled."))
            return 0
        case "e", "edit":
            if let edited = editCommands(pending) {
                pending = edited
                risks = edited.map(Guard.localRisk)
                showCommands(pending, risks: risks)
            } else {
                print(Style.dim("  (edit produced no commands — plan unchanged)"))
            }
            continue approval
        default:
            print(Style.dim("  answer y, n, or e."))
            continue approval
        }
    }

    print()
    return execute(pending)
}

// MARK: - Interactive mode

func interactive(planner: Planner, opts: Options) async {
    print(Style.dim("nlsh \(Cfg.version) — type a request, 'help', or 'exit'."))
    while true {
        print(Style.cyan("nlsh ❯ "), terminator: "")
        fflush(stdout)
        guard let line = readLine() else { print(); return }
        let request = line.trimmingCharacters(in: .whitespaces)
        if request.isEmpty { continue }

        let lower = request.lowercased()
        if lower == "exit" || lower == "quit" || lower == "bye" { return }
        if lower == "help" { print(usage); continue }

        // The planner keeps its transcript, so follow-ups like
        // "now make it read-only" have context from earlier turns.
        _ = await handle(request: request, planner: planner, opts: opts)
    }
}

// MARK: - zsh integration (buffer insertion — stolen from fmc)

let zshInit = """
# nlsh zsh integration — add to ~/.zshrc:  eval "$(nlsh --zsh-init)"

# nls <request>  → plan + confirm + run
nls() { nlsh "$@"; }

# nli <request>  → place the generated command(s) on your prompt line,
#                  review/edit freely, press Enter to run (nothing auto-runs)
nli() {
  local out
  if ! out=$(nlsh --print -- "$@"); then
    print -u2 "nlsh: could not produce a valid command"
    return 1
  fi
  print -z -- "$out"
}

# Ctrl+O — type a request directly on your prompt, press Ctrl+O,
# and the generated command replaces the buffer, ready to review and edit
_nlsh_suggest() {
  local req="$BUFFER" out
  if [[ -z "$req" ]]; then
    zle -M "nlsh: type a request first, then press Ctrl+O"
    return
  fi
  if ! out=$(nlsh --print -- "$req"); then
    zle -M "nlsh: could not produce a valid command"
    return
  fi
  BUFFER="$out"
  CURSOR=$#BUFFER
  zle -M "nlsh: review the command, edit freely, then press Enter"
}
zle -N _nlsh_suggest
bindkey '^O' _nlsh_suggest
"""

// MARK: - Entry point
//
//  The entry point lives in main.swift: Swift only allows top-level code in a
//  file called main.swift, and @main cannot coexist with the file-scope
//  constants this file uses.

