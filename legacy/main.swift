//
//  nlsh — entry point.
//
//  Top-level code has to live in a file called main.swift, so the program
//  itself is in nlsh.swift and only the entry point is here.
//
//  Build:
//    swiftc -O -o nlsh nlsh.swift main.swift
//

#if canImport(Darwin)
import Darwin
#elseif canImport(Glibc)
import Glibc
#endif
import Foundation

let argv = Array(CommandLine.arguments.dropFirst())
guard let opts = Options.parse(argv) else { print(usage); exit(0) }
if opts.showVersion { print("\(Cfg.program) \(Cfg.version)"); exit(0) }
if argv.contains("--zsh-init") { print(zshInit); exit(0) }

guard let planner = await makePlanner(BackendConfig.resolve(opts)) else { exit(1) }

if opts.request.isEmpty {
    await interactive(planner: planner, opts: opts)
} else {
    let code = await handle(
        request: opts.request.joined(separator: " "),
        planner: planner,
        opts: opts)
    exit(code)
}
